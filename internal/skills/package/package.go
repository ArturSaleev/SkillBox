// Package skillpackage reads and validates portable filesystem Skill packages.
// It deliberately treats scripts as untrusted content and never executes them.
package skillpackage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const SkillFileName = "SKILL.md"

var (
	ErrInvalidPackage = errors.New("invalid Skill package")
	ErrUnsafePath     = errors.New("unsafe package path")
)

type FileKind string

const (
	FileSkill      FileKind = "skill"
	FileScript     FileKind = "script"
	FileReference  FileKind = "reference"
	FileAsset      FileKind = "asset"
	FileAdditional FileKind = "additional"
)

// Metadata contains the portable required fields and preserves every unknown
// YAML frontmatter field in Fields for future format extensions.
type Metadata struct {
	Name        string
	Description string
	Fields      map[string]any
}

type File struct {
	Path string
	Kind FileKind
	Size int64
}

type Package struct {
	Root     string
	Metadata Metadata
	Body     string
	Files    []File
	Hash     string
}

// EnsureRoot creates the configured Skill packages directory when necessary.
// It does not create, modify, or migrate any Skill package.
func EnsureRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("%w: skills directory is required", ErrInvalidPackage)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return fmt.Errorf("%w: create skills directory: %v", ErrInvalidPackage, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("inspect skills directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: skills path is not a directory: %s", ErrInvalidPackage, root)
	}
	return nil
}

// WriteSkillMD atomically replaces only SKILL.md in an existing package (or a
// newly created package directory). Other package files are deliberately left
// untouched and no package content is ever executed.
func WriteSkillMD(root string, metadata any, body string) error {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return fmt.Errorf("create package directory: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%w: package root must be a real directory", ErrUnsafePath)
	}
	frontmatter, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode SKILL.md frontmatter: %w", err)
	}
	data := append([]byte("---\n"), frontmatter...)
	data = append(data, []byte("---\n")...)
	data = append(data, []byte(body)...)
	if len(body) > 0 && !strings.HasSuffix(body, "\n") {
		data = append(data, '\n')
	}
	tmp, err := os.CreateTemp(root, ".SKILL.md-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(data)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmpName, filepath.Join(root, SkillFileName)); err != nil {
		return fmt.Errorf("replace SKILL.md: %w", err)
	}
	return nil
}

// ParseSkillMD parses the YAML frontmatter and Markdown body of SKILL.md.
func ParseSkillMD(data []byte) (Metadata, string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return Metadata{}, "", fmt.Errorf("%w: SKILL.md must start with YAML frontmatter", ErrInvalidPackage)
	}
	rest := normalized[len("---\n"):]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		return Metadata{}, "", fmt.Errorf("%w: YAML frontmatter is not closed", ErrInvalidPackage)
	}
	frontmatter := rest[:end]
	body := string(rest[end+len("\n---\n"):])

	fields := map[string]any{}
	decoder := yaml.NewDecoder(bytes.NewReader(frontmatter))
	if err := decoder.Decode(&fields); err != nil {
		return Metadata{}, "", fmt.Errorf("%w: decode YAML frontmatter: %v", ErrInvalidPackage, err)
	}
	name, ok := fields["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return Metadata{}, "", fmt.Errorf("%w: frontmatter name is required and must be a string", ErrInvalidPackage)
	}
	description, ok := fields["description"].(string)
	if !ok || strings.TrimSpace(description) == "" {
		return Metadata{}, "", fmt.Errorf("%w: frontmatter description is required and must be a string", ErrInvalidPackage)
	}
	return Metadata{Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Fields: fields}, body, nil
}

// Load reads one package, builds its manifest, and calculates a deterministic
// hash over every regular file path and its bytes. Symlinks and special files
// are rejected; imported content can therefore never escape the package root.
func Load(root string) (*Package, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(absRoot)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("%w: package root must be a real directory", ErrInvalidPackage)
	}

	var files []File
	contents := make(map[string][]byte)
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absRoot {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err = ValidateRelativePath(rel); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlinks are not allowed: %s", ErrUnsafePath, rel)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: special files are not allowed: %s", ErrUnsafePath, rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		contents[rel] = data
		files = append(files, File{Path: rel, Kind: classify(rel), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	skillMD, ok := contents[SkillFileName]
	if !ok {
		return nil, fmt.Errorf("%w: %s is required at the package root", ErrInvalidPackage, SkillFileName)
	}
	metadata, body, err := ParseSkillMD(skillMD)
	if err != nil {
		return nil, err
	}
	return &Package{Root: absRoot, Metadata: metadata, Body: body, Files: files, Hash: hash(contents)}, nil
}

// Discover finds packages exactly one directory below skillsRoot.
func Discover(skillsRoot string) ([]*Package, error) {
	absRoot, err := filepath.Abs(skillsRoot)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, err
	}
	packages := make([]*Package, 0)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: symlink package directory is not allowed: %s", ErrUnsafePath, entry.Name())
		}
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(absRoot, entry.Name())
		if _, err = os.Lstat(filepath.Join(root, SkillFileName)); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		pkg, err := Load(root)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", entry.Name(), err)
		}
		packages = append(packages, pkg)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Metadata.Name < packages[j].Metadata.Name })
	return packages, nil
}

// ValidateRelativePath accepts only normalized relative paths within a package.
func ValidateRelativePath(name string) error {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.ContainsRune(name, '\x00') || filepath.IsAbs(name) || filepath.VolumeName(name) != "" || strings.HasPrefix(normalized, "/") || hasWindowsVolume(normalized) {
		return fmt.Errorf("%w: path must be relative: %q", ErrUnsafePath, name)
	}
	if normalized == "." {
		return fmt.Errorf("%w: invalid path: %q", ErrUnsafePath, name)
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: invalid path segment in %q", ErrUnsafePath, name)
		}
	}
	if filepath.ToSlash(filepath.Clean(filepath.FromSlash(normalized))) != normalized {
		return fmt.Errorf("%w: path is not normalized: %q", ErrUnsafePath, name)
	}
	return nil
}

func hasWindowsVolume(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) && name[1] == ':'
}

func classify(name string) FileKind {
	if name == SkillFileName {
		return FileSkill
	}
	first, _, _ := strings.Cut(name, "/")
	switch first {
	case "scripts":
		return FileScript
	case "references":
		return FileReference
	case "assets":
		return FileAsset
	default:
		return FileAdditional
	}
}

func hash(contents map[string][]byte) string {
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	var size [8]byte
	for _, name := range names {
		binary.BigEndian.PutUint64(size[:], uint64(len(name)))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write([]byte(name))
		binary.BigEndian.PutUint64(size[:], uint64(len(contents[name])))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write(contents[name])
	}
	return hex.EncodeToString(digest.Sum(nil))
}

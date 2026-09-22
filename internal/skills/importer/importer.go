// Package importer safely copies portable Skill packages from local folders,
// ZIP archives, or Git repositories. Imported content is treated as data: no
// package hook, script, or executable is ever invoked.
package importer

import (
	"archive/tar"
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aibox/skillbox/internal/skills/detector"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
	"github.com/aibox/skillbox/internal/skills/securityscan"
)

const sourceFileName = ".skillbox-source.json"

type Source struct {
	Type       string    `json:"type"`
	URL        string    `json:"url"`
	Revision   string    `json:"revision,omitempty"`
	ImportedAt time.Time `json:"imported_at"`
}

type Result struct {
	PackagePath string `json:"package_path"`
	Hash        string `json:"hash"`
	Source      Source `json:"source"`
	Unchanged   bool   `json:"unchanged"`
}

type Preview struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Files       []skillpackage.File `json:"files"`
	Scripts     int                 `json:"scripts"`
	References  int                 `json:"references"`
	Assets      int                 `json:"assets"`
	Source      Source              `json:"source"`
	PackageHash string              `json:"package_hash"`
	Executable  []detector.Finding  `json:"executable_code"`
	Warnings    []string            `json:"warnings"`
	Security    securityscan.Report `json:"security_scan"`
}

type Importer struct{ root string }

func New(root string) *Importer { return &Importer{root: root} }

func (i *Importer) LocalDirectory(path string) (Result, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Result{}, err
	}
	return i.stageAndCommit(Source{Type: "directory", URL: abs}, func(stage string) error {
		return copyTree(abs, stage)
	})
}

func (i *Importer) ZIP(path string) (Result, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Result{}, err
	}
	return i.ZIPWithSource(abs, abs)
}

func (i *Importer) ZIPWithSource(path, sourceURL string) (Result, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, err
	}
	defer reader.Close()
	return i.stageAndCommit(Source{Type: "zip", URL: sourceURL}, func(stage string) error {
		return extractZIP(reader.File, stage)
	})
}

func PreviewZIP(path, sourceURL string) (Preview, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return Preview{}, err
	}
	defer reader.Close()
	stage, err := os.MkdirTemp("", "skillbox-preview-*")
	if err != nil {
		return Preview{}, err
	}
	defer os.RemoveAll(stage)
	if err = extractZIP(reader.File, stage); err != nil {
		return Preview{}, err
	}
	return inspectStage(stage, Source{Type: "zip", URL: sourceURL})
}

func (i *Importer) Git(url, revision string) (Result, error) {
	return withGitArchive(url, revision, func(stage string, source Source) (Result, error) {
		return i.stageAndCommit(source, func(destination string) error { return copyTree(stage, destination) })
	})
}

func PreviewGit(url, revision string) (Preview, error) {
	return withGitArchive(url, revision, func(stage string, source Source) (Preview, error) {
		return inspectStage(stage, source)
	})
}

func withGitArchive[T any](url, revision string, consume func(string, Source) (T, error)) (T, error) {
	var zero T
	url = strings.TrimSpace(url)
	if url == "" {
		return zero, errors.New("Git repository URL is required")
	}
	if revision == "" {
		revision = "HEAD"
	}
	repository, err := os.MkdirTemp("", "skillbox-git-*.git")
	if err != nil {
		return zero, err
	}
	defer os.RemoveAll(repository)
	if err = os.Remove(repository); err != nil {
		return zero, err
	}
	clone := exec.Command("git", "-c", "core.hooksPath=/dev/null", "clone", "--bare", "--no-recurse-submodules", "--", url, repository)
	clone.Env = safeGitEnvironment()
	if output, cloneErr := clone.CombinedOutput(); cloneErr != nil {
		return zero, fmt.Errorf("clone Git repository: %w: %s", cloneErr, strings.TrimSpace(string(output)))
	}
	resolved, err := gitOutput(repository, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return zero, err
	}
	source := Source{Type: "git", URL: url, Revision: strings.TrimSpace(resolved)}
	stage, err := os.MkdirTemp("", "skillbox-git-archive-*")
	if err != nil {
		return zero, err
	}
	defer os.RemoveAll(stage)
	err = func() error {
		cmd := exec.Command("git", "-c", "core.hooksPath=/dev/null", "--git-dir", repository, "archive", "--format=tar", source.Revision)
		cmd.Env = safeGitEnvironment()
		stdout, pipeErr := cmd.StdoutPipe()
		if pipeErr != nil {
			return pipeErr
		}
		if pipeErr = cmd.Start(); pipeErr != nil {
			return pipeErr
		}
		extractErr := extractTAR(tar.NewReader(stdout), stage)
		waitErr := cmd.Wait()
		if extractErr != nil {
			return extractErr
		}
		return waitErr
	}()
	if err != nil {
		return zero, err
	}
	return consume(stage, source)
}

func inspectStage(stage string, source Source) (Preview, error) {
	root, err := locatePackageRoot(stage)
	if err != nil {
		return Preview{}, err
	}
	pkg, err := skillpackage.Load(root)
	if err != nil {
		return Preview{}, fmt.Errorf("validate imported package: %w", err)
	}
	preview := Preview{Name: pkg.Metadata.Name, Description: pkg.Metadata.Description, Files: pkg.Files, Source: source, PackageHash: pkg.Hash}
	preview.Executable, err = detector.Default().Scan(root)
	if err != nil {
		return Preview{}, err
	}
	if len(preview.Executable) > 0 {
		preview.Warnings = append(preview.Warnings, fmt.Sprintf("Package contains %d potentially executable file(s). SkillBox will not execute them during import.", len(preview.Executable)))
	}
	preview.Security, err = securityscan.Default().Scan(root)
	if err != nil {
		return Preview{}, err
	}
	preview.Warnings = append(preview.Warnings, securityscan.Summary(preview.Security))
	for _, file := range pkg.Files {
		switch file.Kind {
		case skillpackage.FileScript:
			preview.Scripts++
		case skillpackage.FileReference:
			preview.References++
		case skillpackage.FileAsset:
			preview.Assets++
		}
	}
	return preview, nil
}

func (i *Importer) stageAndCommit(source Source, populate func(string) error) (Result, error) {
	if err := skillpackage.EnsureRoot(i.root); err != nil {
		return Result{}, err
	}
	stage, err := os.MkdirTemp(i.root, ".import-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(stage)
	if err = populate(stage); err != nil {
		return Result{}, err
	}
	packageRoot, err := locatePackageRoot(stage)
	if err != nil {
		return Result{}, err
	}
	basePackage, err := skillpackage.Load(packageRoot)
	if err != nil {
		return Result{}, fmt.Errorf("validate imported package: %w", err)
	}
	path := packagePath(basePackage.Metadata.Fields, source)
	destination := filepath.Join(i.root, path)
	if existingSource, sourceErr := readSource(destination); sourceErr == nil && existingSource.Type == source.Type && existingSource.URL == source.URL && existingSource.Revision == source.Revision {
		source.ImportedAt = existingSource.ImportedAt
	} else {
		source.ImportedAt = time.Now().UTC()
	}
	encoded, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		return Result{}, err
	}
	if err = os.WriteFile(filepath.Join(packageRoot, sourceFileName), append(encoded, '\n'), 0o600); err != nil {
		return Result{}, err
	}
	pkg, err := skillpackage.Load(packageRoot)
	if err != nil {
		return Result{}, fmt.Errorf("validate imported package: %w", err)
	}
	if existing, loadErr := skillpackage.Load(destination); loadErr == nil {
		if sameSource(existing.Root, source) && existing.Hash == pkg.Hash {
			return Result{PackagePath: path, Hash: existing.Hash, Source: source, Unchanged: true}, nil
		}
		return Result{}, fmt.Errorf("package path %q already exists with different source", path)
	} else if !os.IsNotExist(loadErr) {
		return Result{}, loadErr
	}
	if err = os.Rename(packageRoot, destination); err != nil {
		return Result{}, fmt.Errorf("commit imported package: %w", err)
	}
	return Result{PackagePath: path, Hash: pkg.Hash, Source: source}, nil
}

func locatePackageRoot(stage string) (string, error) {
	if _, err := os.Stat(filepath.Join(stage, skillpackage.SkillFileName)); err == nil {
		return stage, nil
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return "", err
	}
	var directories []string
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(stage, entry.Name()))
		}
	}
	if len(directories) == 1 {
		if _, err = os.Stat(filepath.Join(directories[0], skillpackage.SkillFileName)); err == nil {
			return directories[0], nil
		}
	}
	return "", errors.New("import must contain one Skill package with SKILL.md at its root")
}

var safeName = regexp.MustCompile(`[^a-z0-9._-]+`)

func packagePath(fields map[string]any, source Source) string {
	name, _ := fields["slug"].(string)
	name = strings.Trim(safeName.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-.")
	if name == "" || skillpackage.ValidateRelativePath(name) != nil || strings.HasPrefix(name, ".") {
		name = "imported-skill"
	}
	return name + "-" + SourceFingerprint(source)[:12]
}

func sameSource(root string, source Source) bool {
	existing, err := readSource(root)
	return err == nil && existing.Type == source.Type && existing.URL == source.URL && existing.Revision == source.Revision
}

func readSource(root string) (Source, error) {
	data, err := os.ReadFile(filepath.Join(root, sourceFileName))
	if err != nil {
		return Source{}, err
	}
	var source Source
	err = json.Unmarshal(data, &source)
	return source, err
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err = skillpackage.ValidateRelativePath(rel); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed: %s", rel)
		}
		target := filepath.Join(destination, filepath.FromSlash(rel))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special files are not allowed: %s", rel)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		return writeFile(target, in)
	})
}

func extractZIP(files []*zip.File, destination string) error {
	for _, file := range files {
		name := strings.TrimSuffix(file.Name, "/")
		if name == "" {
			continue
		}
		if err := skillpackage.ValidateRelativePath(name); err != nil {
			return err
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed: %s", name)
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
			continue
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		err = writeFile(target, io.LimitReader(in, 256<<20))
		closeErr := in.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func extractTAR(reader *tar.Reader, destination string) error {
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name == "" {
			continue
		}
		if err = skillpackage.ValidateRelativePath(name); err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o750)
		case tar.TypeReg, tar.TypeRegA:
			err = writeFile(target, io.LimitReader(reader, 256<<20))
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			continue
		default:
			return fmt.Errorf("unsupported Git archive entry: %s", name)
		}
		if err != nil {
			return err
		}
	}
}

func writeFile(path string, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, reader)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func gitOutput(repository string, args ...string) (string, error) {
	full := append([]string{"-c", "core.hooksPath=/dev/null", "--git-dir", repository}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = safeGitEnvironment()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func safeGitEnvironment() []string {
	env := os.Environ()
	env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0")
	return env
}

func SourceFingerprint(source Source) string {
	sum := sha256.Sum256([]byte(source.Type + "\x00" + source.URL + "\x00" + source.Revision))
	return hex.EncodeToString(sum[:])
}

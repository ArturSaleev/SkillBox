package skillpackage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillMDPreservesUnknownMetadata(t *testing.T) {
	metadata, body, err := ParseSkillMD([]byte("---\r\nname: example-skill\r\ndescription: Use for examples\r\nlicense: MIT\r\nlabels:\r\n  - demo\r\n---\r\n# Instructions\r\n\r\nDo the work.\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Name != "example-skill" || metadata.Description != "Use for examples" {
		t.Fatalf("metadata=%#v", metadata)
	}
	if metadata.Fields["license"] != "MIT" || metadata.Fields["labels"] == nil {
		t.Fatalf("unknown fields were not preserved: %#v", metadata.Fields)
	}
	if body != "# Instructions\n\nDo the work.\n" {
		t.Fatalf("body=%q", body)
	}
}

func TestEnsureRootCreatesDirectoryAndRejectsFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "skills")
	if err := EnsureRoot(root); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("skills root was not created: info=%#v err=%v", info, err)
	}
	file := filepath.Join(t.TempDir(), "skills-file")
	writeFile(t, file, "not a directory")
	if err := EnsureRoot(file); !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("expected ErrInvalidPackage, got %v", err)
	}
}

func TestParseSkillMDRejectsMalformedDocuments(t *testing.T) {
	for name, document := range map[string]string{
		"no frontmatter": "# Instructions",
		"not closed":     "---\nname: demo\ndescription: Demo\n",
		"missing name":   "---\ndescription: Demo\n---\nBody",
		"bad name type":  "---\nname: [demo]\ndescription: Demo\n---\nBody",
		"bad yaml":       "---\nname: demo\ndescription: [\n---\nBody",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseSkillMD([]byte(document)); !errors.Is(err, ErrInvalidPackage) {
				t.Fatalf("expected ErrInvalidPackage, got %v", err)
			}
		})
	}
}

func TestLoadBuildsManifestAndHashesWholePackage(t *testing.T) {
	root := makePackage(t, "demo", "---\nname: demo\ndescription: Demo package\ncustom: value\n---\n# Instructions\n")
	writeFile(t, filepath.Join(root, "scripts", "analyze.py"), "print('stored, never executed')\n")
	writeFile(t, filepath.Join(root, "references", "rules.md"), "rules\n")
	writeFile(t, filepath.Join(root, "assets", "template.json"), "{}\n")
	writeFile(t, filepath.Join(root, "notes.txt"), "extra\n")

	first, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != 5 {
		t.Fatalf("files=%#v", first.Files)
	}
	wantKinds := map[string]FileKind{
		"SKILL.md": FileSkill, "scripts/analyze.py": FileScript,
		"references/rules.md": FileReference, "assets/template.json": FileAsset,
		"notes.txt": FileAdditional,
	}
	for _, file := range first.Files {
		if file.Kind != wantKinds[file.Path] {
			t.Fatalf("file=%#v want kind %q", file, wantKinds[file.Path])
		}
	}

	writeFile(t, filepath.Join(root, "assets", "template.json"), "{\"changed\":true}\n")
	second, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash == second.Hash {
		t.Fatal("package hash did not change when an asset changed")
	}
	writeFile(t, filepath.Join(root, "assets", "template.json"), "{}\n")
	third, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != third.Hash {
		t.Fatalf("hash is not deterministic: %s != %s", first.Hash, third.Hash)
	}
}

func TestDiscoverFindsOnlyDirectSkillPackagesInStableOrder(t *testing.T) {
	root := t.TempDir()
	makePackageAt(t, filepath.Join(root, "z-dir"), "---\nname: zeta\ndescription: Zeta\n---\nBody\n")
	makePackageAt(t, filepath.Join(root, "a-dir"), "---\nname: alpha\ndescription: Alpha\n---\nBody\n")
	if err := os.Mkdir(filepath.Join(root, "not-a-skill"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "README.md"), "ignored\n")

	packages, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0].Metadata.Name != "alpha" || packages[1].Metadata.Name != "zeta" {
		t.Fatalf("packages=%#v", packages)
	}
}

func TestValidateRelativePathRejectsTraversalAndAbsolutePaths(t *testing.T) {
	for _, name := range []string{"", ".", "../secret", "references/../../secret", "/etc/passwd", `C:\\Windows\\secret`, `\\\\server\\share`, "scripts//run.sh", `scripts\\..\\secret`, "bad\x00name"} {
		if err := ValidateRelativePath(name); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("path %q: expected ErrUnsafePath, got %v", name, err)
		}
	}
	for _, name := range []string{"SKILL.md", "scripts/run.sh", "references/nested/rules.md"} {
		if err := ValidateRelativePath(name); err != nil {
			t.Fatalf("path %q rejected: %v", name, err)
		}
	}
}

func TestLoadRejectsSymlinks(t *testing.T) {
	root := makePackage(t, "demo", "---\nname: demo\ndescription: Demo\n---\nBody\n")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")
	if err := os.Symlink(outside, filepath.Join(root, "references")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Load(root); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("expected ErrUnsafePath, got %v", err)
	}
}

func TestLoadRequiresRootSkillMD(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "references", SkillFileName), "---\nname: nested\ndescription: Nested\n---\nBody")
	if _, err := Load(root); !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("expected ErrInvalidPackage, got %v", err)
	}
}

func makePackage(t *testing.T, directory, document string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), directory)
	makePackageAt(t, root, document)
	return root
}

func makePackageAt(t *testing.T, root, document string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, SkillFileName), document)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

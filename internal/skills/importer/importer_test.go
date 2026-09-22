package importer_test

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aibox/skillbox/internal/skills/importer"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
)

const validSkill = "---\nname: Imported\ndescription: Imported package\nslug: imported\n---\nFollow the procedure.\n"

func TestLocalDirectoryImportIsValidatedAndIdempotent(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(source, "scripts"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte(validSkill), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "executed")
	if err := os.WriteFile(filepath.Join(source, "scripts", "never-run.sh"), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "skills")
	service := importer.New(root)
	first, err := service.LocalDirectory(source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.LocalDirectory(source)
	if err != nil {
		t.Fatal(err)
	}
	if first.PackagePath == "" || first.Hash == "" || second.PackagePath != first.PackagePath || !second.Unchanged {
		t.Fatalf("unexpected results: first=%#v second=%#v", first, second)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("imported script was executed: %v", err)
	}
	if _, err = skillpackage.Load(filepath.Join(root, first.PackagePath)); err != nil {
		t.Fatal(err)
	}
}

func TestZIPImportRejectsPathTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "unsafe.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte(validSkill))
	_ = writer.Close()
	_ = file.Close()
	if _, err = importer.New(filepath.Join(t.TempDir(), "skills")).ZIP(archive); err == nil {
		t.Fatal("unsafe ZIP path was accepted")
	}
}

func TestZIPImportPreservesExecutableContentWithoutRunningIt(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "portable-skill.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	marker := filepath.Join(t.TempDir(), "executed")
	entries := map[string]string{
		"portable/SKILL.md":             validSkill,
		"portable/scripts/never-run.sh": "#!/bin/sh\ntouch " + marker + "\n",
		"portable/references/notes.md":  "Reference only\n",
	}
	for name, content := range entries {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, createErr = entry.Write([]byte(content)); createErr != nil {
			t.Fatal(createErr)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "skills")
	result, err := importer.New(root).ZIP(archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ZIP payload was executed: %v", err)
	}
	pkg, err := skillpackage.Load(filepath.Join(root, result.PackagePath))
	if err != nil {
		t.Fatal(err)
	}
	// The importer adds provenance metadata beside the three archive files.
	if len(pkg.Files) != len(entries)+1 {
		t.Fatalf("files=%d want=%d", len(pkg.Files), len(entries)+1)
	}
}

func TestGitImportRecordsResolvedRevisionWithoutCheckoutHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repository := t.TempDir()
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "test@example.com")
	runGit(t, repository, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repository, "SKILL.md"), []byte(validSkill), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repository, "scripts"), 0o750); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "executed")
	if err := os.WriteFile(filepath.Join(repository, "scripts", "payload.sh"), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", ".")
	runGit(t, repository, "commit", "-m", "skill")
	wantRevision := runGit(t, repository, "rev-parse", "HEAD")
	result, err := importer.New(filepath.Join(t.TempDir(), "skills")).Git(repository, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.URL != repository || result.Source.Revision != wantRevision {
		t.Fatalf("source=%#v want revision=%s", result.Source, wantRevision)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("repository code was executed: %v", err)
	}
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = directory
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output.String())
	}
	return string(bytes.TrimSpace(output.Bytes()))
}

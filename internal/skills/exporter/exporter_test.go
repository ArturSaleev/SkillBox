package exporter_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/aibox/skillbox/internal/skills/exporter"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
)

func TestDirectoryAndZIPContainCompletePortablePackage(t *testing.T) {
	source := makePackage(t)
	want, err := skillpackage.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "exported")
	directoryResult, err := exporter.Directory(source, directory)
	if err != nil {
		t.Fatal(err)
	}
	got, err := skillpackage.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hash != want.Hash || directoryResult.Hash != want.Hash || directoryResult.Files != len(want.Files) {
		t.Fatalf("directory export mismatch: result=%#v hash=%s want=%s", directoryResult, got.Hash, want.Hash)
	}

	archive := filepath.Join(t.TempDir(), "skill.zip")
	zipResult, err := exporter.ZIP(source, archive)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := make(map[string]bool, len(reader.File))
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"SKILL.md", "scripts/run.sh", "references/guide.md", "assets/icon.txt"} {
		if !names[name] {
			t.Fatalf("ZIP is missing %s: %#v", name, names)
		}
	}
	if zipResult.Hash != want.Hash || zipResult.Files != len(want.Files) {
		t.Fatalf("ZIP export mismatch: %#v", zipResult)
	}
}

func TestExportDoesNotRequireDatabaseAndNeverOverwrites(t *testing.T) {
	source := makePackage(t)
	destination := filepath.Join(t.TempDir(), "portable")
	if _, err := exporter.Directory(source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := exporter.Directory(source, destination); err == nil {
		t.Fatal("existing export destination was overwritten")
	}
	zipDestination := filepath.Join(t.TempDir(), "portable.zip")
	if err := os.WriteFile(zipDestination, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := exporter.ZIP(source, zipDestination); err == nil {
		t.Fatal("existing ZIP destination was overwritten")
	}
	data, err := os.ReadFile(zipDestination)
	if err != nil || string(data) != "keep" {
		t.Fatalf("existing ZIP changed: data=%q err=%v", data, err)
	}
}

func TestExportRejectsSymlinkedPackageContent(t *testing.T) {
	source := makePackage(t)
	if err := os.Symlink("../SKILL.md", filepath.Join(source, "assets", "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := exporter.Directory(source, filepath.Join(t.TempDir(), "export")); err == nil {
		t.Fatal("package symlink was accepted")
	}
}

func makePackage(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"SKILL.md":            "---\nname: Portable\ndescription: Complete package\nslug: portable\n---\nUse all files.\n",
		"scripts/run.sh":      "#!/bin/sh\nexit 0\n",
		"references/guide.md": "Guide\n",
		"assets/icon.txt":     "asset\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

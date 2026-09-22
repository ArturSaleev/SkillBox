// Package exporter produces portable copies of filesystem Skill packages.
// It operates only on package files and has no database dependency.
package exporter

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skillpackage "github.com/aibox/skillbox/internal/skills/package"
)

type Result struct {
	Destination string `json:"destination"`
	Hash        string `json:"hash"`
	Files       int    `json:"files"`
}

func Directory(packageRoot, destination string) (Result, error) {
	pkg, err := skillpackage.Load(packageRoot)
	if err != nil {
		return Result{}, fmt.Errorf("validate Skill package: %w", err)
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return Result{}, err
	}
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		if err == nil {
			return Result{}, fmt.Errorf("export destination already exists: %s", destination)
		}
		return Result{}, err
	}
	parent := filepath.Dir(destination)
	if err = os.MkdirAll(parent, 0o750); err != nil {
		return Result{}, err
	}
	stage, err := os.MkdirTemp(parent, ".skill-export-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(stage)
	if err = copyPackage(pkg, stage); err != nil {
		return Result{}, err
	}
	if err = os.Rename(stage, destination); err != nil {
		return Result{}, fmt.Errorf("commit directory export: %w", err)
	}
	return Result{Destination: destination, Hash: pkg.Hash, Files: len(pkg.Files)}, nil
}

func ZIP(packageRoot, destination string) (Result, error) {
	pkg, err := skillpackage.Load(packageRoot)
	if err != nil {
		return Result{}, fmt.Errorf("validate Skill package: %w", err)
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return Result{}, err
	}
	if !strings.EqualFold(filepath.Ext(destination), ".zip") {
		return Result{}, errors.New("ZIP export destination must end with .zip")
	}
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		if err == nil {
			return Result{}, fmt.Errorf("export destination already exists: %s", destination)
		}
		return Result{}, err
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return Result{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".skill-export-*.zip")
	if err != nil {
		return Result{}, err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err = writeZIPPackage(tmp, pkg); err != nil {
		return Result{}, err
	}
	if err = tmp.Sync(); err == nil {
		err = tmp.Close()
	}
	if err != nil {
		return Result{}, err
	}
	if err = os.Rename(tmpName, destination); err != nil {
		return Result{}, fmt.Errorf("commit ZIP export: %w", err)
	}
	committed = true
	return Result{Destination: destination, Hash: pkg.Hash, Files: len(pkg.Files)}, nil
}

// WriteZIP streams a complete validated package without consulting a database.
func WriteZIP(packageRoot string, destination io.Writer) (Result, error) {
	pkg, err := skillpackage.Load(packageRoot)
	if err != nil {
		return Result{}, fmt.Errorf("validate Skill package: %w", err)
	}
	if err = writeZIPPackage(destination, pkg); err != nil {
		return Result{}, err
	}
	return Result{Hash: pkg.Hash, Files: len(pkg.Files)}, nil
}

func writeZIPPackage(destination io.Writer, pkg *skillpackage.Package) error {
	writer := zip.NewWriter(destination)
	files := append([]skillpackage.File(nil), pkg.Files...)
	sort.Slice(files, func(a, b int) bool { return files[a].Path < files[b].Path })
	for _, file := range files {
		data, readErr := os.ReadFile(filepath.Join(pkg.Root, filepath.FromSlash(file.Path)))
		if readErr != nil {
			_ = writer.Close()
			return readErr
		}
		header := &zip.FileHeader{Name: file.Path, Method: zip.Deflate}
		header.SetMode(0o600)
		entry, createErr := writer.CreateHeader(header)
		if createErr == nil {
			_, createErr = entry.Write(data)
		}
		if createErr != nil {
			_ = writer.Close()
			return createErr
		}
	}
	return writer.Close()
}

func copyPackage(pkg *skillpackage.Package, destination string) error {
	for _, file := range pkg.Files {
		source := filepath.Join(pkg.Root, filepath.FromSlash(file.Path))
		target := filepath.Join(destination, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		in, err := os.Open(source)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, err = io.Copy(out, in)
		}
		closeOut := error(nil)
		if out != nil {
			closeOut = out.Close()
		}
		closeIn := in.Close()
		if err != nil {
			return err
		}
		if closeOut != nil {
			return closeOut
		}
		if closeIn != nil {
			return closeIn
		}
	}
	return nil
}

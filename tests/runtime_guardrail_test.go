package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestImportedCodeHasNoExecutionPath is an architectural regression guardrail.
// Skill packages may contain scripts, but production code must treat them only
// as files. The sole process-launch exception is the importer invoking Git with
// hooks disabled to read repository objects and create an archive.
func TestImportedCodeHasNoExecutionPath(t *testing.T) {
	repositoryRoot := filepath.Clean("..")
	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "release" || name == "dashboard" || name == "benchmark" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if name == "os/exec" && filepath.ToSlash(path) != "../internal/skills/importer/importer.go" {
				t.Errorf("production package %s can launch processes; imported Skill code must remain inert", path)
			}
			if name == "plugin" {
				t.Errorf("production package %s can dynamically load code; imported Skill code must remain inert", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	fileSet := token.NewFileSet()
	path := filepath.Join(repositoryRoot, "internal", "skills", "importer", "importer.go")
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Command" {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name != "exec" {
			return true
		}
		if len(call.Args) == 0 {
			t.Errorf("%s: process launch without a fixed executable", fileSet.Position(call.Pos()))
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING || literal.Value != `"git"` {
			t.Errorf("%s: importer may invoke only the fixed Git executable", fileSet.Position(call.Pos()))
		}
		return true
	})
}

package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go/parser"
	"go/token"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return root
}

// Card server/08 — Wiring honors the dependency directions.
// Given all four packages (board, api, ui, server) are present
// When  the architecture gate runs
// Then  the dependency directions of the parent plan hold: ui→api via HTTP
//
//	only, api→board in-process, server composing all, nothing importing
//	server or reading data files outside its owner
func TestArchspecGateGreen(t *testing.T) {
	if _, err := exec.LookPath("archspec"); err != nil {
		t.Fatalf("archspec is part of the stack (ADR-001) but not on PATH: %v", err)
	}
	cmd := exec.Command("archspec", "verify", "--strict", ".")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("archspec verify --strict failed:\n%s", out)
	}
	t.Logf("archspec: %s", strings.TrimSpace(string(out)))
}

// The same card (server/08) proven statically on imports. The directions
// pinned: ui imports no project package at all (its coupling to the api is
// HTTP at runtime only — the pre-kanban pin checked api/todos, ui may now
// import none); data-file opening is per-owner, so the SQLite driver
// appears only in the module that owns a data file — board (kanban.db).
// The todos arm of this pin retired with the package: the transitional
// allowance (dependencies_kanban.md, "todos until its endpoints retire at
// KW4") was scheduled to tighten to board alone at KW4, and the api/10
// retirement is where it did.
// And nothing imports the composition root (server, i.e. cmd/todo).
func TestDependencyDirectionsOnImports(t *testing.T) {
	root := repoRoot(t)
	imports := sourceImports(t, root)
	if len(imports) == 0 {
		t.Fatal("no source files found")
	}

	for file, imps := range imports {
		inDataOwner := strings.HasPrefix(file, "board/")
		for _, imp := range imps {
			if strings.HasPrefix(file, "ui/") && strings.HasPrefix(imp, "todo/") {
				t.Errorf("%s imports %q: ui imports no project packages — it reaches the api over HTTP only", file, imp)
			}
			if strings.HasPrefix(imp, "modernc.org/sqlite") && !inDataOwner {
				t.Errorf("%s imports %q: board is the only data-file owner — the todo store package retired at api/10", file, imp)
			}
			if imp == "todo/cmd/todo" {
				t.Errorf("%s imports %q: nothing imports the composition root (server)", file, imp)
			}
		}
	}
}

// sourceImports maps repo-relative non-test .go files to their import paths.
func sourceImports(t *testing.T, root string) map[string][]string {
	t.Helper()
	result := map[string][]string{}
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// static assets and test-support trees (testdata is ignored
			// by the go tool and by archspec) are not served module code;
			// like _test.go files they are out of the model.
			if d.Name() == "static" || d.Name() == "testdata" ||
				(d.Name() != "." && strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, filepath.Join(root, path), nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		var imps []string
		for _, i := range file.Imports {
			imp := strings.Trim(i.Path.Value, `"`)
			imps = append(imps, imp)
		}
		result[filepath.ToSlash(path)] = imps
		return nil
	})
	if err != nil {
		t.Fatalf("scan sources: %v", err)
	}
	return result
}

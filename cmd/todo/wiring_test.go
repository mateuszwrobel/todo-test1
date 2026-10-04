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

// Card server/06 — Wiring honors the dependency directions.
// Given the composed server is running
// When  any page operation is performed
// Then  page requests reach todos data exclusively through the api contract
//
//	over HTTP
//	And no module other than the store's own code opens the data file
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

// The same card proven statically on imports: ui imports no internal module
// (its coupling to api is HTTP at runtime only), and the SQLite driver —
// the opener of the data file — appears only in the todos module.
func TestDependencyDirectionsOnImports(t *testing.T) {
	root := repoRoot(t)
	imports := sourceImports(t, root)
	if len(imports) == 0 {
		t.Fatal("no source files found")
	}

	for file, imps := range imports {
		inUI := strings.HasPrefix(file, "ui/")
		inTodos := strings.HasPrefix(file, "todos/")
		for _, imp := range imps {
			if inUI && (imp == "todo/api" || imp == "todo/todos") {
				t.Errorf("%s imports %q: ui must reach api/todos over HTTP only", file, imp)
			}
			if !inTodos && strings.HasPrefix(imp, "modernc.org/sqlite") {
				t.Errorf("%s imports %q: only the todos module opens the data file", file, imp)
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

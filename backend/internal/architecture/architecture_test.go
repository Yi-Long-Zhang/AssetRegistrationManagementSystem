// Package architecture_test verifies repository-specific Go package boundaries.
package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const internalPrefix = "asset-registration-management-system/backend/internal/"

func TestProductionPackageDependencies(t *testing.T) {
	root := backendRoot(t)
	for _, productionDir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, productionDir), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			checkFileImports(t, path, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s source: %v", productionDir, err)
		}
	}
}

func checkFileImports(t *testing.T, path, relative string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Errorf("parse %s: %v", relative, err)
		return
	}

	owner := ownerPackage(relative)
	for _, item := range file.Imports {
		importPath, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			t.Errorf("parse import in %s: %v", relative, err)
			continue
		}
		if strings.Contains(importPath, "/backend/tests") {
			t.Errorf("%s: production code must not import test packages", relative)
		}
		if forbiddenImport(owner, importPath) {
			t.Errorf("%s: %s package must not import %s", relative, owner, importPath)
		}
	}

	if file.Name.Name == "main" && !strings.HasPrefix(relative, "cmd/") {
		t.Errorf("%s: command packages must live under cmd", relative)
	}
}

func ownerPackage(relative string) string {
	parts := strings.Split(relative, "/")
	if len(parts) >= 3 && parts[0] == "internal" {
		return parts[1]
	}
	return ""
}

func forbiddenImport(owner, importPath string) bool {
	if !strings.HasPrefix(importPath, internalPrefix) {
		return false
	}
	target := strings.TrimPrefix(importPath, internalPrefix)
	target = strings.Split(target, "/")[0]
	switch owner {
	case "model":
		return target == "app" || target == "database" || target == "httpapi" || target == "service"
	case "database":
		return target == "app" || target == "httpapi" || target == "service"
	case "service":
		return target == "app" || target == "httpapi"
	case "httpapi":
		return target == "app"
	default:
		return false
	}
}

func backendRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
}

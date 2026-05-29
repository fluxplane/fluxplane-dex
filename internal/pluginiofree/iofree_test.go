package pluginiofree

import (
	"bufio"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var forbiddenImports = map[string]bool{
	"database/sql": true,
	"net":          true,
	"net/http":     true,
	"os":           true,
	"os/exec":      true,
	"syscall":      true,
}

var sdkImportsRequiringHostHTTP = map[string]bool{
	"github.com/slack-go/slack":              true,
	"gitlab.com/gitlab-org/api/client-go/v2": true,
}

var ioFreeRoots = []string{
	"plugins",
	filepath.Join("internal", "atlassian"),
	filepath.Join("internal", "vision"),
	filepath.Join("internal", "websearch"),
}

func TestPluginDirectIOImportBaseline(t *testing.T) {
	root := repoRoot(t)
	actual := directPluginIOImports(t, root)
	expected := readBaseline(t, filepath.Join(root, "internal", "pluginiofree", "baseline.txt"))
	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("plugin direct IO imports changed\n\nactual:\n%s\n\nexpected baseline:\n%s\n\nRemove migrated entries from internal/pluginiofree/baseline.txt; do not add new plugin IO imports.", strings.Join(actual, "\n"), strings.Join(expected, "\n"))
	}
}

func TestPluginSDKHTTPClientsUseHostTransport(t *testing.T) {
	root := repoRoot(t)
	var violations []string
	fset := token.NewFileSet()
	err := walkIOFreeGoFiles(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		usesSDK := false
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if sdkImportsRequiringHostHTTP[importPath] {
				usesSDK = true
				break
			}
		}
		if !usesSDK {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(data), "HostHTTPClient") {
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			violations = append(violations, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("plugin SDK clients must inject pluginbinding.HostHTTPClient:\n%s", strings.Join(violations, "\n"))
	}
}

func directPluginIOImports(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	err := walkIOFreeGoFiles(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if forbiddenImports[importPath] {
				out = append(out, rel+" "+importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func walkIOFreeGoFiles(root string, fn func(string, os.DirEntry, error) error) error {
	for _, relRoot := range ioFreeRoots {
		scanRoot := filepath.Join(root, relRoot)
		if err := filepath.WalkDir(scanRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			return fn(path, entry, nil)
		}); err != nil {
			return err
		}
	}
	return nil
}

func readBaseline(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var out []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}

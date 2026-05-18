package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildCachedReusesUnchangedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/cachetest\n")
	writeFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")

	idx, stats, err := BuildCached(root)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Loaded {
		t.Fatal("first cache build should be cold")
	}
	if !strings.HasSuffix(filepath.ToSlash(stats.Path), "/.ctxpack/index.sqlite") {
		t.Fatalf("cache path = %q, want SQLite cache", stats.Path)
	}
	if stats.ParsedFiles != 2 || stats.ReusedFiles != 0 {
		t.Fatalf("first stats = %#v", stats)
	}
	if err := Save(idx, stats.Path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(stats.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "SQLite format 3") {
		t.Fatalf("cache file is not SQLite: %q", string(data[:16]))
	}

	idx, stats, err = BuildCached(root)
	if err != nil {
		t.Fatal(err)
	}
	if !stats.Loaded {
		t.Fatal("second cache build should load cache")
	}
	if stats.ReusedFiles != len(idx.Files) || stats.ParsedFiles != 0 {
		t.Fatalf("second stats = %#v, files = %d", stats, len(idx.Files))
	}
}

func TestBuildCachedReparsesChangedFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/cachetest\n")
	mainPath := filepath.Join(root, "main.go")
	writeFile(t, mainPath, "package main\n\nfunc main() {}\n")

	idx, stats, err := BuildCached(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(idx, stats.Path); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond)
	writeFile(t, mainPath, "package main\n\nfunc main() {}\nfunc changed() {}\n")

	_, stats, err = BuildCached(root)
	if err != nil {
		t.Fatal(err)
	}
	if stats.ParsedFiles != 1 || stats.ReusedFiles != 1 {
		t.Fatalf("stats after change = %#v", stats)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

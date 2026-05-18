package rank

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestChangedFilesDetailedIncludesWorkingTreeAndUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "ctxpack@example.test")
	runGit(t, root, "config", "user.name", "ctxpack")

	writeTestFile(t, root, "service.go", "package demo\n\nfunc Service() {}\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	writeTestFile(t, root, "service.go", "package demo\n\nfunc Service() {}\nfunc Changed() {}\n")
	writeTestFile(t, root, "new_service.go", "package demo\n\nfunc NewService() {}\n")
	writeTestFile(t, root, ".ctxpack/index.sqlite", "SQLite format 3\x00\n")

	changes, err := ChangedFilesDetailed(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]ChangedFile{}
	for _, change := range changes {
		byPath[change.Path] = change
	}
	if got := byPath["service.go"].Status; got != "modified" {
		t.Fatalf("service.go status = %q, want modified; changes=%+v", got, changes)
	}
	if got := byPath["service.go"].Additions; got != 1 {
		t.Fatalf("service.go additions = %d, want 1; changes=%+v", got, changes)
	}
	if got := byPath["new_service.go"].Status; got != "untracked" {
		t.Fatalf("new_service.go status = %q, want untracked; changes=%+v", got, changes)
	}
	if got := byPath["new_service.go"].Additions; got != 3 {
		t.Fatalf("new_service.go additions = %d, want 3; changes=%+v", got, changes)
	}
	if _, ok := byPath[".ctxpack/index.sqlite"]; ok {
		t.Fatalf("ctxpack cache artifact should not be included: %+v", changes)
	}

	paths := ChangedPaths(changes)
	if len(paths) != 2 {
		t.Fatalf("changed paths = %#v, want 2 paths", paths)
	}
}

func TestParseNameStatusRename(t *testing.T) {
	changes := parseNameStatus("R100\told/path.go\tnew/path.go\n", "main...HEAD")
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(changes))
	}
	change := changes[0]
	if change.Status != "renamed" || change.OldPath != "old/path.go" || change.Path != "new/path.go" {
		t.Fatalf("unexpected rename parse: %+v", change)
	}
}

func TestParseNumstat(t *testing.T) {
	changes := parseNumstat("12\t3\tservice.go\n-\t-\timage.png\n", "unstaged")
	if len(changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(changes))
	}
	if changes[0].Path != "service.go" || changes[0].Additions != 12 || changes[0].Deletions != 3 {
		t.Fatalf("unexpected text stat: %+v", changes[0])
	}
	if changes[1].Path != "image.png" || !changes[1].Binary {
		t.Fatalf("unexpected binary stat: %+v", changes[1])
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeTestFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

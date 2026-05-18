package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilyaux/ctxpack/internal/rank"
)

func TestFlexibleFlagArgsAllowsFlagsAfterTask(t *testing.T) {
	args := flexibleFlagArgs([]string{
		"add commission endpoint",
		"--budget", "12000",
		"--include-tests",
		"--output", ".ctxpack/out.md",
	}, map[string]bool{"include-tests": true})

	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	budget := fs.Int("budget", 0, "")
	includeTests := fs.Bool("include-tests", false, "")
	output := fs.String("output", "", "")
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}

	if *budget != 12000 {
		t.Fatalf("budget = %d, want 12000", *budget)
	}
	if !*includeTests {
		t.Fatal("include-tests was not parsed")
	}
	if *output != ".ctxpack/out.md" {
		t.Fatalf("output = %q", *output)
	}
	if got := fs.Arg(0); got != "add commission endpoint" {
		t.Fatalf("task = %q", got)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run(version) code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ctxpack dev") || !strings.Contains(stdout.String(), "commit:") {
		t.Fatalf("unexpected version output:\n%s", stdout.String())
	}
}

func TestRunHelpListsPublicCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run(--help) code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"ctxpack index",
		"ctxpack pack",
		"ctxpack diff",
		"ctxpack ci",
		"ctxpack bench",
		"ctxpack mcp",
		"ctxpack explain",
		"ctxpack version",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunDiffRendersGitDiffContext(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init")
	runCLIGit(t, root, "config", "user.email", "ctxpack@example.test")
	runCLIGit(t, root, "config", "user.name", "ctxpack")

	writeCLIFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\n")
	writeCLIFile(t, root, "service_test.go", "package demo\n\nfunc TestService() {}\n")
	runCLIGit(t, root, "add", ".")
	runCLIGit(t, root, "commit", "-m", "initial")

	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\nfunc Changed() {}\n")
	writeCLIFile(t, root, "new_service.go", "package demo\n\nfunc NewService() {}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"diff", "--repo", root, "--base", "HEAD", "--stdout", "review service diff"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run(diff) code = %d, stderr=%s", code, stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{
		"## Git diff context",
		"modified `service.go`",
		"untracked `new_service.go`",
		"Selected changed files:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("diff output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, ".ctxpack/index.sqlite") {
		t.Fatalf("diff output included ctxpack cache artifact:\n%s", text)
	}
}

func TestRunDiffWritesSummaryJSON(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init")
	runCLIGit(t, root, "config", "user.email", "ctxpack@example.test")
	runCLIGit(t, root, "config", "user.name", "ctxpack")

	writeCLIFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\n")
	writeCLIFile(t, root, "service_test.go", "package demo\n\nfunc TestService() {}\n")
	runCLIGit(t, root, "add", ".")
	runCLIGit(t, root, "commit", "-m", "initial")

	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\nfunc Changed() {}\n")
	writeCLIFile(t, root, "new_service.go", "package demo\n\nfunc NewService() {}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"diff",
		"--repo", root,
		"--base", "HEAD",
		"--output", ".ctxpack/diff.md",
		"--summary-json", "reports/diff-summary.json",
		"review service diff",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run(diff) code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Wrote summary") {
		t.Fatalf("stdout did not mention summary path:\n%s", stdout.String())
	}

	data, err := os.ReadFile(filepath.Join(root, "reports", "diff-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary diffSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Task != "review service diff" {
		t.Fatalf("task = %q", summary.Task)
	}
	if summary.EstimatedTokens <= 0 {
		t.Fatalf("estimated tokens = %d", summary.EstimatedTokens)
	}
	if len(summary.ChangedFiles) != 2 {
		t.Fatalf("changed files = %d, want 2: %+v", len(summary.ChangedFiles), summary.ChangedFiles)
	}
	if !containsString(summary.SelectedChangedFiles, "service.go") {
		t.Fatalf("selected changed files missing service.go: %+v", summary.SelectedChangedFiles)
	}
	if len(summary.Selection) == 0 || !summary.Selection[0].Changed {
		t.Fatalf("selection missing changed marker: %+v", summary.Selection)
	}
	if len(summary.ReviewChecklist) == 0 {
		t.Fatal("review checklist is empty")
	}
	if summary.MarkdownOutput == "" {
		t.Fatal("markdown output path is empty")
	}
}

func TestRunDiffFailOnMissingTestsStillWritesArtifacts(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init")
	runCLIGit(t, root, "config", "user.email", "ctxpack@example.test")
	runCLIGit(t, root, "config", "user.name", "ctxpack")

	writeCLIFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\n")
	runCLIGit(t, root, "add", ".")
	runCLIGit(t, root, "commit", "-m", "initial")

	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\nfunc Changed() {}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"diff",
		"--repo", root,
		"--base", "HEAD",
		"--output", ".ctxpack/diff.md",
		"--summary-json", "reports/diff-summary.json",
		"--fail-on", "missing-tests",
		"review service diff",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run(diff) code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing-tests") {
		t.Fatalf("stderr missing check failure:\n%s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".ctxpack", "diff.md")); err != nil {
		t.Fatalf("markdown artifact was not written: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "reports", "diff-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary diffSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Passed {
		t.Fatalf("summary passed = true, want false: %+v", summary.Checks)
	}
	if len(summary.Checks) != 1 || summary.Checks[0].Name != "missing-tests" || summary.Checks[0].Passed {
		t.Fatalf("unexpected checks: %+v", summary.Checks)
	}
}

func TestRunCIAliasUsesDefaultArtifactsAndChecks(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init")
	runCLIGit(t, root, "config", "user.email", "ctxpack@example.test")
	runCLIGit(t, root, "config", "user.name", "ctxpack")

	writeCLIFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\n")
	runCLIGit(t, root, "add", ".")
	runCLIGit(t, root, "commit", "-m", "initial")

	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\nfunc Changed() {}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"ci",
		"--repo", root,
		"--base", "HEAD",
		"review service diff",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run(ci) code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".ctxpack", "ci-context.md")); err != nil {
		t.Fatalf("ci markdown artifact was not written: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "reports", "ctxpack-diff-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary diffSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Passed {
		t.Fatalf("summary passed = true, want false: %+v", summary.Checks)
	}
	if summary.MarkdownOutput == "" || !strings.HasSuffix(filepath.ToSlash(summary.MarkdownOutput), ".ctxpack/ci-context.md") {
		t.Fatalf("unexpected markdown output: %q", summary.MarkdownOutput)
	}
}

func TestRunCIAliasAllowsCheckOverride(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init")
	runCLIGit(t, root, "config", "user.email", "ctxpack@example.test")
	runCLIGit(t, root, "config", "user.name", "ctxpack")

	writeCLIFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\n")
	runCLIGit(t, root, "add", ".")
	runCLIGit(t, root, "commit", "-m", "initial")

	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\nfunc Changed() {}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"ci",
		"--repo", root,
		"--base", "HEAD",
		"--fail-on", "over-budget",
		"review service diff",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run(ci) code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "reports", "ctxpack-diff-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary diffSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if !summary.Passed {
		t.Fatalf("summary passed = false, want true: %+v", summary.Checks)
	}
	if len(summary.Checks) != 1 || summary.Checks[0].Name != "over-budget" {
		t.Fatalf("unexpected checks: %+v", summary.Checks)
	}
}

func TestRunCIReadsConfigDefaults(t *testing.T) {
	root := t.TempDir()
	runCLIGit(t, root, "init")
	runCLIGit(t, root, "config", "user.email", "ctxpack@example.test")
	runCLIGit(t, root, "config", "user.name", "ctxpack")

	writeCLIFile(t, root, ".ctxpack.yml", `
ci:
  base: HEAD
  budget: 7777
  output: .ctxpack/config-ci.md
  summary_json: reports/config-ci-summary.json
  fail_on:
    - over-budget
`)
	writeCLIFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\n")
	runCLIGit(t, root, "add", ".")
	runCLIGit(t, root, "commit", "-m", "initial")

	writeCLIFile(t, root, "service.go", "package demo\n\nfunc Service() {}\nfunc Changed() {}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"ci", "--repo", root, "review service diff"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run(ci) code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".ctxpack", "config-ci.md")); err != nil {
		t.Fatalf("configured markdown artifact was not written: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "reports", "config-ci-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary diffSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Budget != 7777 {
		t.Fatalf("summary budget = %d, want 7777", summary.Budget)
	}
	if len(summary.Checks) != 1 || summary.Checks[0].Name != "over-budget" {
		t.Fatalf("unexpected checks: %+v", summary.Checks)
	}
	if !strings.HasSuffix(filepath.ToSlash(summary.MarkdownOutput), ".ctxpack/config-ci.md") {
		t.Fatalf("unexpected markdown output: %q", summary.MarkdownOutput)
	}
}

func TestEvaluateDiffChecksForbiddenPaths(t *testing.T) {
	summary := diffSummary{
		Budget:               100,
		EstimatedTokens:      90,
		SelectedChangedFiles: []string{"services/auth/login.go"},
		ChangedFiles: rankChangedFilesForTest{
			{Path: "services/auth/login.go"},
		}.toRank(),
		Selection: []diffSelectionSummary{
			{Path: "services/auth/login.go", Changed: true},
			{Path: "services/billing/billing_test.go", IsTest: true},
		},
	}
	checks, err := evaluateDiffChecks(summary, "all", "services/auth/**")
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 4 {
		t.Fatalf("checks = %d, want 4: %+v", len(checks), checks)
	}
	var forbidden diffCheckResult
	for _, check := range checks {
		if check.Name == "forbidden-paths" {
			forbidden = check
		}
	}
	if forbidden.Passed || len(forbidden.Details) == 0 {
		t.Fatalf("forbidden check did not fail: %+v", forbidden)
	}
}

type rankChangedFileForTest struct {
	Path string
}

type rankChangedFilesForTest []rankChangedFileForTest

func (files rankChangedFilesForTest) toRank() []rank.ChangedFile {
	out := make([]rank.ChangedFile, 0, len(files))
	for _, file := range files {
		out = append(out, rank.ChangedFile{Path: file.Path})
	}
	return out
}

func runCLIGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeCLIFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

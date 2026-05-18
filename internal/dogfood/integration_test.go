package dogfood

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ilyaux/ctxpack/internal/budget"
	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/pack"
	"github.com/ilyaux/ctxpack/internal/rank"
	"github.com/ilyaux/ctxpack/internal/tokens"
)

type dogfoodReport struct {
	Repo            string          `json:"repo"`
	GeneratedAt     string          `json:"generated_at"`
	IndexedFiles    int             `json:"indexed_files"`
	Stack           index.StackInfo `json:"stack"`
	FilesByLanguage map[string]int  `json:"files_by_language"`
	TopDirectories  map[string]int  `json:"top_directories"`
	Packs           []packReport    `json:"packs,omitempty"`
}

type packReport struct {
	Task            string   `json:"task"`
	Budget          int      `json:"budget"`
	EstimatedTokens int      `json:"estimated_tokens"`
	SelectedFiles   int      `json:"selected_files"`
	TopFiles        []string `json:"top_files"`
	Output          string   `json:"output"`
}

func TestDogfoodIndexAndPacks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping dogfood integration test in short mode")
	}
	repo := dogfoodRepo(t)
	reportsDir, err := filepath.Abs(filepath.Join("..", "..", "reports", "dogfood"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		t.Fatal(err)
	}

	idx, err := index.Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Files) < 4000 {
		t.Fatalf("indexed %d files, want at least 4000 for dogfood repo", len(idx.Files))
	}
	assertContains(t, idx.Stack.Languages, "Java")
	assertContains(t, idx.Stack.PackageManagers, "Maven")

	report := dogfoodReport{
		Repo:            repo,
		GeneratedAt:     time.Now().Format(time.RFC3339),
		IndexedFiles:    len(idx.Files),
		Stack:           idx.Stack,
		FilesByLanguage: filesByLanguage(idx.Files),
		TopDirectories:  topDirectories(idx.Files, 12),
	}

	tasks := []string{
		"fix JSF alarm report page validation and backing bean logic",
		"review Maven module dependencies for report generation",
		"add audit log database query to Java service",
	}
	for _, task := range tasks {
		packReport := writePackReport(t, reportsDir, idx, task, 12000)
		report.Packs = append(report.Packs, packReport)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reportsDir, "summary.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func writePackReport(t *testing.T, reportsDir string, idx *index.RepoIndex, task string, tokenBudget int) packReport {
	t.Helper()

	scored := rank.ScoreFiles(task, idx, rank.Options{IncludeTests: true})
	if len(scored) == 0 {
		t.Fatalf("no scored files for task %q", task)
	}
	selections := budget.Select(scored, tokenBudget)
	input := pack.RenderInput{Task: task, Budget: tokenBudget, Repo: idx, Selections: selections}
	markdown, selections := pack.FitToBudget(input)
	estimated := tokens.Estimate(markdown)
	if estimated > tokenBudget {
		t.Fatalf("pack for %q estimated at %d tokens, budget %d", task, estimated, tokenBudget)
	}
	if !strings.Contains(markdown, "## Budget decisions") || !strings.Contains(markdown, "## Context files") {
		t.Fatalf("pack for %q is missing required sections", task)
	}

	output := filepath.Join(reportsDir, slug(task)+".md")
	if err := os.WriteFile(output, []byte(markdown), 0644); err != nil {
		t.Fatal(err)
	}

	var topFiles []string
	for _, sel := range selections {
		if sel.Mode == budget.ModeOmitted {
			continue
		}
		topFiles = append(topFiles, sel.Scored.File.Path)
		if len(topFiles) >= 10 {
			break
		}
	}
	if len(topFiles) == 0 {
		t.Fatalf("pack for %q selected no files", task)
	}

	return packReport{
		Task:            task,
		Budget:          tokenBudget,
		EstimatedTokens: estimated,
		SelectedFiles:   len(topFiles),
		TopFiles:        topFiles,
		Output:          output,
	}
}

func dogfoodRepo(t *testing.T) string {
	t.Helper()
	repo := os.Getenv("CTXPACK_DOGFOOD_REPO")
	if repo == "" {
		t.Skip("set CTXPACK_DOGFOOD_REPO to run the optional large-repo dogfood test")
	}
	info, err := os.Stat(repo)
	if err != nil || !info.IsDir() {
		t.Skipf("dogfood repo not found at %s", repo)
	}
	return repo
}

func filesByLanguage(files []index.FileInfo) map[string]int {
	counts := map[string]int{}
	for _, file := range files {
		counts[file.Language]++
	}
	return counts
}

func topDirectories(files []index.FileInfo, limit int) map[string]int {
	counts := map[string]int{}
	for _, file := range files {
		dir := "(root)"
		parts := strings.Split(filepath.ToSlash(file.Path), "/")
		if len(parts) > 1 {
			dir = parts[0]
		}
		counts[dir]++
	}

	type item struct {
		name  string
		count int
	}
	items := make([]item, 0, len(counts))
	for name, count := range counts {
		items = append(items, item{name: name, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].name < items[j].name
		}
		return items[i].count > items[j].count
	})
	if len(items) > limit {
		items = items[:limit]
	}

	out := map[string]int{}
	for _, item := range items {
		out[item.name] = item.count
	}
	return out
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %v", want, values)
}

func slug(text string) string {
	text = strings.ToLower(text)
	var b strings.Builder
	lastDash := false
	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

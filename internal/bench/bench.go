package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ilyaux/ctxpack/internal/budget"
	"github.com/ilyaux/ctxpack/internal/config"
	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/pack"
	"github.com/ilyaux/ctxpack/internal/pathspec"
	"github.com/ilyaux/ctxpack/internal/rank"
	"github.com/ilyaux/ctxpack/internal/tokens"
)

type Task struct {
	Name   string   `json:"name"`
	Task   string   `json:"task"`
	Budget int      `json:"budget,omitempty"`
	Expect []string `json:"expect,omitempty"`
	Avoid  []string `json:"avoid,omitempty"`
}

type Options struct {
	Repo             string
	TasksPath        string
	OutputDir        string
	Budget           int
	IncludeTests     bool
	IncludeBaselines bool
}

type Report struct {
	Repo         string          `json:"repo"`
	GeneratedAt  string          `json:"generated_at"`
	IndexedFiles int             `json:"indexed_files"`
	Stack        index.StackInfo `json:"stack"`
	ConfigPath   string          `json:"config_path,omitempty"`
	Cache        CacheReport     `json:"cache"`
	Tasks        []TaskResult    `json:"tasks"`
}

type CacheReport struct {
	Path         string `json:"path,omitempty"`
	Loaded       bool   `json:"loaded"`
	ReusedFiles  int    `json:"reused_files"`
	ParsedFiles  int    `json:"parsed_files"`
	RemovedFiles int    `json:"removed_files,omitempty"`
}

type TaskResult struct {
	Name            string           `json:"name"`
	Task            string           `json:"task"`
	Budget          int              `json:"budget"`
	EstimatedTokens int              `json:"estimated_tokens"`
	SelectedFiles   int              `json:"selected_files"`
	TopFiles        []string         `json:"top_files"`
	Selection       []FileBreakdown  `json:"selection,omitempty"`
	Baselines       []BaselineResult `json:"baselines,omitempty"`
	ExpectedHits    []string         `json:"expected_hits,omitempty"`
	MissingExpected []string         `json:"missing_expected,omitempty"`
	AvoidHits       []string         `json:"avoid_hits,omitempty"`
	Output          string           `json:"output,omitempty"`
	Passed          bool             `json:"passed"`
}

type FileBreakdown struct {
	Path       string                `json:"path"`
	Mode       string                `json:"mode"`
	Score      float64               `json:"score"`
	Reasons    []string              `json:"reasons,omitempty"`
	Components []rank.ScoreComponent `json:"components,omitempty"`
}

func Run(opt Options) (Report, error) {
	if opt.Repo == "" {
		opt.Repo = "."
	}
	if opt.Budget <= 0 {
		opt.Budget = 12000
	}
	tasks, err := ParseTasksFile(opt.TasksPath)
	if err != nil {
		return Report{}, err
	}
	if len(tasks) == 0 {
		return Report{}, fmt.Errorf("no benchmark tasks found in %s", opt.TasksPath)
	}

	idx, stats, err := index.BuildCached(opt.Repo)
	if err != nil {
		return Report{}, err
	}
	if err := index.Save(idx, stats.Path); err != nil {
		return Report{}, err
	}
	cfg, cfgPath, err := config.Load(idx.Root)
	if err != nil {
		return Report{}, err
	}
	idx = config.Apply(idx, cfg)
	defaultBudget := opt.Budget
	if cfg.Budget > 0 {
		defaultBudget = cfg.Budget
	}
	includeTests := opt.IncludeTests
	if cfg.IncludeTests != nil {
		includeTests = *cfg.IncludeTests
	}

	report := Report{
		Repo:         idx.Root,
		GeneratedAt:  time.Now().Format(time.RFC3339),
		IndexedFiles: len(idx.Files),
		Stack:        idx.Stack,
		ConfigPath:   cfgPath,
		Cache: CacheReport{
			Path:         stats.Path,
			Loaded:       stats.Loaded,
			ReusedFiles:  stats.ReusedFiles,
			ParsedFiles:  stats.ParsedFiles,
			RemovedFiles: stats.RemovedFiles,
		},
	}
	for _, task := range tasks {
		taskBudget := task.Budget
		if taskBudget <= 0 {
			taskBudget = defaultBudget
		}
		result, err := runTask(idx, cfg, task, taskBudget, includeTests, opt.IncludeBaselines, opt.OutputDir)
		if err != nil {
			return report, err
		}
		report.Tasks = append(report.Tasks, result)
	}

	if opt.OutputDir != "" {
		if err := os.MkdirAll(opt.OutputDir, 0755); err != nil {
			return report, err
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return report, err
		}
		if err := os.WriteFile(filepath.Join(opt.OutputDir, "bench-summary.json"), data, 0644); err != nil {
			return report, err
		}
	}
	return report, nil
}

func runTask(idx *index.RepoIndex, cfg config.Config, task Task, taskBudget int, includeTests bool, includeBaselines bool, outputDir string) (TaskResult, error) {
	name := task.Name
	if name == "" {
		name = slug(task.Task)
	}
	scored := rank.ScoreFiles(task.Task, idx, rank.Options{
		IncludeTests: includeTests,
		Priority:     cfg.Priority,
	})
	selections := budget.Select(scored, taskBudget)
	input := pack.RenderInput{Task: task.Task, Budget: taskBudget, Repo: idx, Selections: selections}
	markdown, selections := pack.FitToBudget(input)
	estimated := tokens.Estimate(markdown)

	var topFiles []string
	var breakdown []FileBreakdown
	for _, sel := range selections {
		if sel.Mode == budget.ModeOmitted {
			continue
		}
		topFiles = append(topFiles, sel.Scored.File.Path)
		breakdown = append(breakdown, FileBreakdown{
			Path:       sel.Scored.File.Path,
			Mode:       string(sel.Mode),
			Score:      sel.Scored.Score,
			Reasons:    limitedStrings(sel.Scored.Reasons, 5),
			Components: topScoreComponents(sel.Scored.Components, 6),
		})
		if len(topFiles) >= 15 {
			break
		}
	}
	expectedHits, missing := matchExpectations(task.Expect, topFiles)
	avoidHits := matchAvoid(task.Avoid, topFiles)
	passed := estimated <= taskBudget && len(missing) == 0 && len(avoidHits) == 0 && len(topFiles) > 0
	var baselines []BaselineResult
	if includeBaselines {
		baselines = compareBaselines(idx, task, taskBudget)
	}

	output := ""
	if outputDir != "" {
		packDir := filepath.Join(outputDir, "packs")
		if err := os.MkdirAll(packDir, 0755); err != nil {
			return TaskResult{}, err
		}
		output = filepath.Join(packDir, slug(name)+".md")
		if err := os.WriteFile(output, []byte(markdown), 0644); err != nil {
			return TaskResult{}, err
		}
	}

	return TaskResult{
		Name:            name,
		Task:            task.Task,
		Budget:          taskBudget,
		EstimatedTokens: estimated,
		SelectedFiles:   len(topFiles),
		TopFiles:        topFiles,
		Selection:       breakdown,
		Baselines:       baselines,
		ExpectedHits:    expectedHits,
		MissingExpected: missing,
		AvoidHits:       avoidHits,
		Output:          output,
		Passed:          passed,
	}, nil
}

func topScoreComponents(components []rank.ScoreComponent, limit int) []rank.ScoreComponent {
	out := append([]rank.ScoreComponent(nil), components...)
	sort.SliceStable(out, func(i, j int) bool {
		ai := out[i].Points
		if ai < 0 {
			ai = -ai
		}
		aj := out[j].Points
		if aj < 0 {
			aj = -aj
		}
		if ai == aj {
			return out[i].Reason < out[j].Reason
		}
		return ai > aj
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func limitedStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func matchExpectations(patterns []string, files []string) ([]string, []string) {
	var hits []string
	var missing []string
	for _, pattern := range patterns {
		found := false
		for _, file := range files {
			if pathspec.Match(pattern, file) {
				hits = append(hits, pattern)
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, pattern)
		}
	}
	return hits, missing
}

func matchAvoid(patterns []string, files []string) []string {
	var hits []string
	for _, file := range files {
		if pathspec.MatchAny(patterns, file) {
			hits = append(hits, file)
		}
	}
	return hits
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
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "task"
	}
	if len(out) > 80 {
		out = strings.Trim(out[:80], "-")
	}
	return out
}

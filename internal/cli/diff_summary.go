package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ilyaux/ctxpack/internal/budget"
	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/pack"
	"github.com/ilyaux/ctxpack/internal/rank"
)

type diffSummary struct {
	Repository           string                 `json:"repository"`
	Task                 string                 `json:"task"`
	GeneratedAt          string                 `json:"generated_at"`
	DiffBase             string                 `json:"diff_base"`
	Budget               int                    `json:"budget"`
	EstimatedTokens      int                    `json:"estimated_tokens"`
	IndexedFiles         int                    `json:"indexed_files"`
	Stack                index.StackInfo        `json:"stack"`
	ConfigPath           string                 `json:"config_path,omitempty"`
	Cache                diffCacheSummary       `json:"cache"`
	ChangedFiles         []rank.ChangedFile     `json:"changed_files"`
	SelectedChangedFiles []string               `json:"selected_changed_files,omitempty"`
	OmittedChangedFiles  []string               `json:"omitted_changed_files,omitempty"`
	TopFiles             []string               `json:"top_files"`
	Selection            []diffSelectionSummary `json:"selection"`
	ReviewChecklist      []string               `json:"review_checklist,omitempty"`
	Passed               bool                   `json:"passed"`
	Checks               []diffCheckResult      `json:"checks,omitempty"`
	MarkdownOutput       string                 `json:"markdown_output,omitempty"`
}

type diffCacheSummary struct {
	Path         string `json:"path,omitempty"`
	Loaded       bool   `json:"loaded"`
	ReusedFiles  int    `json:"reused_files"`
	ParsedFiles  int    `json:"parsed_files"`
	RemovedFiles int    `json:"removed_files,omitempty"`
}

type diffSelectionSummary struct {
	Path            string                `json:"path"`
	Mode            string                `json:"mode"`
	EstimatedTokens int                   `json:"estimated_tokens"`
	Score           float64               `json:"score"`
	Changed         bool                  `json:"changed"`
	IsTest          bool                  `json:"is_test,omitempty"`
	Reasons         []string              `json:"reasons,omitempty"`
	Components      []rank.ScoreComponent `json:"components,omitempty"`
}

func buildDiffSummary(input pack.RenderInput, stats index.CacheStats, configPath string, estimatedTokens int, markdownOutput string) diffSummary {
	selected := map[string]bool{}
	changed := map[string]bool{}
	for _, change := range input.ChangedFiles {
		changed[change.Path] = true
	}

	var topFiles []string
	var selection []diffSelectionSummary
	for _, sel := range input.Selections {
		if sel.Mode == budget.ModeOmitted {
			continue
		}
		path := sel.Scored.File.Path
		selected[path] = true
		if len(topFiles) < 15 {
			topFiles = append(topFiles, path)
		}
		selection = append(selection, diffSelectionSummary{
			Path:            path,
			Mode:            string(sel.Mode),
			EstimatedTokens: sel.EstimatedTokens,
			Score:           sel.Scored.Score,
			Changed:         changed[path],
			IsTest:          sel.Scored.File.IsTest,
			Reasons:         limitedStrings(sel.Scored.Reasons, 5),
			Components:      topScoreComponents(sel.Scored.Components, 6),
		})
	}

	var selectedChanged []string
	var omittedChanged []string
	for _, change := range input.ChangedFiles {
		if selected[change.Path] {
			selectedChanged = append(selectedChanged, change.Path)
		} else {
			omittedChanged = append(omittedChanged, change.Path)
		}
	}

	return diffSummary{
		Repository:           input.Repo.Root,
		Task:                 input.Task,
		GeneratedAt:          time.Now().Format(time.RFC3339),
		DiffBase:             input.DiffBase,
		Budget:               input.Budget,
		EstimatedTokens:      estimatedTokens,
		IndexedFiles:         len(input.Repo.Files),
		Stack:                input.Repo.Stack,
		ConfigPath:           configPath,
		Cache:                buildDiffCacheSummary(stats),
		ChangedFiles:         input.ChangedFiles,
		SelectedChangedFiles: selectedChanged,
		OmittedChangedFiles:  omittedChanged,
		TopFiles:             topFiles,
		Selection:            selection,
		ReviewChecklist:      pack.ReviewChecklist(input),
		Passed:               true,
		MarkdownOutput:       markdownOutput,
	}
}

func buildDiffCacheSummary(stats index.CacheStats) diffCacheSummary {
	return diffCacheSummary{
		Path:         stats.Path,
		Loaded:       stats.Loaded,
		ReusedFiles:  stats.ReusedFiles,
		ParsedFiles:  stats.ParsedFiles,
		RemovedFiles: stats.RemovedFiles,
	}
}

func writeDiffSummaryJSON(path string, summary diffSummary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
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

package pack

import (
	"strings"
	"testing"

	"github.com/ilyaux/ctxpack/internal/budget"
	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/rank"
)

func TestFitToBudgetPromotesOmittedSummariesWhenThereIsRoom(t *testing.T) {
	repo := &index.RepoIndex{
		Root: "repo",
		Files: []index.FileInfo{
			{Path: "a.go", Language: "go", Content: "package a\n"},
			{Path: "b.go", Language: "go", Content: "package b\n"},
		},
	}
	selections := []budget.Selection{
		{
			Scored: rank.ScoredFile{
				File:    repo.Files[0],
				Reasons: []string{"repository entry point"},
			},
			Mode:            budget.ModeSummary,
			EstimatedTokens: budget.SummarySelectionCost(repo.Files[0]),
		},
		{
			Scored: rank.ScoredFile{
				File:    repo.Files[1],
				Reasons: []string{"repository entry point"},
			},
			Mode: budget.ModeOmitted,
		},
	}

	markdown, fitted := FitToBudget(RenderInput{
		Task:       "test task",
		Budget:     5000,
		Repo:       repo,
		Selections: selections,
	})
	if fitted[1].Mode != budget.ModeSummary {
		t.Fatalf("second selection mode = %s, want summary", fitted[1].Mode)
	}
	if !strings.Contains(markdown, "b.go") || strings.Contains(markdown, "Omitted:\n- b.go") {
		t.Fatalf("markdown did not promote b.go summary:\n%s", markdown)
	}
}

func TestRenderMarkdownIncludesGitDiffContext(t *testing.T) {
	repo := &index.RepoIndex{
		Root: "repo",
		Files: []index.FileInfo{
			{Path: "service.go", Language: "go", Content: "package service\n"},
			{Path: "service_test.go", Language: "go", IsTest: true, Content: "package service\n"},
		},
	}
	selections := []budget.Selection{
		{
			Scored: rank.ScoredFile{
				File:       repo.Files[0],
				Score:      120,
				Reasons:    []string{"changed in current diff"},
				Components: []rank.ScoreComponent{{Reason: "changed in current diff", Points: 120}},
			},
			Mode:            budget.ModeSummary,
			EstimatedTokens: budget.SummarySelectionCost(repo.Files[0]),
		},
		{
			Scored: rank.ScoredFile{
				File:       repo.Files[1],
				Score:      8,
				Reasons:    []string{"nearby test for relevant file service.go"},
				Components: []rank.ScoreComponent{{Reason: "nearby test for relevant file service.go", Points: 8}},
			},
			Mode:            budget.ModeSummary,
			EstimatedTokens: budget.SummarySelectionCost(repo.Files[1]),
		},
	}

	markdown := RenderMarkdown(RenderInput{
		Task:       "review service diff",
		Budget:     5000,
		Repo:       repo,
		Selections: selections,
		DiffBase:   "main",
		ChangedFiles: []rank.ChangedFile{
			{Path: "service.go", Status: "modified", Additions: 4, Deletions: 1, Sources: []string{"main...HEAD"}},
		},
	})

	for _, want := range []string{
		"## Git diff context",
		"Diff base: `main`",
		"modified `service.go` +4/-1",
		"Related context selected around the diff:",
		"`service_test.go` via nearby test for relevant file service.go",
		"## Review checklist",
		"approximately +4/-1 lines",
		"Run the nearby tests selected in this pack before broader suites.",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
}

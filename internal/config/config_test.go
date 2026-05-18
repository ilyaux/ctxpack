package config

import (
	"strings"
	"testing"

	"github.com/ilyaux/ctxpack/internal/index"
)

func TestParseConfig(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
budget: 16000
include_tests: false
ignore:
  - target/**
  - vendor/**
priority:
  - services/billing/**
ci:
  base: origin/main
  budget: 9000
  output: .ctxpack/ci.md
  summary_json: reports/ctxpack-ci.json
  fail_on:
    - over-budget
    - missing-tests
  forbid:
    - services/auth/**
languages:
  go: true
  java: false
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Budget != 16000 {
		t.Fatalf("budget = %d", cfg.Budget)
	}
	if cfg.IncludeTests == nil || *cfg.IncludeTests {
		t.Fatalf("include_tests = %#v", cfg.IncludeTests)
	}
	if len(cfg.Ignore) != 2 || len(cfg.Priority) != 1 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if !cfg.Languages["go"] || cfg.Languages["java"] {
		t.Fatalf("languages = %#v", cfg.Languages)
	}
	if cfg.CI.Base != "origin/main" || cfg.CI.Budget != 9000 {
		t.Fatalf("ci base/budget = %#v", cfg.CI)
	}
	if cfg.CI.Output != ".ctxpack/ci.md" || cfg.CI.SummaryJSON != "reports/ctxpack-ci.json" {
		t.Fatalf("ci outputs = %#v", cfg.CI)
	}
	if len(cfg.CI.FailOn) != 2 || cfg.CI.FailOn[1] != "missing-tests" {
		t.Fatalf("ci fail_on = %#v", cfg.CI.FailOn)
	}
	if len(cfg.CI.Forbid) != 1 || cfg.CI.Forbid[0] != "services/auth/**" {
		t.Fatalf("ci forbid = %#v", cfg.CI.Forbid)
	}
}

func TestParseInlineCIRules(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
ci:
  fail_on: over-budget, omitted-changed
  forbid: services/auth/**, apps/admin/**
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CI.FailOn) != 2 || cfg.CI.FailOn[0] != "over-budget" || cfg.CI.FailOn[1] != "omitted-changed" {
		t.Fatalf("ci fail_on = %#v", cfg.CI.FailOn)
	}
	if len(cfg.CI.Forbid) != 2 || cfg.CI.Forbid[1] != "apps/admin/**" {
		t.Fatalf("ci forbid = %#v", cfg.CI.Forbid)
	}
}

func TestApplyFiltersIgnoredPathsAndLanguages(t *testing.T) {
	idx := &index.RepoIndex{Files: []index.FileInfo{
		{Path: "services/billing/main.go", Language: "go"},
		{Path: "target/generated.java", Language: "java"},
		{Path: "web/App.tsx", Language: "typescriptreact"},
	}}
	filtered := Apply(idx, Config{
		Ignore:    []string{"target/**"},
		Languages: map[string]bool{"go": true, "typescript": true},
	})
	if len(filtered.Files) != 2 {
		t.Fatalf("filtered file count = %d", len(filtered.Files))
	}
	if filtered.Files[0].Path != "services/billing/main.go" || filtered.Files[1].Path != "web/App.tsx" {
		t.Fatalf("filtered files = %#v", filtered.Files)
	}
}

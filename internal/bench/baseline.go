package bench

import (
	"sort"
	"strings"

	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/tokens"
)

type BaselineResult struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Budget          int      `json:"budget"`
	EstimatedTokens int      `json:"estimated_tokens"`
	SelectedFiles   int      `json:"selected_files"`
	TopFiles        []string `json:"top_files,omitempty"`
	ExpectedHits    []string `json:"expected_hits,omitempty"`
	MissingExpected []string `json:"missing_expected,omitempty"`
	AvoidHits       []string `json:"avoid_hits,omitempty"`
	Passed          bool     `json:"passed"`
}

func compareBaselines(idx *index.RepoIndex, task Task, budget int) []BaselineResult {
	return []BaselineResult{
		fullRepoBaseline(idx, task, budget),
		repomixStyleBaseline(idx, task, budget),
		lexicalBudgetBaseline(idx, task, budget),
	}
}

func fullRepoBaseline(idx *index.RepoIndex, task Task, budget int) BaselineResult {
	files := make([]string, 0, len(idx.Files))
	estimated := 0
	for _, file := range idx.Files {
		files = append(files, file.Path)
		estimated += tokens.Estimate(file.Content) + 24
	}
	sort.Strings(files)
	expectedHits, missing := matchExpectations(task.Expect, files)
	avoidHits := matchAvoid(task.Avoid, files)
	return BaselineResult{
		Name:            "full-repo",
		Description:     "all indexed files, similar to a repo dump",
		Budget:          budget,
		EstimatedTokens: estimated,
		SelectedFiles:   len(files),
		TopFiles:        firstStrings(files, 15),
		ExpectedHits:    expectedHits,
		MissingExpected: missing,
		AvoidHits:       avoidHits,
		Passed:          estimated <= budget && len(missing) == 0 && len(avoidHits) == 0,
	}
}

func repomixStyleBaseline(idx *index.RepoIndex, task Task, budget int) BaselineResult {
	files := make([]string, 0, len(idx.Files))
	var markdown strings.Builder
	markdown.WriteString("# Repository Pack\n\n")
	for _, file := range idx.Files {
		files = append(files, file.Path)
		markdown.WriteString("## ")
		markdown.WriteString(file.Path)
		markdown.WriteString("\n\n```")
		markdown.WriteString(file.FenceLanguage())
		markdown.WriteString("\n")
		markdown.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			markdown.WriteByte('\n')
		}
		markdown.WriteString("```\n\n")
	}
	sort.Strings(files)
	expectedHits, missing := matchExpectations(task.Expect, files)
	avoidHits := matchAvoid(task.Avoid, files)
	estimated := tokens.Estimate(markdown.String()) + len(files)*24
	return BaselineResult{
		Name:            "repomix-style",
		Description:     "all indexed files concatenated into markdown with file headers",
		Budget:          budget,
		EstimatedTokens: estimated,
		SelectedFiles:   len(files),
		TopFiles:        firstStrings(files, 15),
		ExpectedHits:    expectedHits,
		MissingExpected: missing,
		AvoidHits:       avoidHits,
		Passed:          estimated <= budget && len(missing) == 0 && len(avoidHits) == 0,
	}
}

func lexicalBudgetBaseline(idx *index.RepoIndex, task Task, budget int) BaselineResult {
	terms := benchTerms(task.Task)
	var scored []baselineFile
	for _, file := range idx.Files {
		score := lexicalBaselineScore(file, terms)
		if score > 0 {
			scored = append(scored, baselineFile{path: file.Path, tokens: tokens.Estimate(file.Content) + 24, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].path < scored[j].path
		}
		return scored[i].score > scored[j].score
	})

	var files []string
	estimated := 0
	for _, file := range scored {
		if estimated+file.tokens > budget && len(files) > 0 {
			continue
		}
		estimated += file.tokens
		files = append(files, file.path)
		if estimated >= budget {
			break
		}
	}
	expectedHits, missing := matchExpectations(task.Expect, files)
	avoidHits := matchAvoid(task.Avoid, files)
	return BaselineResult{
		Name:            "lexical-budget",
		Description:     "budgeted naive path/content keyword selection",
		Budget:          budget,
		EstimatedTokens: estimated,
		SelectedFiles:   len(files),
		TopFiles:        firstStrings(files, 15),
		ExpectedHits:    expectedHits,
		MissingExpected: missing,
		AvoidHits:       avoidHits,
		Passed:          estimated <= budget && len(missing) == 0 && len(avoidHits) == 0 && len(files) > 0,
	}
}

type baselineFile struct {
	path   string
	tokens int
	score  int
}

func lexicalBaselineScore(file index.FileInfo, terms []string) int {
	lowerPath := strings.ToLower(file.Path)
	lowerContent := strings.ToLower(file.Content)
	score := 0
	for _, term := range terms {
		if strings.Contains(lowerPath, term) {
			score += 10
		}
		count := strings.Count(lowerContent, term)
		if count > 8 {
			count = 8
		}
		score += count
	}
	if file.IsTest {
		score += 1
	}
	return score
}

func benchTerms(text string) []string {
	raw := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true,
		"add": true, "fix": true, "new": true, "update": true, "change": true,
	}
	seen := map[string]bool{}
	var out []string
	for _, term := range raw {
		if len(term) < 3 || stop[term] || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out
}

func firstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

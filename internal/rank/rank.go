package rank

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/pathspec"
)

type Options struct {
	IncludeTests bool
	DiffFiles    []string
	Mode         string
	Priority     []string
}

type ScoredFile struct {
	File       index.FileInfo
	Score      float64
	Reasons    []string
	Components []ScoreComponent
}

type ScoreComponent struct {
	Reason string  `json:"reason"`
	Points float64 `json:"points"`
}

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true,
	"that": true, "this": true, "into": true, "after": true, "before": true,
	"add": true, "fix": true, "new": true, "update": true, "change": true,
	"make": true, "use": true, "using": true, "please": true, "repo": true,
	"review": true, "implement": true, "create": true, "delete": true, "remove": true,
}

func ScoreFiles(task string, idx *index.RepoIndex, opt Options) []ScoredFile {
	coreTerms := taskTerms(task)
	terms := expandTerms(coreTerms)
	diffSet := make(map[string]bool, len(opt.DiffFiles))
	diffDirs := make(map[string]bool)
	for _, path := range opt.DiffFiles {
		path = filepath.ToSlash(path)
		diffSet[path] = true
		diffDirs[filepath.Dir(path)] = true
	}

	scoredByPath := make(map[string]ScoredFile, len(idx.Files))
	for _, file := range idx.Files {
		score, reasons, components := scoreFile(file, terms, coreTerms, task, opt, diffSet, diffDirs)
		if score > 0 {
			scoredByPath[file.Path] = ScoredFile{File: file, Score: score, Reasons: reasons, Components: components}
		}
	}
	applyRelatedFileBoosts(scoredByPath, idx, opt)

	scored := make([]ScoredFile, 0, len(scoredByPath))
	for _, item := range scoredByPath {
		if item.Score > 0 {
			scored = append(scored, item)
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].File.Path < scored[j].File.Path
		}
		return scored[i].Score > scored[j].Score
	})

	if len(scored) == 0 {
		return fallbackFiles(idx)
	}
	return scored
}

func scoreFile(file index.FileInfo, terms []string, coreTerms []string, task string, opt Options, diffSet map[string]bool, diffDirs map[string]bool) (float64, []string, []ScoreComponent) {
	lowerPath := strings.ToLower(file.Path)
	lowerContent := strings.ToLower(file.Content)
	lowerTask := strings.ToLower(task)
	var score float64
	var reasons []string
	var components []ScoreComponent
	matchedCoreTerms := map[string]bool{}
	routeSymbolBoosted := false
	reactSymbolBoosted := false
	mavenSymbolBoosted := false

	add := func(points float64, reason string) {
		score += points
		components = addComponent(components, reason, points)
		if reason != "" && !contains(reasons, reason) && len(reasons) < 5 {
			reasons = append(reasons, reason)
		}
	}
	recordCoreMatch := func(term string) {
		if contains(coreTerms, term) {
			matchedCoreTerms[term] = true
		}
	}

	if diffSet[file.Path] {
		add(120, "changed in current diff")
	}
	if diffDirs[filepath.Dir(file.Path)] && !diffSet[file.Path] {
		add(12, "near changed files")
	}
	if pathspec.MatchAny(opt.Priority, file.Path) {
		add(24, "matches configured priority path")
	}

	for _, term := range terms {
		if strings.Contains(lowerPath, term) {
			add(14, "path matches task term "+term)
			recordCoreMatch(term)
		}
		symbolMatched := false
		signatureMatched := false
		for _, sym := range file.Symbols {
			name := strings.ToLower(sym.Name)
			sig := strings.ToLower(sym.Signature)
			kind := strings.ToLower(sym.Kind)
			if strings.Contains(name, term) && !symbolMatched {
				add(18, "symbol matches task term "+term)
				recordCoreMatch(term)
				symbolMatched = true
			} else if strings.Contains(sig, term) && !signatureMatched {
				add(8, "signature matches task term "+term)
				recordCoreMatch(term)
				signatureMatched = true
			}
			if symbolMatched && signatureMatched {
				break
			}
			if !routeSymbolBoosted && mentionsAny(lowerTask, "endpoint", "route", "api", "handler", "controller") && (kind == "route" || kind == "handler" || kind == "controller") {
				add(5, "route symbol boundary")
				routeSymbolBoosted = true
			}
			if !reactSymbolBoosted && mentionsAny(lowerTask, "react", "frontend", "component", "hook", "page") && (kind == "component" || kind == "hook") {
				add(5, "React symbol boundary")
				reactSymbolBoosted = true
			}
			if !mavenSymbolBoosted && mentionsAny(lowerTask, "maven", "pom", "module", "dependency", "dependencies") && strings.HasPrefix(kind, "maven-") {
				add(5, "Maven symbol boundary")
				mavenSymbolBoosted = true
			}
		}
		importMatched := false
		for _, imported := range file.Imports {
			if strings.Contains(strings.ToLower(imported), term) && !importMatched {
				add(5, "import matches task term "+term)
				recordCoreMatch(term)
				importMatched = true
				break
			}
		}
		count := strings.Count(lowerContent, term)
		if count > 0 {
			if count > 10 {
				count = 10
			}
			add(float64(count)*1.5, "content mentions task term "+term)
			recordCoreMatch(term)
		}
	}

	if len(matchedCoreTerms) >= 2 {
		add(float64(len(matchedCoreTerms)-1)*9, "matches multiple task concepts")
	} else if len(coreTerms) >= 4 && len(matchedCoreTerms) == 1 && score > 0 {
		for term := range matchedCoreTerms {
			if isNoisySingleTerm(term) {
				before := score
				score *= 0.7
				components = addComponent(components, "broad single-term penalty "+term, score-before)
				if !contains(reasons, "only matches broad task term "+term) && len(reasons) < 5 {
					reasons = append(reasons, "only matches broad task term "+term)
				}
			}
		}
	}

	if mentionsAny(lowerTask, "endpoint", "route", "api", "webhook", "handler") && file.IsRoute {
		add(18, "route/API file for endpoint-like task")
	}
	if mentionsAny(lowerTask, "controller", "resource", "endpoint", "rest") &&
		(file.Language == "java" && file.IsRoute) {
		add(16, "Java controller/resource boundary")
	}
	if mentionsAny(lowerTask, "service", "business", "logic") &&
		(file.Language == "java" && strings.Contains(lowerPath, "service")) {
		add(12, "Java service layer")
	}
	if mentionsAny(lowerTask, "dao", "repository", "database", "query", "sql") &&
		(file.Language == "java" || file.Language == "sql") &&
		(strings.Contains(lowerPath, "dao") || strings.Contains(lowerPath, "repository") || file.Language == "sql") {
		add(12, "database access boundary")
	}
	if mentionsAny(lowerTask, "jsf", "xhtml", "page", "view", "screen", "form") &&
		(file.Language == "xhtml" || file.Language == "html") {
		add(12, "view/template boundary")
	}
	if mentionsAny(lowerTask, "frontend", "ui", "react", "page", "component", "screen") &&
		(file.Language == "typescriptreact" || file.Language == "xhtml" || file.Language == "html" || strings.Contains(lowerPath, "/pages/") || strings.Contains(lowerPath, "/components/")) {
		add(14, "frontend/UI boundary")
	}
	if mentionsAny(lowerTask, "client", "sdk", "dto", "request", "response") &&
		(strings.Contains(lowerPath, "api-client") || strings.Contains(lowerPath, "client") || strings.Contains(lowerPath, "types")) {
		add(12, "client/types contract")
	}
	if mentionsAny(lowerTask, "test", "tests", "bug", "fix", "regression") && file.IsTest {
		add(10, "test file")
	}
	if mentionsAny(lowerTask, "go", "backend", "service") && file.Language == "go" {
		add(4, "backend language match")
	}
	if mentionsAny(lowerTask, "java", "backend", "ejb", "service") && file.Language == "java" {
		add(4, "Java/backend language match")
	}
	if mentionsAny(lowerTask, "typescript", "react", "frontend") &&
		(file.Language == "typescript" || file.Language == "typescriptreact") {
		add(4, "frontend language match")
	}
	if file.IsConfig && mentionsAny(lowerTask, "workspace", "package", "build", "config") {
		add(7, "configuration file")
	}
	if file.IsConfig && mentionsAny(lowerTask, "maven", "pom", "module", "dependency", "dependencies") {
		if filepath.Base(lowerPath) == "pom.xml" {
			add(22, "Maven dependency manifest")
		} else {
			add(8, "dependency/configuration file")
		}
	}
	if mentionsAny(lowerTask, "maven", "pom", "module", "dependency", "dependencies") &&
		file.Language == "xml" && filepath.Base(lowerPath) == "pom.xml" {
		add(12, "Maven project graph")
	}

	return score, reasons, components
}

func isNoisySingleTerm(term string) bool {
	switch term {
	case "log", "report", "data", "user", "service", "page", "bean":
		return true
	default:
		return false
	}
}

func fallbackFiles(idx *index.RepoIndex) []ScoredFile {
	preferred := map[string]bool{
		"README.md":           true,
		"readme.md":           true,
		"go.mod":              true,
		"pom.xml":             true,
		"package.json":        true,
		"pnpm-workspace.yaml": true,
		"tsconfig.json":       true,
	}
	var out []ScoredFile
	for _, file := range idx.Files {
		if preferred[filepath.Base(file.Path)] {
			out = append(out, ScoredFile{
				File:       file,
				Score:      1,
				Reasons:    []string{"repository entry point"},
				Components: []ScoreComponent{{Reason: "repository entry point", Points: 1}},
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].File.Path < out[j].File.Path
	})
	return out
}

func taskTerms(task string) []string {
	raw := wordRe.FindAllString(strings.ToLower(task), -1)
	var terms []string
	seen := map[string]bool{}
	for _, term := range raw {
		if len(term) < 3 || stopWords[term] || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func expandTerms(terms []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(term string) {
		if term != "" && !seen[term] {
			seen[term] = true
			out = append(out, term)
		}
	}
	for _, term := range terms {
		add(term)
		switch term {
		case "endpoint":
			add("route")
			add("handler")
			add("api")
		case "webhook":
			add("handler")
			add("retry")
		case "frontend":
			add("react")
			add("page")
			add("component")
		case "commission":
			add("fee")
			add("rate")
		case "oauth":
			add("auth")
			add("callback")
			add("login")
		case "transaction":
			add("wallet")
			add("ledger")
		}
	}
	return out
}

func mentionsAny(text string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func addComponent(components []ScoreComponent, reason string, points float64) []ScoreComponent {
	if reason == "" || points == 0 {
		return components
	}
	for i := range components {
		if components[i].Reason == reason {
			components[i].Points += points
			return components
		}
	}
	return append(components, ScoreComponent{Reason: reason, Points: points})
}

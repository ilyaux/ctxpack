package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ilyaux/ctxpack/internal/pathspec"
)

type diffCheckResult struct {
	Name    string   `json:"name"`
	Passed  bool     `json:"passed"`
	Details []string `json:"details,omitempty"`
}

func evaluateDiffChecks(summary diffSummary, failOnRaw string, forbidRaw string) ([]diffCheckResult, error) {
	checks, err := parseDiffCheckNames(failOnRaw, forbidRaw != "")
	if err != nil {
		return nil, err
	}
	if len(checks) == 0 {
		return nil, nil
	}
	forbiddenPatterns := parseCSV(forbidRaw)

	var results []diffCheckResult
	for _, name := range checks {
		switch name {
		case "over-budget":
			results = append(results, checkOverBudget(summary))
		case "omitted-changed":
			results = append(results, checkOmittedChanged(summary))
		case "missing-tests":
			results = append(results, checkMissingTests(summary))
		case "forbidden-paths":
			if len(forbiddenPatterns) == 0 {
				return nil, fmt.Errorf("--fail-on forbidden-paths requires --forbid patterns")
			}
			results = append(results, checkForbiddenPaths(summary, forbiddenPatterns))
		default:
			return nil, fmt.Errorf("unknown diff check %q", name)
		}
	}
	return results, nil
}

func parseDiffCheckNames(raw string, hasForbiddenPatterns bool) ([]string, error) {
	values := parseCSV(raw)
	if len(values) == 0 {
		return nil, nil
	}
	enabled := map[string]bool{}
	for _, value := range values {
		switch normalizeDiffCheckName(value) {
		case "all":
			enabled["over-budget"] = true
			enabled["omitted-changed"] = true
			enabled["missing-tests"] = true
			if hasForbiddenPatterns {
				enabled["forbidden-paths"] = true
			}
		case "over-budget", "omitted-changed", "missing-tests", "forbidden-paths":
			enabled[normalizeDiffCheckName(value)] = true
		default:
			return nil, fmt.Errorf("unknown diff check %q", value)
		}
	}
	order := []string{"over-budget", "omitted-changed", "missing-tests", "forbidden-paths"}
	var out []string
	for _, name := range order {
		if enabled[name] {
			out = append(out, name)
		}
	}
	return out, nil
}

func normalizeDiffCheckName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "-")
	switch name {
	case "budget", "tokens", "token-budget":
		return "over-budget"
	case "omitted", "omitted-files", "omitted-changes":
		return "omitted-changed"
	case "tests", "no-tests", "missing-test":
		return "missing-tests"
	case "forbidden", "forbid", "forbidden-path":
		return "forbidden-paths"
	default:
		return name
	}
}

func checkOverBudget(summary diffSummary) diffCheckResult {
	result := diffCheckResult{Name: "over-budget", Passed: summary.EstimatedTokens <= summary.Budget}
	if !result.Passed {
		result.Details = []string{fmt.Sprintf("estimated tokens %d exceed budget %d", summary.EstimatedTokens, summary.Budget)}
	}
	return result
}

func checkOmittedChanged(summary diffSummary) diffCheckResult {
	result := diffCheckResult{Name: "omitted-changed", Passed: len(summary.OmittedChangedFiles) == 0}
	if !result.Passed {
		result.Details = []string{"omitted changed files: " + strings.Join(summary.OmittedChangedFiles, ", ")}
	}
	return result
}

func checkMissingTests(summary diffSummary) diffCheckResult {
	for _, sel := range summary.Selection {
		if sel.IsTest {
			return diffCheckResult{Name: "missing-tests", Passed: true}
		}
	}
	return diffCheckResult{Name: "missing-tests", Passed: false, Details: []string{"no selected test files"}}
}

func checkForbiddenPaths(summary diffSummary, patterns []string) diffCheckResult {
	var hits []string
	for _, change := range summary.ChangedFiles {
		if pattern := matchedPattern(patterns, change.Path); pattern != "" {
			hits = append(hits, fmt.Sprintf("changed %s matches %s", change.Path, pattern))
		}
		if change.OldPath != "" {
			if pattern := matchedPattern(patterns, change.OldPath); pattern != "" {
				hits = append(hits, fmt.Sprintf("changed %s matches %s", change.OldPath, pattern))
			}
		}
	}
	for _, sel := range summary.Selection {
		if pattern := matchedPattern(patterns, sel.Path); pattern != "" {
			hits = append(hits, fmt.Sprintf("selected %s matches %s", sel.Path, pattern))
		}
	}
	hits = uniqueStrings(hits)
	return diffCheckResult{Name: "forbidden-paths", Passed: len(hits) == 0, Details: hits}
}

func matchedPattern(patterns []string, path string) string {
	for _, pattern := range patterns {
		if pathspec.Match(pattern, path) {
			return pattern
		}
	}
	return ""
}

func parseCSV(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func applyDiffCheckResults(summary *diffSummary, checks []diffCheckResult) error {
	summary.Checks = checks
	summary.Passed = true
	var failed []string
	for _, check := range checks {
		if check.Passed {
			continue
		}
		summary.Passed = false
		if len(check.Details) == 0 {
			failed = append(failed, check.Name)
		} else {
			failed = append(failed, check.Name+": "+strings.Join(check.Details, "; "))
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return fmt.Errorf("diff checks failed: %s", strings.Join(failed, " | "))
}

func printDiffChecks(w io.Writer, checks []diffCheckResult) {
	if len(checks) == 0 {
		return
	}
	for _, check := range checks {
		status := "PASS"
		if !check.Passed {
			status = "FAIL"
		}
		if len(check.Details) == 0 {
			fmt.Fprintf(w, "%s check %s\n", status, check.Name)
			continue
		}
		fmt.Fprintf(w, "%s check %s: %s\n", status, check.Name, strings.Join(check.Details, "; "))
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

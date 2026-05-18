package pack

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ilyaux/ctxpack/internal/budget"
	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/rank"
	"github.com/ilyaux/ctxpack/internal/tokens"
)

type RenderInput struct {
	Task         string
	Budget       int
	Repo         *index.RepoIndex
	Selections   []budget.Selection
	DiffBase     string
	ChangedFiles []rank.ChangedFile
}

func RenderMarkdown(input RenderInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Context Pack: %s\n\n", input.Task)
	fmt.Fprintf(&b, "Token budget: %d\n", input.Budget)
	fmt.Fprintf(&b, "Repository: %s\n", filepath.Base(input.Repo.Root))
	fmt.Fprintf(&b, "Generated: %s\n\n", time.Now().Format(time.RFC3339))

	if input.DiffBase != "" && len(input.ChangedFiles) == 0 {
		fmt.Fprintf(&b, "Diff base: %s\n\n", input.DiffBase)
	}

	writeTaskInterpretation(&b, input)
	writeArchitecture(&b, input.Repo)
	writeGitDiffContext(&b, input)
	writeReviewChecklist(&b, input)
	writeFilesToRead(&b, input.Selections)
	writeSelectionRationale(&b, input.Selections)
	writeImportantSymbols(&b, input.Selections)
	writePlan(&b, input.Task)
	writeBudgetDecisions(&b, input.Selections)
	writeDoNotTouch(&b, input.Repo, input.Selections)
	writeContextFiles(&b, input.Selections)

	return b.String()
}

func writeGitDiffContext(b *strings.Builder, input RenderInput) {
	if input.DiffBase == "" && len(input.ChangedFiles) == 0 {
		return
	}

	fmt.Fprintf(b, "## Git diff context\n\n")
	if input.DiffBase != "" {
		fmt.Fprintf(b, "Diff base: `%s`\n\n", input.DiffBase)
	}
	if len(input.ChangedFiles) == 0 {
		fmt.Fprintf(b, "No changed files were provided.\n\n")
		return
	}

	selected := selectedPathSet(input.Selections)
	changedSelected := 0
	for _, change := range input.ChangedFiles {
		if selected[change.Path] {
			changedSelected++
		}
	}
	fmt.Fprintf(b, "Changed files: %d. Selected changed files: %d.\n\n", len(input.ChangedFiles), changedSelected)
	fmt.Fprintf(b, "Changed file list:\n")
	limit := diffListLimit(input.Budget)
	for i, change := range input.ChangedFiles {
		if i >= limit {
			fmt.Fprintf(b, "- ... %d more changed files omitted from this list.\n", len(input.ChangedFiles)-limit)
			break
		}
		fmt.Fprintf(b, "- %s\n", formatChangedFile(change))
	}
	fmt.Fprintf(b, "\n")

	var related []string
	for _, sel := range input.Selections {
		if sel.Mode == budget.ModeOmitted || selectedDiffPath(input.ChangedFiles, sel.Scored.File.Path) {
			continue
		}
		if reason := diffSelectionReason(sel); reason != "" {
			related = append(related, fmt.Sprintf("- `%s` via %s", sel.Scored.File.Path, reason))
		}
		if len(related) >= 10 {
			break
		}
	}
	if len(related) == 0 {
		return
	}
	fmt.Fprintf(b, "Related context selected around the diff:\n")
	for _, item := range related {
		fmt.Fprintf(b, "%s\n", item)
	}
	fmt.Fprintf(b, "\n")
}

func writeReviewChecklist(b *strings.Builder, input RenderInput) {
	if len(input.ChangedFiles) == 0 {
		return
	}
	items := ReviewChecklist(input)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "## Review checklist\n\n")
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
	fmt.Fprintf(b, "\n")
}

func ReviewChecklist(input RenderInput) []string {
	return reviewChecklist(input)
}

func writeSelectionRationale(b *strings.Builder, selections []budget.Selection) {
	fmt.Fprintf(b, "## Selection rationale\n\n")
	count := 0
	for _, sel := range selections {
		if sel.Mode == budget.ModeOmitted {
			continue
		}
		count++
		fmt.Fprintf(b, "- `%s` score %.1f, mode `%s`: %s\n",
			sel.Scored.File.Path,
			sel.Scored.Score,
			sel.Mode,
			componentSummary(sel.Scored.Components, 3),
		)
		if count >= 10 {
			break
		}
	}
	if count == 0 {
		fmt.Fprintf(b, "No selected files to explain.\n")
	}
	fmt.Fprintf(b, "\n")
}

func FitToBudget(input RenderInput) (string, []budget.Selection) {
	selections := input.Selections
	input.Selections = selections
	markdown := RenderMarkdown(input)
	for tokens.Estimate(markdown) > input.Budget && budget.DowngradeLeastImportant(selections) {
		input.Selections = selections
		markdown = RenderMarkdown(input)
	}

	for {
		promoted := false
		for i := range selections {
			if selections[i].Mode != budget.ModeOmitted {
				continue
			}
			old := selections[i]
			selections[i].Mode = budget.ModeSummary
			selections[i].EstimatedTokens = budget.SummarySelectionCost(selections[i].Scored.File)
			input.Selections = selections
			trial := RenderMarkdown(input)
			if tokens.Estimate(trial) <= input.Budget {
				markdown = trial
				promoted = true
				break
			}
			selections[i] = old
		}
		if !promoted {
			break
		}
	}
	return markdown, selections
}

func writeTaskInterpretation(b *strings.Builder, input RenderInput) {
	fmt.Fprintf(b, "## Task interpretation\n\n")
	items := interpretTask(input.Task)
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
	fmt.Fprintf(b, "\n")
}

func writeArchitecture(b *strings.Builder, repo *index.RepoIndex) {
	fmt.Fprintf(b, "## Relevant architecture\n\n")
	if len(repo.Stack.Languages) > 0 {
		fmt.Fprintf(b, "Detected stack: %s\n\n", strings.Join(repo.Stack.Languages, ", "))
	}
	if len(repo.Stack.PackageManagers) > 0 {
		fmt.Fprintf(b, "Package managers: %s\n\n", strings.Join(repo.Stack.PackageManagers, ", "))
	}
	if len(repo.Stack.GoModules) > 0 {
		fmt.Fprintf(b, "Go modules:\n")
		for _, module := range repo.Stack.GoModules {
			fmt.Fprintf(b, "- %s\n", module)
		}
		fmt.Fprintf(b, "\n")
	}

	areas := topAreas(repo.Files)
	if len(areas) > 0 {
		fmt.Fprintf(b, "Repository areas:\n")
		for _, area := range areas {
			fmt.Fprintf(b, "- %s\n", area)
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeFilesToRead(b *strings.Builder, selections []budget.Selection) {
	fmt.Fprintf(b, "## Files to read first\n\n")
	count := 0
	for _, sel := range selections {
		if sel.Mode == budget.ModeOmitted {
			continue
		}
		count++
		fmt.Fprintf(b, "%d. %s\n", count, sel.Scored.File.Path)
		fmt.Fprintf(b, "   Why: %s.\n\n", reasons(sel.Scored.Reasons))
		if count >= 15 {
			break
		}
	}
	if count == 0 {
		fmt.Fprintf(b, "No relevant files fit the budget.\n\n")
	}
}

func writeImportantSymbols(b *strings.Builder, selections []budget.Selection) {
	fmt.Fprintf(b, "## Important symbols\n\n")
	wrote := false
	for _, sel := range selections {
		if sel.Mode == budget.ModeOmitted || len(sel.Scored.File.Symbols) == 0 {
			continue
		}
		fmt.Fprintf(b, "### %s\n\n", sel.Scored.File.Path)
		for i, sym := range sel.Scored.File.Symbols {
			if i >= 12 {
				fmt.Fprintf(b, "- ...\n")
				break
			}
			line := ""
			if sym.Line > 0 {
				line = fmt.Sprintf(":%d", sym.Line)
			}
			fmt.Fprintf(b, "- `%s` %s%s\n", sym.Name, sym.Kind, line)
		}
		fmt.Fprintf(b, "\n")
		wrote = true
	}
	if !wrote {
		fmt.Fprintf(b, "No symbols were extracted for the selected files.\n\n")
	}
}

func writePlan(b *strings.Builder, task string) {
	fmt.Fprintf(b, "## Suggested implementation plan\n\n")
	for _, step := range suggestedPlan(task) {
		fmt.Fprintf(b, "- %s\n", step)
	}
	fmt.Fprintf(b, "\n")
}

func writeBudgetDecisions(b *strings.Builder, selections []budget.Selection) {
	fmt.Fprintf(b, "## Budget decisions\n\n")
	groups := []budget.Mode{budget.ModeFull, budget.ModeSlices, budget.ModeSignatures, budget.ModeSummary, budget.ModeOmitted}
	for _, mode := range groups {
		var items []budget.Selection
		for _, sel := range selections {
			if sel.Mode == mode {
				items = append(items, sel)
			}
		}
		if len(items) == 0 {
			continue
		}
		if mode == budget.ModeOmitted {
			fmt.Fprintf(b, "Omitted:\n")
		} else {
			fmt.Fprintf(b, "Included %s:\n", mode)
		}
		for _, sel := range items {
			if mode == budget.ModeOmitted {
				fmt.Fprintf(b, "- %s. Reason: lower priority or did not fit remaining budget.\n", sel.Scored.File.Path)
			} else {
				fmt.Fprintf(b, "- %s (~%d tokens). Reason: %s.\n", sel.Scored.File.Path, sel.EstimatedTokens, reasons(sel.Scored.Reasons))
			}
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeDoNotTouch(b *strings.Builder, repo *index.RepoIndex, selections []budget.Selection) {
	selected := map[string]bool{}
	for _, sel := range selections {
		if sel.Mode != budget.ModeOmitted {
			selected[topDir(sel.Scored.File.Path)] = true
		}
	}
	candidates := map[string]bool{}
	for _, file := range repo.Files {
		dir := topDir(file.Path)
		if dir != "" && !selected[dir] {
			candidates[dir] = true
		}
	}
	var dirs []string
	for dir := range candidates {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	if len(dirs) > 8 {
		dirs = dirs[:8]
	}

	fmt.Fprintf(b, "## Do not touch unless needed\n\n")
	if len(dirs) == 0 {
		fmt.Fprintf(b, "- No unrelated top-level areas detected.\n\n")
		return
	}
	for _, dir := range dirs {
		if strings.Contains(filepath.Base(dir), ".") {
			fmt.Fprintf(b, "- %s\n", dir)
		} else {
			fmt.Fprintf(b, "- %s/*\n", dir)
		}
	}
	fmt.Fprintf(b, "\n")
}

func writeContextFiles(b *strings.Builder, selections []budget.Selection) {
	fmt.Fprintf(b, "## Context files\n\n")
	for _, sel := range selections {
		if sel.Mode == budget.ModeOmitted {
			continue
		}
		file := sel.Scored.File
		fmt.Fprintf(b, "### %s\n\n", file.Path)
		fmt.Fprintf(b, "Mode: %s\n\n", sel.Mode)
		fmt.Fprintf(b, "Why: %s.\n\n", reasons(sel.Scored.Reasons))
		switch sel.Mode {
		case budget.ModeFull:
			writeFence(b, file.FenceLanguage(), file.Content)
		case budget.ModeSlices:
			writeFence(b, file.FenceLanguage(), budget.SliceBlock(sel.Scored))
		case budget.ModeSignatures:
			writeFence(b, file.FenceLanguage(), budget.SignatureBlock(file))
		case budget.ModeSummary:
			fmt.Fprintf(b, "%s\n\n", budget.Summary(file))
		}
	}
}

func writeFence(b *strings.Builder, lang string, content string) {
	if strings.TrimSpace(content) == "" {
		fmt.Fprintf(b, "_Empty file._\n\n")
		return
	}
	fmt.Fprintf(b, "````%s\n%s\n````\n\n", lang, strings.TrimRight(content, "\n"))
}

func interpretTask(task string) []string {
	lower := strings.ToLower(task)
	items := []string{"identify the smallest set of files that can explain and implement the requested change"}
	if containsAny(lower, "endpoint", "route", "api", "webhook") {
		items = append(items, "find route definitions, handlers, request/response contracts, and nearby tests")
	}
	if containsAny(lower, "frontend", "ui", "react", "page", "screen") {
		items = append(items, "include frontend entry points, API client calls, and shared UI/types only where relevant")
	}
	if containsAny(lower, "test", "bug", "fix", "regression") {
		items = append(items, "include nearby tests or regression coverage before unrelated implementation files")
	}
	if containsAny(lower, "diff", "review", "pr") {
		items = append(items, "center the pack on changed files and the contracts they depend on")
	}
	return items
}

func suggestedPlan(task string) []string {
	lower := strings.ToLower(task)
	steps := []string{
		"Read the files listed first before scanning unrelated directories.",
		"Confirm the existing boundary and naming pattern before adding new code.",
	}
	if containsAny(lower, "endpoint", "route", "api", "webhook") {
		steps = append(steps, "Add or update the handler, route registration, DTO/types, and route-level tests together.")
	}
	if containsAny(lower, "frontend", "ui", "react", "page", "screen") {
		steps = append(steps, "Update the API client before wiring UI state so frontend code uses the existing contract style.")
	}
	steps = append(steps,
		"Run the smallest relevant test target first.",
		"Avoid touching omitted areas unless a selected file proves the dependency is real.",
	)
	return steps
}

func topAreas(files []index.FileInfo) []string {
	counts := map[string]int{}
	for _, file := range files {
		dir := topDir(file.Path)
		if dir != "" {
			counts[dir]++
		}
	}
	type area struct {
		name  string
		count int
	}
	var areas []area
	for name, count := range counts {
		areas = append(areas, area{name: name, count: count})
	}
	sort.SliceStable(areas, func(i, j int) bool {
		if areas[i].count == areas[j].count {
			return areas[i].name < areas[j].name
		}
		return areas[i].count > areas[j].count
	})
	limit := len(areas)
	if limit > 10 {
		limit = 10
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, fmt.Sprintf("%s (%d files)", areas[i].name, areas[i].count))
	}
	return out
}

func topDir(path string) string {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return filepath.Base(parts[0])
	}
	return parts[0]
}

func reasons(items []string) string {
	if len(items) == 0 {
		return "best structural match for the task"
	}
	if len(items) > 3 {
		items = items[:3]
	}
	return strings.Join(items, "; ")
}

func componentSummary(components []rank.ScoreComponent, limit int) string {
	if len(components) == 0 {
		return "no score components recorded"
	}
	components = topComponents(components, limit)
	parts := make([]string, 0, len(components))
	for _, component := range components {
		parts = append(parts, fmt.Sprintf("%+.1f %s", component.Points, component.Reason))
	}
	return strings.Join(parts, "; ")
}

func topComponents(components []rank.ScoreComponent, limit int) []rank.ScoreComponent {
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

func selectedPathSet(selections []budget.Selection) map[string]bool {
	out := map[string]bool{}
	for _, sel := range selections {
		if sel.Mode != budget.ModeOmitted {
			out[sel.Scored.File.Path] = true
		}
	}
	return out
}

func selectedDiffPath(changes []rank.ChangedFile, path string) bool {
	for _, change := range changes {
		if change.Path == path {
			return true
		}
	}
	return false
}

func diffListLimit(budget int) int {
	switch {
	case budget > 0 && budget <= 4000:
		return 15
	case budget > 0 && budget <= 8000:
		return 25
	default:
		return 40
	}
}

func formatChangedFile(change rank.ChangedFile) string {
	label := change.Status
	if label == "" {
		label = "changed"
	}
	var detail string
	if change.OldPath != "" && change.OldPath != change.Path {
		detail = fmt.Sprintf("`%s` -> `%s`", change.OldPath, change.Path)
	} else {
		detail = fmt.Sprintf("`%s`", change.Path)
	}
	if stats := diffStatsLabel(change); stats != "" {
		detail += " " + stats
	}
	if len(change.Sources) > 0 {
		return fmt.Sprintf("%s %s (%s)", label, detail, strings.Join(change.Sources, ", "))
	}
	return fmt.Sprintf("%s %s", label, detail)
}

func diffStatsLabel(change rank.ChangedFile) string {
	if change.Binary {
		return "binary"
	}
	if change.Additions == 0 && change.Deletions == 0 {
		return ""
	}
	return fmt.Sprintf("+%d/-%d", change.Additions, change.Deletions)
}

func reviewChecklist(input RenderInput) []string {
	selected := selectedPathSet(input.Selections)
	files := fileByPath(input.Repo.Files)
	var additions int
	var deletions int
	var untracked bool
	var deletedOrRenamed bool
	var omittedChanged int
	var hasRoute bool
	var hasDatabase bool
	var hasFrontend bool
	var hasTests bool

	for _, change := range input.ChangedFiles {
		additions += change.Additions
		deletions += change.Deletions
		if change.Status == "untracked" {
			untracked = true
		}
		if change.Status == "deleted" || change.Status == "renamed" {
			deletedOrRenamed = true
		}
		if !selected[change.Path] {
			omittedChanged++
		}
		file := files[change.Path]
		lang := file.Language
		if lang == "" {
			lang = languageFromPath(change.Path)
		}
		if file.IsRoute {
			hasRoute = true
		}
		if file.IsTest {
			hasTests = true
		}
		if isDatabaseFile(change.Path, lang) {
			hasDatabase = true
		}
		if isFrontendFile(change.Path, lang) {
			hasFrontend = true
		}
	}
	for _, sel := range input.Selections {
		if sel.Mode == budget.ModeOmitted {
			continue
		}
		file := sel.Scored.File
		if file.IsTest {
			hasTests = true
		}
		if file.IsRoute {
			hasRoute = true
		}
		if isDatabaseFile(file.Path, file.Language) {
			hasDatabase = true
		}
		if isFrontendFile(file.Path, file.Language) {
			hasFrontend = true
		}
	}

	items := []string{
		fmt.Sprintf("Review changed files before related context; this pack summarizes approximately +%d/-%d lines.", additions, deletions),
	}
	if untracked {
		items = append(items, "Confirm untracked files are intentionally part of the patch or add them to ignore rules.")
	}
	if omittedChanged > 0 {
		items = append(items, fmt.Sprintf("%d changed file(s) did not fit the selected context; raise the budget or inspect them manually before approval.", omittedChanged))
	}
	if deletedOrRenamed {
		items = append(items, "Check imports, references, generated metadata, and build files for deleted or renamed paths.")
	}
	if hasRoute {
		items = append(items, "For API or route changes, verify request/response contracts, callers, and backward compatibility.")
	}
	if hasDatabase {
		items = append(items, "For database or query changes, verify migrations, transaction boundaries, and affected tests.")
	}
	if hasFrontend {
		items = append(items, "For frontend or template changes, verify field names, validation states, and rendered behavior.")
	}
	if hasTests {
		items = append(items, "Run the nearby tests selected in this pack before broader suites.")
	} else {
		items = append(items, "No nearby tests were selected; identify the smallest relevant test target before relying on the patch.")
	}
	return items
}

func fileByPath(files []index.FileInfo) map[string]index.FileInfo {
	out := map[string]index.FileInfo{}
	for _, file := range files {
		out[file.Path] = file
	}
	return out
}

func languageFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".sql":
		return "sql"
	case ".xhtml":
		return "xhtml"
	case ".html", ".htm":
		return "html"
	default:
		return ""
	}
}

func isDatabaseFile(path string, language string) bool {
	lower := strings.ToLower(path)
	return language == "sql" ||
		strings.Contains(lower, "/dao/") ||
		strings.Contains(lower, "/repository/") ||
		strings.Contains(lower, "migration") ||
		strings.Contains(lower, "schema")
}

func isFrontendFile(path string, language string) bool {
	lower := strings.ToLower(path)
	switch language {
	case "typescriptreact", "javascriptreact", "xhtml", "html", "css":
		return true
	default:
		return strings.Contains(lower, "/pages/") ||
			strings.Contains(lower, "/components/") ||
			strings.Contains(lower, "/views/")
	}
}

func diffSelectionReason(sel budget.Selection) string {
	for _, reason := range sel.Scored.Reasons {
		if strings.Contains(reason, "relevant file") ||
			strings.Contains(reason, "nearby test") ||
			strings.Contains(reason, "near changed files") {
			return reason
		}
	}
	for _, component := range topComponents(sel.Scored.Components, 4) {
		if strings.Contains(component.Reason, "relevant file") ||
			strings.Contains(component.Reason, "nearby test") ||
			strings.Contains(component.Reason, "near changed files") {
			return component.Reason
		}
	}
	return ""
}

func containsAny(text string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

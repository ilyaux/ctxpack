package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ilyaux/ctxpack/internal/bench"
	"github.com/ilyaux/ctxpack/internal/budget"
	"github.com/ilyaux/ctxpack/internal/config"
	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/mcp"
	"github.com/ilyaux/ctxpack/internal/pack"
	"github.com/ilyaux/ctxpack/internal/rank"
	"github.com/ilyaux/ctxpack/internal/tokens"
	"github.com/ilyaux/ctxpack/internal/version"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}

	var err error
	switch args[0] {
	case "index":
		err = runIndex(args[1:], stdout)
	case "pack":
		err = runPack(args[1:], stdout, false)
	case "diff":
		err = runDiff(args[1:], stdout, false)
	case "ci":
		err = runCI(args[1:], stdout)
	case "bench":
		err = runBench(args[1:], stdout)
	case "mcp":
		err = runMCP(args[1:], stdout, stderr)
	case "explain":
		err = runExplain(args[1:], stdout)
	case "version":
		err = runVersion(args[1:], stdout)
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}

	if err != nil {
		fmt.Fprintf(stderr, "ctxpack: %v\n", err)
		return 1
	}
	return 0
}

func runVersion(args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("version does not accept arguments")
	}
	fmt.Fprintln(stdout, version.String())
	return nil
}

func runCI(args []string, stdout io.Writer) error {
	return runDiff(args, stdout, true)
}

func runIndex(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoRoot := fs.String("repo", ".", "repository root")
	output := fs.String("output", ".ctxpack/index.sqlite", "index output path")
	if err := fs.Parse(flexibleFlagArgs(args, map[string]bool{})); err != nil {
		return err
	}

	idx, stats, err := index.BuildCached(*repoRoot)
	if err != nil {
		return err
	}
	outputPath := resolveOutput(idx.Root, *output)
	if err := index.Save(idx, outputPath); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Indexed %d files\n", len(idx.Files))
	fmt.Fprintf(stdout, "Detected stack: %s\n", joinOrNone(idx.Stack.Languages))
	printCacheStats(stdout, stats)
	fmt.Fprintf(stdout, "Wrote %s\n", outputPath)
	return nil
}

func runPack(args []string, stdout io.Writer, diffMode bool) error {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoRoot := fs.String("repo", ".", "repository root")
	tokenBudget := fs.Int("budget", 12000, "hard token budget")
	output := fs.String("output", "", "context pack output path")
	includeTests := fs.Bool("include-tests", true, "boost nearby test files")
	format := fs.String("format", "markdown", "output format: markdown")
	stdoutOnly := fs.Bool("stdout", false, "write pack to stdout")
	if err := fs.Parse(flexibleFlagArgs(args, map[string]bool{
		"include-tests": true,
		"stdout":        true,
	})); err != nil {
		return err
	}
	if *format != "markdown" && *format != "claude" {
		return fmt.Errorf("unsupported format %q", *format)
	}
	task := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if task == "" {
		return errors.New("pack requires a task, for example: ctxpack pack \"add billing retry\"")
	}

	idx, stats, err := index.BuildCached(*repoRoot)
	if err != nil {
		return err
	}
	if err := index.Save(idx, stats.Path); err != nil {
		return err
	}
	cfg, cfgPath, err := config.Load(idx.Root)
	if err != nil {
		return err
	}
	idx = config.Apply(idx, cfg)
	if !flagPresent(args, "budget") && cfg.Budget > 0 {
		*tokenBudget = cfg.Budget
	}
	if !flagPresent(args, "include-tests") && cfg.IncludeTests != nil {
		*includeTests = *cfg.IncludeTests
	}
	scored := rank.ScoreFiles(task, idx, rank.Options{IncludeTests: *includeTests, Priority: cfg.Priority})
	selections := budget.Select(scored, *tokenBudget)
	input := pack.RenderInput{Task: task, Budget: *tokenBudget, Repo: idx, Selections: selections}
	markdown, selections := pack.FitToBudget(input)

	if *stdoutOnly {
		fmt.Fprint(stdout, markdown)
		return nil
	}

	outputPath := *output
	if outputPath == "" {
		outputPath = filepath.Join(".ctxpack", slugify(task)+"-context.md")
	}
	outputPath = resolveOutput(idx.Root, outputPath)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Indexed %d files\n", len(idx.Files))
	fmt.Fprintf(stdout, "Detected stack: %s\n", joinOrNone(idx.Stack.Languages))
	printCacheStats(stdout, stats)
	if cfgPath != "" {
		fmt.Fprintf(stdout, "Loaded config: %s\n", cfgPath)
	}
	fmt.Fprintf(stdout, "Found %d candidate files\n", len(scored))
	fmt.Fprintf(stdout, "Selected %d files under ~%d tokens\n", countIncluded(selections), tokens.Estimate(markdown))
	fmt.Fprintf(stdout, "Wrote %s\n", outputPath)

	_ = diffMode
	return nil
}

func runDiff(args []string, stdout io.Writer, ciMode bool) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoRoot := fs.String("repo", ".", "repository root")
	base := fs.String("base", "main", "git base ref")
	tokenBudget := fs.Int("budget", 12000, "hard token budget")
	defaultOutput := ".ctxpack/diff-context.md"
	defaultSummaryJSON := ""
	defaultFailOn := ""
	if ciMode {
		defaultOutput = ".ctxpack/ci-context.md"
		defaultSummaryJSON = "reports/ctxpack-diff-summary.json"
		defaultFailOn = "all"
	}
	output := fs.String("output", defaultOutput, "context pack output path")
	summaryJSON := fs.String("summary-json", defaultSummaryJSON, "write machine-readable diff summary JSON")
	failOn := fs.String("fail-on", defaultFailOn, "comma-separated CI checks: over-budget, omitted-changed, missing-tests, forbidden-paths, all")
	forbid := fs.String("forbid", "", "comma-separated repo path globs for forbidden-paths checks")
	stdoutOnly := fs.Bool("stdout", false, "write pack to stdout")
	if err := fs.Parse(flexibleFlagArgs(args, map[string]bool{
		"stdout": true,
	})); err != nil {
		return err
	}
	task := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if task == "" {
		task = "review and explain the current diff"
	}

	idx, stats, err := index.BuildCached(*repoRoot)
	if err != nil {
		return err
	}
	if err := index.Save(idx, stats.Path); err != nil {
		return err
	}
	cfg, cfgPath, err := config.Load(idx.Root)
	if err != nil {
		return err
	}
	idx = config.Apply(idx, cfg)
	if ciMode {
		if !flagPresent(args, "base") && cfg.CI.Base != "" {
			*base = cfg.CI.Base
		}
		if !flagPresent(args, "budget") && cfg.CI.Budget > 0 {
			*tokenBudget = cfg.CI.Budget
		}
		if !flagPresent(args, "output") && cfg.CI.Output != "" {
			*output = cfg.CI.Output
		}
		if !flagPresent(args, "summary-json") && cfg.CI.SummaryJSON != "" {
			*summaryJSON = cfg.CI.SummaryJSON
		}
		if !flagPresent(args, "fail-on") && len(cfg.CI.FailOn) > 0 {
			*failOn = strings.Join(cfg.CI.FailOn, ",")
		}
		if !flagPresent(args, "forbid") && len(cfg.CI.Forbid) > 0 {
			*forbid = strings.Join(cfg.CI.Forbid, ",")
		}
	}
	if !flagPresent(args, "budget") && *tokenBudget == 12000 && cfg.Budget > 0 {
		*tokenBudget = cfg.Budget
	}
	changed, err := rank.ChangedFilesDetailed(idx.Root, *base)
	if err != nil {
		return fmt.Errorf("could not read git diff against %s: %w", *base, err)
	}
	if len(changed) == 0 {
		return fmt.Errorf("no changed files found against %s", *base)
	}
	changedPaths := rank.ChangedPaths(changed)
	scored := rank.ScoreFiles(task, idx, rank.Options{
		IncludeTests: true,
		DiffFiles:    changedPaths,
		Mode:         "diff",
		Priority:     cfg.Priority,
	})
	selections := budget.Select(scored, *tokenBudget)
	input := pack.RenderInput{Task: task, Budget: *tokenBudget, Repo: idx, Selections: selections, DiffBase: *base, ChangedFiles: changed}
	markdown, selections := pack.FitToBudget(input)
	input.Selections = selections
	estimated := tokens.Estimate(markdown)

	outputPath := ""
	if !*stdoutOnly {
		outputPath = resolveOutput(idx.Root, *output)
	}
	summaryPath := ""
	if *summaryJSON != "" {
		summaryPath = resolveOutput(idx.Root, *summaryJSON)
	}
	summary := buildDiffSummary(input, stats, cfgPath, estimated, outputPath)
	checks, err := evaluateDiffChecks(summary, *failOn, *forbid)
	if err != nil {
		return err
	}
	checkErr := applyDiffCheckResults(&summary, checks)
	if summaryPath != "" {
		if err := writeDiffSummaryJSON(summaryPath, summary); err != nil {
			return err
		}
	}

	if *stdoutOnly {
		fmt.Fprint(stdout, markdown)
		printDiffChecks(stdout, checks)
		return checkErr
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Indexed %d files\n", len(idx.Files))
	printCacheStats(stdout, stats)
	if cfgPath != "" {
		fmt.Fprintf(stdout, "Loaded config: %s\n", cfgPath)
	}
	fmt.Fprintf(stdout, "Changed files: %d\n", len(changed))
	fmt.Fprintf(stdout, "Found %d candidate files\n", len(scored))
	fmt.Fprintf(stdout, "Selected %d files under ~%d tokens\n", countIncluded(selections), estimated)
	fmt.Fprintf(stdout, "Wrote %s\n", outputPath)
	if summaryPath != "" {
		fmt.Fprintf(stdout, "Wrote summary %s\n", summaryPath)
	}
	printDiffChecks(stdout, checks)
	return checkErr
}

func runExplain(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoRoot := fs.String("repo", ".", "repository root")
	if err := fs.Parse(flexibleFlagArgs(args, map[string]bool{})); err != nil {
		return err
	}

	path := ""
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	} else {
		latest, err := latestPack(*repoRoot)
		if err != nil {
			return err
		}
		path = latest
	}
	data, err := os.ReadFile(resolveOutput(*repoRoot, path))
	if err != nil {
		return err
	}
	text := string(data)
	for _, section := range []string{"## Files to read first", "## Budget decisions", "## Do not touch unless needed"} {
		excerpt := sectionExcerpt(text, section)
		if excerpt != "" {
			fmt.Fprintln(stdout, excerpt)
			fmt.Fprintln(stdout)
		}
	}
	return nil
}

func runMCP(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoRoot := fs.String("repo", ".", "default repository root")
	if err := fs.Parse(flexibleFlagArgs(args, map[string]bool{})); err != nil {
		return err
	}
	server := mcp.Server{DefaultRepo: *repoRoot}
	return server.Run(os.Stdin, stdout, stderr)
}

func runBench(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoRoot := fs.String("repo", ".", "repository root")
	tasksPath := fs.String("tasks", "", "benchmark tasks YAML/JSON")
	outputDir := fs.String("output", "reports/bench", "benchmark output directory")
	tokenBudget := fs.Int("budget", 12000, "default hard token budget")
	includeTests := fs.Bool("include-tests", true, "include nearby tests")
	includeBaselines := fs.Bool("baselines", true, "include full-repo and lexical baseline comparisons")
	if err := fs.Parse(flexibleFlagArgs(args, map[string]bool{
		"include-tests": true,
		"baselines":     true,
	})); err != nil {
		return err
	}
	if *tasksPath == "" && fs.NArg() > 0 {
		*tasksPath = fs.Arg(0)
	}
	if *tasksPath == "" {
		return errors.New("bench requires --tasks tasks.yaml")
	}
	report, err := bench.Run(bench.Options{
		Repo:             *repoRoot,
		TasksPath:        *tasksPath,
		OutputDir:        *outputDir,
		Budget:           *tokenBudget,
		IncludeTests:     *includeTests,
		IncludeBaselines: *includeBaselines,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Index cache: %s, reused %d, parsed %d\n", cacheStatus(report.Cache.Loaded), report.Cache.ReusedFiles, report.Cache.ParsedFiles)
	passed := 0
	for _, task := range report.Tasks {
		status := "FAIL"
		if task.Passed {
			status = "PASS"
			passed++
		}
		fmt.Fprintf(stdout, "%s %s: %d files, ~%d/%d tokens\n", status, task.Name, task.SelectedFiles, task.EstimatedTokens, task.Budget)
		if len(task.MissingExpected) > 0 {
			fmt.Fprintf(stdout, "  missing expected: %s\n", strings.Join(task.MissingExpected, ", "))
		}
		if len(task.AvoidHits) > 0 {
			fmt.Fprintf(stdout, "  avoid hits: %s\n", strings.Join(task.AvoidHits, ", "))
		}
		for _, baseline := range task.Baselines {
			baselineStatus := "FAIL"
			if baseline.Passed {
				baselineStatus = "PASS"
			}
			fmt.Fprintf(stdout, "  baseline %s %s: %d files, ~%d/%d tokens\n", baseline.Name, baselineStatus, baseline.SelectedFiles, baseline.EstimatedTokens, baseline.Budget)
		}
	}
	fmt.Fprintf(stdout, "Bench: %d/%d passed\n", passed, len(report.Tasks))
	if *outputDir != "" {
		fmt.Fprintf(stdout, "Wrote %s\n", *outputDir)
	}
	if passed != len(report.Tasks) {
		return errors.New("benchmark failed")
	}
	return nil
}

func cacheStatus(loaded bool) string {
	if loaded {
		return "warm"
	}
	return "cold"
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "ctxpack - task-specific context packs for AI coding agents")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ctxpack index [--repo .]")
	fmt.Fprintln(w, "  ctxpack pack \"task\" --budget 12000 [--include-tests] [--stdout]")
	fmt.Fprintln(w, "  ctxpack diff --base main --budget 12000 [--summary-json diff-summary.json] [--fail-on omitted-changed,missing-tests] [\"optional task\"]")
	fmt.Fprintln(w, "  ctxpack ci --base main --budget 12000 [--forbid services/auth/**] [\"optional task\"]")
	fmt.Fprintln(w, "  ctxpack bench --tasks tasks.yaml [--repo .] [--output reports/bench] [--baselines]")
	fmt.Fprintln(w, "  ctxpack mcp [--repo .]")
	fmt.Fprintln(w, "  ctxpack explain [pack.md]")
	fmt.Fprintln(w, "  ctxpack version")
}

func flexibleFlagArgs(args []string, boolFlags map[string]bool) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		if strings.Contains(arg, "=") {
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if boolFlags[name] {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}

func flagPresent(args []string, name string) bool {
	long := "--" + name
	short := "-" + name
	for _, arg := range args {
		if arg == long || arg == short || strings.HasPrefix(arg, long+"=") || strings.HasPrefix(arg, short+"=") {
			return true
		}
	}
	return false
}

func resolveOutput(root string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func slugify(text string) string {
	text = strings.ToLower(text)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	text = strings.Trim(re.ReplaceAllString(text, "-"), "-")
	if text == "" {
		return "context"
	}
	if len(text) > 70 {
		text = strings.Trim(text[:70], "-")
	}
	return text
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func countIncluded(selections []budget.Selection) int {
	count := 0
	for _, sel := range selections {
		if sel.Mode != budget.ModeOmitted {
			count++
		}
	}
	return count
}

func printCacheStats(w io.Writer, stats index.CacheStats) {
	if stats.Path == "" {
		return
	}
	status := "cold"
	if stats.Loaded {
		status = "warm"
	}
	fmt.Fprintf(w, "Index cache: %s, reused %d, parsed %d", status, stats.ReusedFiles, stats.ParsedFiles)
	if stats.RemovedFiles > 0 {
		fmt.Fprintf(w, ", removed %d", stats.RemovedFiles)
	}
	fmt.Fprintln(w)
}

func latestPack(root string) (string, error) {
	dir := resolveOutput(root, ".ctxpack")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	type file struct {
		path string
		mod  int64
	}
	var files []file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, file{
			path: filepath.Join(dir, entry.Name()),
			mod:  info.ModTime().UnixNano(),
		})
	}
	if len(files) == 0 {
		return "", errors.New("no context packs found in .ctxpack")
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mod > files[j].mod
	})
	return files[0].path, nil
}

func sectionExcerpt(text string, section string) string {
	start := strings.Index(text, section)
	if start < 0 {
		return ""
	}
	rest := text[start:]
	next := strings.Index(rest[len(section):], "\n## ")
	if next < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:len(section)+next])
}

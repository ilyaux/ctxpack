package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ilyaux/ctxpack/internal/budget"
	"github.com/ilyaux/ctxpack/internal/index"
	"github.com/ilyaux/ctxpack/internal/pack"
	"github.com/ilyaux/ctxpack/internal/rank"
	"github.com/ilyaux/ctxpack/internal/version"
)

type Server struct {
	DefaultRepo string
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s Server) Run(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if s.DefaultRepo == "" {
		s.DefaultRepo = "."
	}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(stderr, "ctxpack mcp: invalid json: %v\n", err)
			continue
		}
		if len(req.ID) == 0 {
			s.handleNotification(req)
			continue
		}
		resp := s.handle(req)
		if err := writeMessage(stdout, resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s Server) handleNotification(req request) {
	_ = req
}

func (s Server) handle(req request) response {
	switch req.Method {
	case "initialize":
		return ok(req.ID, map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "ctxpack",
				"version": version.Version,
			},
		})
	case "ping":
		return ok(req.ID, map[string]any{})
	case "tools/list":
		return ok(req.ID, map[string]any{"tools": tools()})
	case "tools/call":
		result, err := s.callTool(req.Params)
		if err != nil {
			return fail(req.ID, -32000, err.Error())
		}
		return ok(req.ID, result)
	default:
		return fail(req.ID, -32601, "method not found")
	}
}

func (s Server) callTool(params json.RawMessage) (any, error) {
	var call toolCallParams
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, err
	}
	var text string
	var err error
	switch call.Name {
	case "repo_context_pack":
		text, err = s.repoContextPack(call.Arguments)
	case "search_symbol":
		text, err = s.searchSymbol(call.Arguments)
	case "explain_repo_area":
		text, err = s.explainRepoArea(call.Arguments)
	case "repo_related_tests":
		text, err = s.relatedTests(call.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool %q", call.Name)
	}
	if err != nil {
		return map[string]any{
			"isError": true,
			"content": []map[string]string{{
				"type": "text",
				"text": err.Error(),
			}},
		}, nil
	}
	return map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": text,
		}},
	}, nil
}

func (s Server) repoContextPack(raw json.RawMessage) (string, error) {
	var args struct {
		Task         string `json:"task"`
		TokenBudget  int    `json:"token_budget"`
		Repo         string `json:"repo"`
		IncludeTests *bool  `json:"include_tests"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Task) == "" {
		return "", fmt.Errorf("task is required")
	}
	if args.TokenBudget <= 0 {
		args.TokenBudget = 12000
	}
	includeTests := true
	if args.IncludeTests != nil {
		includeTests = *args.IncludeTests
	}

	idx, err := index.Build(s.repo(args.Repo))
	if err != nil {
		return "", err
	}
	scored := rank.ScoreFiles(args.Task, idx, rank.Options{IncludeTests: includeTests})
	selections := budget.Select(scored, args.TokenBudget)
	input := pack.RenderInput{Task: args.Task, Budget: args.TokenBudget, Repo: idx, Selections: selections}
	markdown, _ := pack.FitToBudget(input)
	return markdown, nil
}

func (s Server) searchSymbol(raw json.RawMessage) (string, error) {
	var args struct {
		Name  string `json:"name"`
		Repo  string `json:"repo"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if args.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}

	idx, err := index.Build(s.repo(args.Repo))
	if err != nil {
		return "", err
	}
	query := strings.ToLower(args.Name)
	var lines []string
	for _, file := range idx.Files {
		for _, sym := range file.Symbols {
			if strings.Contains(strings.ToLower(sym.Name), query) {
				line := file.Path
				if sym.Line > 0 {
					line = fmt.Sprintf("%s:%d", line, sym.Line)
				}
				lines = append(lines, fmt.Sprintf("- %s `%s` %s", line, sym.Name, sym.Signature))
			}
		}
	}
	if len(lines) == 0 {
		return "No matching symbols found.", nil
	}
	if len(lines) > args.Limit {
		lines = lines[:args.Limit]
	}
	return strings.Join(lines, "\n"), nil
}

func (s Server) explainRepoArea(raw json.RawMessage) (string, error) {
	var args struct {
		Path  string `json:"path"`
		Repo  string `json:"repo"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if args.Limit <= 0 {
		args.Limit = 40
	}

	idx, err := index.Build(s.repo(args.Repo))
	if err != nil {
		return "", err
	}
	prefix := filepath.ToSlash(strings.Trim(args.Path, `/\`))
	var lines []string
	for _, file := range idx.Files {
		if file.Path != prefix && !strings.HasPrefix(file.Path, prefix+"/") {
			continue
		}
		desc := budget.Summary(file)
		lines = append(lines, "- "+desc)
	}
	if len(lines) == 0 {
		return "No indexed files found for " + args.Path + ".", nil
	}
	sort.Strings(lines)
	if len(lines) > args.Limit {
		lines = lines[:args.Limit]
	}
	return strings.Join(lines, "\n"), nil
}

func (s Server) relatedTests(raw json.RawMessage) (string, error) {
	var args struct {
		Path  string `json:"path"`
		Repo  string `json:"repo"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}

	idx, err := index.Build(s.repo(args.Repo))
	if err != nil {
		return "", err
	}
	target := filepath.ToSlash(args.Path)
	targetDir := filepath.Dir(target)
	targetBase := strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))
	var lines []string
	for _, file := range idx.Files {
		if !file.IsTest {
			continue
		}
		fileDir := filepath.Dir(file.Path)
		fileBase := strings.ToLower(filepath.Base(file.Path))
		if fileDir == targetDir || strings.Contains(fileBase, strings.ToLower(targetBase)) || strings.Contains(strings.ToLower(file.Content), strings.ToLower(targetBase)) {
			lines = append(lines, "- "+budget.Summary(file))
		}
	}
	if len(lines) == 0 {
		return "No related tests found.", nil
	}
	sort.Strings(lines)
	if len(lines) > args.Limit {
		lines = lines[:args.Limit]
	}
	return strings.Join(lines, "\n"), nil
}

func (s Server) repo(repo string) string {
	if repo == "" {
		return s.DefaultRepo
	}
	return repo
}

func tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "repo_context_pack",
			"description": "Build a task-specific context pack under a hard token budget.",
			"inputSchema": objectSchema(map[string]any{
				"task":          stringSchema("User task to prepare context for."),
				"token_budget":  integerSchema("Hard token budget. Defaults to 12000."),
				"repo":          stringSchema("Repository root. Defaults to the server repo."),
				"include_tests": booleanSchema("Whether to boost nearby tests. Defaults to true."),
			}, []string{"task"}),
		},
		{
			"name":        "search_symbol",
			"description": "Search indexed Go/TypeScript symbols by name.",
			"inputSchema": objectSchema(map[string]any{
				"name":  stringSchema("Symbol name or substring."),
				"repo":  stringSchema("Repository root. Defaults to the server repo."),
				"limit": integerSchema("Maximum results. Defaults to 20."),
			}, []string{"name"}),
		},
		{
			"name":        "explain_repo_area",
			"description": "Summarize indexed files, imports, and symbols under a repository path.",
			"inputSchema": objectSchema(map[string]any{
				"path":  stringSchema("Path or directory to explain."),
				"repo":  stringSchema("Repository root. Defaults to the server repo."),
				"limit": integerSchema("Maximum files. Defaults to 40."),
			}, []string{"path"}),
		},
		{
			"name":        "repo_related_tests",
			"description": "Find nearby test files for a repository path.",
			"inputSchema": objectSchema(map[string]any{
				"path":  stringSchema("Source file or directory."),
				"repo":  stringSchema("Repository root. Defaults to the server repo."),
				"limit": integerSchema("Maximum tests. Defaults to 20."),
			}, []string{"path"}),
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerSchema(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func booleanSchema(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func ok(id json.RawMessage, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func fail(id json.RawMessage, code int, message string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}

func writeMessage(w io.Writer, msg response) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

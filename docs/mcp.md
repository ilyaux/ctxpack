# MCP Server

`ctxpack mcp` starts a stdio MCP server for Claude Desktop, Claude Code, Codex, and other MCP clients.

```bash
ctxpack mcp --repo .
```

The server reads JSON-RPC messages from stdin and writes responses to stdout. The `--repo` value becomes the default repository for tool calls.

## Tools

### `repo_context_pack`

Builds a task-specific markdown context pack.

Arguments:

- `task` required string
- `token_budget` optional integer, defaults to `12000`
- `repo` optional string, overrides the server default repo
- `include_tests` optional boolean, defaults to `true`

Returns: markdown context pack text.

### `search_symbol`

Searches indexed symbols by name substring.

Arguments:

- `name` required string
- `repo` optional string
- `limit` optional integer, defaults to `20`

Returns: markdown bullet list of matching symbols.

### `explain_repo_area`

Summarizes indexed files below a path.

Arguments:

- `path` required string
- `repo` optional string
- `limit` optional integer, defaults to `40`

Returns: markdown bullet list of file summaries.

### `repo_related_tests`

Finds nearby test files for a source path.

Arguments:

- `path` required string
- `repo` optional string
- `limit` optional integer, defaults to `20`

Returns: markdown bullet list of related tests.

## Claude Desktop Example

```json
{
  "mcpServers": {
    "ctxpack": {
      "command": "ctxpack",
      "args": ["mcp", "--repo", "/absolute/path/to/repo"]
    }
  }
}
```

Use an absolute repo path in MCP client configs. Relative paths are resolved from the client process working directory, which is often not the repository root.

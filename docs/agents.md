# Using ctxpack With Coding Agents

`ctxpack` is designed to run before the coding agent. Generate the task-specific pack, then feed that pack to the tool you already use.

## Claude Code

```bash
ctxpack pack "add retry logic to the billing webhook" --budget 12000
claude < .ctxpack/add-retry-logic-to-the-billing-webhook-context.md
```

For pull requests:

```bash
ctxpack diff --base origin/main --budget 12000 --output .ctxpack/pr-context.md "review this pull request"
claude < .ctxpack/pr-context.md
```

## Codex

```bash
ctxpack pack "add commission calculation endpoint and update frontend" --budget 12000
codex < .ctxpack/add-commission-calculation-endpoint-and-update-frontend-context.md
```

In a local Codex thread, use the generated markdown pack as the first message context when you want the agent to avoid scanning the whole repo.

## Gemini CLI

```bash
ctxpack pack "fix login redirect after OAuth callback" --budget 10000
gemini < .ctxpack/fix-login-redirect-after-oauth-callback-context.md
```

## Local Models

For smaller local models, lower the budget first instead of dumping the repository:

```bash
ctxpack pack "add wallet transaction CSV export" --budget 8000 --stdout > /tmp/ctxpack.md
```

Then paste or pipe `/tmp/ctxpack.md` into your local model wrapper.

## MCP Clients

Start the stdio MCP server:

```bash
ctxpack mcp --repo /absolute/path/to/repo
```

MCP clients can call:

- `repo_context_pack`
- `search_symbol`
- `explain_repo_area`
- `repo_related_tests`

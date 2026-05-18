# Troubleshooting

## Pack Includes Too Many Files

Lower the budget first:

```bash
ctxpack pack "task" --budget 8000
```

Then add `.ctxpack.yml` priority and ignore rules:

```yaml
ignore:
  - generated/**
  - target/**

priority:
  - services/billing/**
  - packages/api-client/**
```

## PR Check Fails On Missing Tests

`missing-tests` means changed non-test files were selected but no nearby test file was selected.

Options:

- add or modify a nearby test
- lower unrelated changed files
- disable the check for that repo with `ci.fail_on`

## Changed Files Are Omitted

`omitted-changed` means the diff has files that did not fit in the token budget. Raise `ci.budget`, split the PR, or add targeted priority rules.

## MCP Client Cannot Find The Repo

Use an absolute path:

```json
{
  "command": "ctxpack",
  "args": ["mcp", "--repo", "/absolute/path/to/repo"]
}
```

Relative paths are resolved from the MCP client process, not necessarily from your shell.

## Cache Looks Stale

The default cache is SQLite at:

```text
.ctxpack/index.sqlite
```

Delete it to force a cold reindex:

```bash
rm -rf .ctxpack
ctxpack index
```

On Windows PowerShell:

```powershell
Remove-Item -Recurse -Force .ctxpack
ctxpack index
```

## Bench Fails But The Pack Looks Reasonable

Bench tasks are strict. A task fails when an `expect` glob is missing, an `avoid` glob appears, the pack exceeds budget, or no files are selected. Adjust the task file only when the expected repo boundary is wrong.

# ctxpack Examples

This page shows common `ctxpack` workflows for coding agents, pull requests, and benchmark runs.

Copyable example files:

- [`examples/ctxpack.yml`](../examples/ctxpack.yml)
- [`examples/bench-tasks.yaml`](../examples/bench-tasks.yaml)
- [`examples/github-actions.yml`](../examples/github-actions.yml)
- [`evals/go-ts-monorepo/tasks.yaml`](../evals/go-ts-monorepo/tasks.yaml)
- [`evals/java-maven-webapp/tasks.yaml`](../evals/java-maven-webapp/tasks.yaml)

Installation options are documented in [`docs/install.md`](install.md).

## Task Pack

Generate a compact context pack for an implementation task:

```bash
ctxpack pack "add commission calculation endpoint and update frontend" --budget 12000
```

Send it to an agent:

```bash
claude < .ctxpack/add-commission-calculation-endpoint-and-update-frontend-context.md
```

Or keep everything on stdout:

```bash
ctxpack pack "fix login redirect after OAuth callback" --budget 10000 --stdout
```

## Diff Pack

Generate a review/follow-up context pack around the current diff:

```bash
ctxpack diff --base origin/main \
  --budget 12000 \
  --output .ctxpack/diff-context.md \
  --summary-json reports/diff-summary.json \
  "review this pull request"
```

The markdown pack is for humans and coding agents. The JSON summary is for CI, dashboards, or benchmark comparison.

## CI Check

Use `ctxpack ci` in pull requests:

```bash
ctxpack ci --base origin/main \
  --budget 12000 \
  --forbid "services/auth/**,apps/admin/**" \
  "review this pull request"
```

`ctxpack ci` writes:

- `.ctxpack/ci-context.md`
- `reports/ctxpack-diff-summary.json`

It exits with code `1` when enabled checks fail.

## Config

Put repository defaults in `.ctxpack.yml`:

```yaml
budget: 12000
include_tests: true

ignore:
  - target/**
  - dist/**
  - generated/**

priority:
  - services/billing/**
  - packages/api-client/**

languages:
  go: true
  typescript: true
  java: true
  sql: true

ci:
  base: origin/main
  budget: 12000
  output: .ctxpack/ci-context.md
  summary_json: reports/ctxpack-diff-summary.json
  fail_on:
    - over-budget
    - omitted-changed
  forbid:
    - services/auth/**
    - apps/admin/**
```

Command-line flags override config values.

## Bench

Run repeatable selection checks:

```bash
ctxpack bench \
  --repo testdata/fixtures/go-ts-monorepo \
  --tasks evals/go-ts-monorepo/tasks.yaml \
  --output reports/evals/go-ts-monorepo \
  --budget 9000
```

Bench output includes `ctxpack` results plus baseline comparison:

- `full-repo`: all indexed files, similar to a repo dump
- `repomix-style`: all indexed files concatenated into markdown with file headers
- `lexical-budget`: naive keyword selection under the same token budget

Disable baselines if you only want the `ctxpack` result:

```bash
ctxpack bench --tasks bench/tasks.yaml --baselines=false
```

## MCP

Run a stdio MCP server for Claude Desktop, Claude Code, or other MCP clients:

```bash
ctxpack mcp --repo .
```

Tools:

- `repo_context_pack(task, token_budget)`
- `search_symbol(name)`
- `explain_repo_area(path)`
- `repo_related_tests(path)`

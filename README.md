# ctxpack

[![CI](https://github.com/ilyaux/ctxpack/actions/workflows/ci.yml/badge.svg)](https://github.com/ilyaux/ctxpack/actions/workflows/ci.yml)
[![Release](https://github.com/ilyaux/ctxpack/actions/workflows/release.yml/badge.svg)](https://github.com/ilyaux/ctxpack/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ilyaux/ctxpack.svg)](https://pkg.go.dev/github.com/ilyaux/ctxpack)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Give your AI coding agent the right 12k tokens, not your whole monorepo.

`ctxpack` is a context compiler for AI coding agents. It builds task-specific context packs for Claude Code, Codex, Gemini CLI, Cursor, Aider, and local LLMs.

```bash
ctxpack pack "add retry logic to the billing webhook" --budget 12000
```

It selects:

- relevant files
- important symbols
- nearby tests
- dependency and route hints
- git diff context
- SQLite index cache
- strict token-budget decisions

## Why

Coding agents usually fail in large repositories because the context is bad, not because the model is useless. A full repo dump burns tokens on unrelated files, while shallow text search misses architecture.

`ctxpack` sits before the agent:

```text
task -> repo index -> ranked slices -> token budget -> context pack -> coding agent
```

The core promise:

```text
Give Claude Code a 12k-token surgical context instead of a 100k-token repo dump.
```

## Quick Start

Install a release binary from the GitHub release page, then verify it:

```bash
ctxpack version
```

Full install options are documented in [docs/install.md](docs/install.md).

Or install from source:

```bash
go install github.com/ilyaux/ctxpack/cmd/ctxpack@latest
```

Or run inside this repository:

```bash
go run ./cmd/ctxpack pack "fix login redirect after OAuth callback" --budget 10000
```

## Basic Usage

Generate a task-specific context pack:

```bash
ctxpack pack "add commission calculation endpoint and update frontend" --budget 12000
```

Send it to an agent:

```bash
claude < .ctxpack/add-commission-calculation-endpoint-and-update-frontend-context.md
```

Generate a pull-request context pack:

```bash
ctxpack diff --base origin/main \
  --output .ctxpack/diff-context.md \
  --summary-json reports/diff-summary.json \
  "review this pull request"
```

Run a CI check:

```bash
ctxpack ci --base origin/main \
  --budget 12000 \
  --forbid "services/auth/**,apps/admin/**" \
  "review this pull request"
```

Run a benchmark with baselines:

```bash
ctxpack bench \
  --repo testdata/fixtures/go-ts-monorepo \
  --tasks evals/go-ts-monorepo/tasks.yaml \
  --output reports/evals/go-ts-monorepo \
  --budget 9000
```

## What A Pack Contains

A generated markdown pack includes:

- task interpretation
- relevant architecture
- files to read first
- selection rationale
- important symbols
- suggested implementation plan
- budget decisions
- selected file content, slices, signatures, or summaries
- unrelated areas to avoid

When the context is too large, `ctxpack` degrades detail in this order:

```text
full file -> relevant slices -> signatures only -> summary only -> omitted
```

## Supported Repositories

Strong support:

- Go
- Java / Maven
- TypeScript / JavaScript
- React / TSX / JSX
- HTML / XHTML / JSF-style views
- SQL

Practical structured support:

- XML / JRXML
- JSON
- YAML
- `.properties`
- Velocity templates
- Markdown
- plain text fallback

The indexer extracts Go packages/imports/symbols, TypeScript package imports/default exports/components/hooks/route handlers, Java annotations/controllers/resources/wildcard imports, Maven artifacts/modules/dependencies, and nearby tests.

## Commands

```text
ctxpack index
ctxpack pack "task" --budget 12000
ctxpack diff --base main --budget 12000
ctxpack ci --base main --budget 12000
ctxpack bench --tasks tasks.yaml --output reports/bench
ctxpack mcp --repo .
ctxpack explain
ctxpack version
```

## CI

`ctxpack ci` writes:

```text
.ctxpack/ci-context.md
reports/ctxpack-diff-summary.json
```

It exits with code `1` when configured checks fail. Artifacts are written before failure so CI can upload them.

Checks:

- `over-budget`
- `omitted-changed`
- `missing-tests`
- `forbidden-paths`
- `all`

This repository includes:

- project CI: [.github/workflows/ci.yml](.github/workflows/ci.yml)
- release workflow: [.github/workflows/release.yml](.github/workflows/release.yml)

## Configuration

`ctxpack` reads `.ctxpack.yml` or `.ctxpack.yaml` from the repository root.

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

## Documentation

- [Install](docs/install.md)
- [Examples](docs/examples.md)
- [Using with Claude/Codex/Gemini](docs/agents.md)
- [Demo](docs/demo.md)
- [Evals](docs/evals.md)
- [Benchmark stories](docs/benchmark-stories.md)
- [MCP server](docs/mcp.md)
- [Schemas](docs/schemas.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Release checklist](docs/release-checklist.md)
- [v1.0 release notes](docs/release-notes-v1.0.md)
- [Example `.ctxpack.yml`](examples/ctxpack.yml)
- [Example bench tasks](examples/bench-tasks.yaml)
- [Example GitHub Actions workflow](examples/github-actions.yml)
- [Config schema](docs/schemas/ctxpack-config.schema.json)
- [Bench tasks schema](docs/schemas/bench-tasks.schema.json)
- [Diff summary schema](docs/schemas/diff-summary.schema.json)
- [Bench summary schema](docs/schemas/bench-summary.schema.json)

## Benchmarking

`ctxpack bench` writes repeatable selection reports and compares `ctxpack` against baselines:

- `full-repo`: all indexed files, similar to a repo dump
- `repomix-style`: all indexed files concatenated into markdown with file headers
- `lexical-budget`: naive keyword selection under the same budget

The checked-in synthetic fixture lives at:

```text
testdata/fixtures/go-ts-monorepo
```

Run it:

```bash
ctxpack bench \
  --repo testdata/fixtures/go-ts-monorepo \
  --tasks evals/go-ts-monorepo/tasks.yaml \
  --output reports/evals/go-ts-monorepo \
  --budget 9000
```

The repository also includes a Java/Maven + JSF fixture:

```bash
ctxpack bench \
  --repo testdata/fixtures/java-maven-webapp \
  --tasks evals/java-maven-webapp/tasks.yaml \
  --output reports/evals/java-maven-webapp \
  --budget 9000
```

## MCP

Run a stdio MCP server:

```bash
ctxpack mcp --repo .
```

Tools:

- `repo_context_pack(task, token_budget)`
- `search_symbol(name)`
- `explain_repo_area(path)`
- `repo_related_tests(path)`

## Release

Maintainers publish releases by pushing a semver tag:

```bash
bash scripts/release-smoke.sh
```

```bash
git tag v1.0.0
git push origin v1.0.0
```

The release workflow builds static binaries for Linux, macOS, and Windows, injects version metadata with Go linker flags, and publishes checksums.

## Status

Current scope is stable public CLI + MCP for task-specific context packing in large Go/Java/TypeScript monorepos. `ctxpack` is not another autonomous coding agent; it is the context layer before the agent.

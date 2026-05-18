# Changelog

## v1.0.0 - 2026-05-18

First stable public CLI/MCP release target.

- CLI commands: `index`, `pack`, `diff`, `ci`, `bench`, `mcp`, `explain`, and `version`.
- Task-specific context packs with strict token budgets and explicit budget decisions.
- Go, Java/Maven, TypeScript/JavaScript/React, SQL, XML, YAML, JSON, properties, Velocity, Markdown, and text fallback indexing.
- Git diff context packing for pull requests and code review.
- CI checks for over-budget packs, omitted changed files, missing nearby tests, and forbidden paths.
- Config-driven `.ctxpack.yml` defaults for local and CI usage.
- Stdio MCP server with context-pack, symbol search, path explanation, and related-test tools.
- Repeatable benchmark/eval reports with `full-repo` and `lexical-budget` baselines.
- `repomix-style` baseline for repo-dump markdown comparison.
- SQLite index cache at `.ctxpack/index.sqlite`.
- GitHub Actions CI and tagged release binaries for Linux, macOS, and Windows.
- Public install, MCP, schema, eval, and release checklist documentation.

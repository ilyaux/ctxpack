# ctxpack v1.0.0 Release Notes

`ctxpack` v1.0.0 is the first stable public CLI/MCP release target.

Highlights:

- task-specific context packs with strict token budgets
- CLI commands for `pack`, `diff`, `ci`, `bench`, `mcp`, and `version`
- config-driven CI checks in `.ctxpack.yml`
- SQLite index cache for large repositories
- Go, Java/Maven, TypeScript/React, JSF/XHTML, SQL, JSON, YAML, XML, properties, Velocity, Markdown, and text fallback indexing
- repeatable eval suites with `full-repo`, `repomix-style`, and `lexical-budget` baselines
- release binaries for Linux, macOS, and Windows
- public docs for install, agent usage, MCP, schemas, evals, and troubleshooting

Known limits:

- TypeScript/JavaScript parsing is practical regex-based indexing, not a full tree-sitter AST.
- The eval suite measures context selection, not final patch correctness.
- GitHub tags and releases must be created from a real git checkout.

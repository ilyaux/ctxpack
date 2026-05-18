# Schemas

This page documents the public JSON/YAML artifacts that `ctxpack` emits or reads.

Machine-readable schemas live in:

- [`docs/schemas/ctxpack-config.schema.json`](schemas/ctxpack-config.schema.json)
- [`docs/schemas/bench-tasks.schema.json`](schemas/bench-tasks.schema.json)
- [`docs/schemas/diff-summary.schema.json`](schemas/diff-summary.schema.json)
- [`docs/schemas/bench-summary.schema.json`](schemas/bench-summary.schema.json)

## `.ctxpack.yml`

`ctxpack` reads `.ctxpack.yml` or `.ctxpack.yaml` from the repository root.

Top-level fields:

- `budget`: default token budget for pack/diff/bench when no command flag overrides it.
- `include_tests`: default test-file selection behavior.
- `ignore`: repo-relative path globs removed from the index.
- `priority`: repo-relative path globs boosted during ranking.
- `languages`: language allowlist.
- `ci`: defaults for `ctxpack ci`.

`ci` fields:

- `base`: default git base ref.
- `budget`: CI token budget.
- `output`: markdown context pack path.
- `summary_json`: JSON summary path.
- `fail_on`: CI checks to enforce.
- `forbid`: forbidden path globs for `forbidden-paths`.

Supported `fail_on` values:

- `over-budget`
- `omitted-changed`
- `missing-tests`
- `forbidden-paths`
- `all`

## Diff Summary JSON

Written by:

```bash
ctxpack diff --summary-json reports/diff-summary.json
ctxpack ci
```

Important fields:

- `repository`: absolute repository root used for the run.
- `task`: task string used for ranking.
- `diff_base`: git base ref.
- `budget`: requested token budget.
- `estimated_tokens`: estimated tokens in the markdown pack.
- `changed_files`: git changed files with status and `+/-` line stats.
- `selected_changed_files`: changed files included in selected context.
- `omitted_changed_files`: changed files omitted under budget.
- `selection`: selected files with mode, score, reasons, and score components.
- `review_checklist`: generated PR review checklist.
- `passed`: final CI/check status.
- `checks`: CI check results.
- `markdown_output`: markdown pack path.

Selection modes:

- `full file`
- `relevant slices`
- `signatures only`
- `summary only`
- `omitted`

## Bench Tasks

Read by:

```bash
ctxpack bench --tasks evals/go-ts-monorepo/tasks.yaml
```

Supported shape:

```yaml
tasks:
  - name: commission-endpoint
    task: add commission calculation endpoint and update frontend
    budget: 9000
    expect:
      - services/billing/**
      - packages/api-client/**
    avoid:
      - services/auth/**
```

Fields:

- `name`: stable task id used in reports and output filenames.
- `task`: natural-language task used for ranking.
- `budget`: optional task-specific token budget.
- `expect`: path globs that must appear in selected files.
- `avoid`: path globs that must not appear in selected files.

## Bench Summary JSON

Written by:

```bash
ctxpack bench --output reports/bench
```

Top-level fields:

- `repo`
- `generated_at`
- `indexed_files`
- `stack`
- `config_path`
- `cache`
- `tasks`

Each task includes:

- `name`
- `task`
- `budget`
- `estimated_tokens`
- `selected_files`
- `top_files`
- `selection`
- `baselines`
- `expected_hits`
- `missing_expected`
- `avoid_hits`
- `output`
- `passed`

Baseline names:

- `full-repo`
- `repomix-style`
- `lexical-budget`

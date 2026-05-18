# Evals

`ctxpack` ships with a small repeatable eval suite. It is intentionally synthetic, checked in, and safe to run in CI.

The suite answers one question:

```text
For a known task, did ctxpack select the expected repo areas under budget while avoiding unrelated paths?
```

## Run The Public Evals

```bash
ctxpack bench \
  --repo testdata/fixtures/go-ts-monorepo \
  --tasks evals/go-ts-monorepo/tasks.yaml \
  --output reports/evals/go-ts-monorepo \
  --budget 9000

ctxpack bench \
  --repo testdata/fixtures/java-maven-webapp \
  --tasks evals/java-maven-webapp/tasks.yaml \
  --output reports/evals/java-maven-webapp \
  --budget 9000
```

The command writes:

```text
reports/evals/go-ts-monorepo/bench-summary.json
reports/evals/go-ts-monorepo/packs/*.md
```

The summary includes:

- selected files and selection modes
- expected path hits
- missing expected paths
- avoided path hits
- estimated token count
- `full-repo` baseline
- `repomix-style` baseline
- `lexical-budget` baseline

## Add A Repo-Specific Eval

Create a task file:

```yaml
tasks:
  - name: webhook-retry
    task: add retry logic to the billing webhook
    budget: 12000
    expect:
      - services/billing/**
      - internal/events/**
      - services/billing/**/*test*
    avoid:
      - services/auth/**
```

Run it:

```bash
ctxpack bench \
  --repo . \
  --tasks evals/my-repo/tasks.yaml \
  --output reports/evals/my-repo
```

Keep reports out of git. The repository `.gitignore` already ignores `reports/`.

## Pass Criteria

An eval task passes when:

- the generated pack is under the requested budget
- every `expect` glob matches at least one selected file
- no selected file matches an `avoid` glob
- at least one file is selected

This does not claim the coding agent will produce a correct patch. It measures the context-selection layer directly and repeatably.

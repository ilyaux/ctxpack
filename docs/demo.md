# Demo

This demo uses the checked-in synthetic monorepo fixture:

```text
testdata/fixtures/go-ts-monorepo
```

The fixture contains:

- Go billing service
- TypeScript/React billing page
- API client package
- unrelated auth service
- benchmark task file

## Generate A Context Pack

```bash
ctxpack pack \
  --repo testdata/fixtures/go-ts-monorepo \
  --budget 9000 \
  "add commission calculation endpoint and update frontend"
```

Expected shape:

```text
Indexed 12 files
Detected stack: Go, JavaScript, React, TypeScript
Found ... candidate files
Selected ... files under ~... tokens
Wrote ...commission-calculation-endpoint-and-update-frontend-context.md
```

The generated pack should prioritize:

- `services/billing/api/routes.go`
- `services/billing/domain/fees.go`
- `services/billing/domain/fees_test.go`
- `packages/api-client/src/billing.ts`
- `apps/web/src/pages/billing/BillingPage.tsx`

It should avoid the unrelated auth service unless the task mentions auth.

## Run Benchmark With Baselines

```bash
ctxpack bench \
  --repo testdata/fixtures/go-ts-monorepo \
  --tasks evals/go-ts-monorepo/tasks.yaml \
  --output reports/evals/go-ts-monorepo \
  --budget 9000
```

Expected shape:

```text
PASS commission-endpoint: ... files, ~.../9000 tokens
PASS billing-client-ui: ... files, ~.../9000 tokens
  baseline full-repo ...
  baseline repomix-style ...
  baseline lexical-budget ...
Bench: 2/2 passed
```

The detailed JSON report is written to:

```text
reports/evals/go-ts-monorepo/bench-summary.json
```

## PR Check Demo

Use a temporary branch in the fixture repository, make a change, then run:

```bash
ctxpack ci \
  --repo testdata/fixtures/go-ts-monorepo \
  --base HEAD \
  --budget 9000 \
  "review fixture change"
```

Outputs:

```text
.ctxpack/ci-context.md
reports/ctxpack-diff-summary.json
```

If checks fail, the command still writes both files before exiting with code `1`.

## Demo Recording Script

A simple terminal recording can show:

```text
1. tree testdata/fixtures/go-ts-monorepo
2. ctxpack pack ... --budget 9000
3. open the generated markdown pack
4. ctxpack bench ... --output reports/evals/go-ts-monorepo
5. show bench-summary.json baselines
```

The point of the demo is to show task-specific selection, not chat. The strongest visual is the contrast between a full repo tree and the small selected file list.

## GIF Storyboard

Use three terminal panes or cuts:

```text
1. Left: show the fixture repo tree and count files.
2. Middle: run ctxpack bench with baselines.
3. Right: open the generated pack and highlight "Files to read first" plus "Budget decisions".
```

Suggested caption:

```text
Full repo dump selects unrelated auth files.
ctxpack selects the billing endpoint, domain logic, API client, UI page, and nearby tests under budget.
```

For the Java/Maven story, use:

```bash
ctxpack bench \
  --repo testdata/fixtures/java-maven-webapp \
  --tasks evals/java-maven-webapp/tasks.yaml \
  --output reports/evals/java-maven-webapp \
  --budget 9000
```

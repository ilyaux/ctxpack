# Benchmark Stories

These stories are synthetic and repeatable. They are meant to show the shape of the evaluation, not claim universal performance.

## Go + TypeScript Monorepo

Fixture:

```text
testdata/fixtures/go-ts-monorepo
```

Task:

```text
add commission calculation endpoint and update frontend
```

Expected context:

- Go billing routes and domain logic
- Go fee tests
- shared TypeScript API client
- React billing page
- no auth service files

Run:

```bash
ctxpack bench \
  --repo testdata/fixtures/go-ts-monorepo \
  --tasks evals/go-ts-monorepo/tasks.yaml \
  --output reports/evals/go-ts-monorepo \
  --budget 9000
```

Baseline comparison:

- `full-repo`: includes every indexed file and fails the avoid-path check.
- `repomix-style`: concatenates every indexed file into markdown and fails the avoid-path check.
- `lexical-budget`: can pass in this small fixture, but has no architecture-level budget decisions.
- `ctxpack`: selects the billing backend, API client, frontend page, and nearby tests.

## Java/Maven + JSF Webapp

Fixture:

```text
testdata/fixtures/java-maven-webapp
```

Task:

```text
add commission preview endpoint to the Java billing service and update the JSF billing page
```

Expected context:

- Java controller/service/request/response
- nearby Java test
- JSF page/backing bean
- no auth service files

Run:

```bash
ctxpack bench \
  --repo testdata/fixtures/java-maven-webapp \
  --tasks evals/java-maven-webapp/tasks.yaml \
  --output reports/evals/java-maven-webapp \
  --budget 9000
```

This fixture is intentionally small but contains the failure mode that appears in large enterprise repos: Maven module boundaries and unrelated auth modules look superficially relevant unless the context pack is task-specific.

# Contributing

## Development

Run the local gate before sending changes:

```bash
bash scripts/release-smoke.sh
```

On Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/release-smoke.ps1
```

Generated files under `.ctxpack/` and `reports/` are ignored and should not be committed.

## Fixtures

Keep checked-in fixtures synthetic. Do not copy proprietary code into `testdata/fixtures`.

Good fixture tasks include:

- expected repo areas via `expect`
- unrelated areas via `avoid`
- a tight enough budget that full-repo dumps fail

## Optional Dogfood

Large local repository dogfood is opt-in:

```bash
CTXPACK_DOGFOOD_REPO=/absolute/path/to/large/repo go test ./internal/dogfood
```

The dogfood test writes reports under `reports/dogfood`, which is ignored.

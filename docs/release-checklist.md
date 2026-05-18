# Release Checklist

This checklist is for maintainers publishing a public `ctxpack` release.

## Before Tagging

Confirm the repository is a real git checkout and clean:

```bash
git status --short
git fetch --tags
git tag --list "v*"
```

Run the local gate:

```bash
bash scripts/release-smoke.sh
```

On Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/release-smoke.ps1
```

Check that:

- README examples still match CLI flags.
- `CHANGELOG.md` has the target version.
- `docs/schemas/*.schema.json` parse as JSON.
- `docs/install.md` references the current release asset names.
- `reports/` and `.ctxpack/` are not committed.
- `go run ./cmd/ctxpack index` writes `.ctxpack/index.sqlite`.
- `scripts/release-smoke.*` passes on at least one local machine before tagging.

## Tag

```bash
git tag v1.0.0
git push origin v1.0.0
```

Do not reuse a public semver tag for different source content. If a release workflow needs to be retried, rerun the workflow for the same tag; the upload step uses `--clobber` for assets.

The release workflow builds:

- `ctxpack-linux-amd64`
- `ctxpack-linux-arm64`
- `ctxpack-darwin-amd64`
- `ctxpack-darwin-arm64`
- `ctxpack-windows-amd64.exe`

It publishes `.tar.gz` or `.zip` archives plus `checksums.txt`.

## After Publishing

Verify:

- `ctxpack version` prints the tag, commit, and build date from the released binary.
- Archives unpack to a single executable.
- GitHub release assets include `checksums.txt`.
- The public eval command passes from a clean clone.

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUTPUT_ROOT="${OUTPUT_ROOT:-reports/release-smoke}"
DIST_DIR="$ROOT/dist/release-smoke"
SKIP_CROSS_COMPILE="${SKIP_CROSS_COMPILE:-0}"

mkdir -p "$OUTPUT_ROOT" "$DIST_DIR"

OLD_CGO_ENABLED="${CGO_ENABLED-}"
OLD_GOOS="${GOOS-}"
OLD_GOARCH="${GOARCH-}"
TMP_DIR=""

step() {
  printf '\n==> %s\n' "$1"
}

cleanup() {
  if [[ -n "$TMP_DIR" && -d "$TMP_DIR" ]]; then
    rm -rf "$TMP_DIR"
  fi
  rm -rf "$ROOT/testdata/fixtures/go-ts-monorepo/.ctxpack"
  rm -rf "$ROOT/testdata/fixtures/java-maven-webapp/.ctxpack"
  if [[ -n "$OLD_CGO_ENABLED" ]]; then export CGO_ENABLED="$OLD_CGO_ENABLED"; else unset CGO_ENABLED || true; fi
  if [[ -n "$OLD_GOOS" ]]; then export GOOS="$OLD_GOOS"; else unset GOOS || true; fi
  if [[ -n "$OLD_GOARCH" ]]; then export GOARCH="$OLD_GOARCH"; else unset GOARCH || true; fi
}
trap cleanup EXIT

step "go test"
go test ./...

step "schema JSON"
TMP_DIR="$(mktemp -d)"
cat > "$TMP_DIR/check_json.go" <<'GO'
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	for _, path := range os.Args[1:] {
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		if !json.Valid(data) {
			panic(fmt.Sprintf("invalid JSON: %s", path))
		}
	}
	fmt.Println("schemas ok")
}
GO
go run "$TMP_DIR/check_json.go" docs/schemas/*.schema.json

step "build local binary"
go build -trimpath -o "$DIST_DIR/ctxpack-smoke" ./cmd/ctxpack
"$DIST_DIR/ctxpack-smoke" version

step "eval go-ts-monorepo"
go run ./cmd/ctxpack bench \
  --repo testdata/fixtures/go-ts-monorepo \
  --tasks evals/go-ts-monorepo/tasks.yaml \
  --output "$OUTPUT_ROOT/evals/go-ts-monorepo" \
  --budget 9000

step "eval java-maven-webapp"
go run ./cmd/ctxpack bench \
  --repo testdata/fixtures/java-maven-webapp \
  --tasks evals/java-maven-webapp/tasks.yaml \
  --output "$OUTPUT_ROOT/evals/java-maven-webapp" \
  --budget 9000

step "SQLite cache smoke"
go run ./cmd/ctxpack index --repo testdata/fixtures/go-ts-monorepo
test -f "$ROOT/testdata/fixtures/go-ts-monorepo/.ctxpack/index.sqlite"

if [[ "$SKIP_CROSS_COMPILE" != "1" ]]; then
  step "cross-compile release targets"
  targets=(
    "linux amd64 "
    "linux arm64 "
    "darwin amd64 "
    "darwin arm64 "
    "windows amd64 .exe"
  )
  for target in "${targets[@]}"; do
    read -r goos goarch ext <<<"$target"
    export CGO_ENABLED=0
    export GOOS="$goos"
    export GOARCH="$goarch"
    echo "building $goos/$goarch"
    go build -trimpath -o "$DIST_DIR/ctxpack-$goos-$goarch$ext" ./cmd/ctxpack
  done
fi

step "done"
echo "release smoke passed"

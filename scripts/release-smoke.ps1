[CmdletBinding()]
param(
    [switch]$SkipCrossCompile,
    [string]$OutputRoot = "reports/release-smoke"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent $ScriptDir
Set-Location $Root

$OutputRootPath = if ([System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot
} else {
    Join-Path $Root $OutputRoot
}
$DistDir = Join-Path (Join-Path $Root "dist") "release-smoke"

New-Item -ItemType Directory -Force -Path $OutputRootPath | Out-Null
New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

$OldEnv = @{
    CGO_ENABLED = $env:CGO_ENABLED
    GOOS = $env:GOOS
    GOARCH = $env:GOARCH
}

function Step {
    param([string]$Name)
    Write-Host ""
    Write-Host "==> $Name"
}

function Restore-GoEnv {
    foreach ($Key in $OldEnv.Keys) {
        if ($null -eq $OldEnv[$Key]) {
            Remove-Item "Env:$Key" -ErrorAction SilentlyContinue
        } else {
            Set-Item "Env:$Key" $OldEnv[$Key]
        }
    }
}

try {
    Step "go test"
    go test ./...

    Step "schema JSON"
    Get-ChildItem -Path (Join-Path $Root "docs/schemas") -Filter "*.schema.json" | ForEach-Object {
        Get-Content -Raw $_.FullName | ConvertFrom-Json | Out-Null
    }
    Write-Host "schemas ok"

    Step "build local binary"
    $LocalBinary = Join-Path $DistDir "ctxpack-smoke.exe"
    go build -trimpath -o $LocalBinary ./cmd/ctxpack
    & $LocalBinary version

    Step "eval go-ts-monorepo"
    go run ./cmd/ctxpack bench `
        --repo testdata/fixtures/go-ts-monorepo `
        --tasks evals/go-ts-monorepo/tasks.yaml `
        --output (Join-Path $OutputRootPath "evals/go-ts-monorepo") `
        --budget 9000

    Step "eval java-maven-webapp"
    go run ./cmd/ctxpack bench `
        --repo testdata/fixtures/java-maven-webapp `
        --tasks evals/java-maven-webapp/tasks.yaml `
        --output (Join-Path $OutputRootPath "evals/java-maven-webapp") `
        --budget 9000

    Step "SQLite cache smoke"
    go run ./cmd/ctxpack index --repo testdata/fixtures/go-ts-monorepo
    $FixtureCache = Join-Path $Root "testdata/fixtures/go-ts-monorepo/.ctxpack/index.sqlite"
    if (-not (Test-Path -LiteralPath $FixtureCache)) {
        throw "expected SQLite cache at $FixtureCache"
    }

    if (-not $SkipCrossCompile) {
        Step "cross-compile release targets"
        $Targets = @(
            @{ GOOS = "linux"; GOARCH = "amd64"; Ext = "" },
            @{ GOOS = "linux"; GOARCH = "arm64"; Ext = "" },
            @{ GOOS = "darwin"; GOARCH = "amd64"; Ext = "" },
            @{ GOOS = "darwin"; GOARCH = "arm64"; Ext = "" },
            @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe" }
        )
        foreach ($Target in $Targets) {
            $env:CGO_ENABLED = "0"
            $env:GOOS = $Target.GOOS
            $env:GOARCH = $Target.GOARCH
            $Out = Join-Path $DistDir ("ctxpack-{0}-{1}{2}" -f $Target.GOOS, $Target.GOARCH, $Target.Ext)
            Write-Host "building $($Target.GOOS)/$($Target.GOARCH)"
            go build -trimpath -o $Out ./cmd/ctxpack
        }
    }

    Step "done"
    Write-Host "release smoke passed"
} finally {
    Restore-GoEnv
    Remove-Item -LiteralPath (Join-Path $Root "testdata/fixtures/go-ts-monorepo/.ctxpack") -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $Root "testdata/fixtures/java-maven-webapp/.ctxpack") -Recurse -Force -ErrorAction SilentlyContinue
}

# Install

`ctxpack` is distributed as release binaries and can also be installed from source with Go.

## Release Binaries

Download the archive for your platform from the GitHub release page:

```text
ctxpack-v1.0.0-linux-amd64.tar.gz
ctxpack-v1.0.0-linux-arm64.tar.gz
ctxpack-v1.0.0-darwin-amd64.tar.gz
ctxpack-v1.0.0-darwin-arm64.tar.gz
ctxpack-v1.0.0-windows-amd64.zip
```

Each release includes `checksums.txt`.

Linux/macOS:

```bash
tar -xzf ctxpack-v1.0.0-linux-amd64.tar.gz
chmod +x ctxpack-linux-amd64
sudo mv ctxpack-linux-amd64 /usr/local/bin/ctxpack
ctxpack version
```

Windows PowerShell:

```powershell
Expand-Archive .\ctxpack-v1.0.0-windows-amd64.zip .
.\ctxpack-windows-amd64.exe version
```

## Go Install

```bash
go install github.com/ilyaux/ctxpack/cmd/ctxpack@latest
ctxpack version
```

Use this path for development builds or when you already have a matching Go toolchain installed.

## Cache Location

`ctxpack` writes its local index cache to:

```text
.ctxpack/index.sqlite
```

The cache is local to the repository and safe to delete.

## Verify A Release

Linux/macOS:

```bash
sha256sum -c checksums.txt
```

Windows PowerShell:

```powershell
Get-FileHash .\ctxpack-v1.0.0-windows-amd64.zip -Algorithm SHA256
Get-Content .\checksums.txt
```

The version command should print the tag, commit, and UTC build timestamp for release binaries.

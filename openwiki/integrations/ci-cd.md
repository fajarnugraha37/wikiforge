---
type: Playbook
title: CI/CD and Release Operations
description: GitHub Actions workflows for multi-platform CI testing and GoReleaser-powered binary releases
tags: [ci, cd, github-actions, goreleaser, release]
resource: /.github/workflows
---

# CI/CD and Release Operations

WikiForge has two GitHub Actions workflows, a GoReleaser configuration, a Dockerfile, and documented release procedures.

## CI workflow

[`/.github/workflows/ci.yml`](/.github/workflows/ci.yml) runs on every push and pull request:

- **Matrix**: Ubuntu, Windows, macOS (Go 1.23.x)
- **Steps**:
  1. `actions/checkout@v4`
  2. `actions/setup-go@v5` with Go 1.23.x
  3. `go test -count=1 ./...` (all OS)
  4. `go vet ./...` (all OS)
  5. `go test -race -count=1 ./...` (Linux only, for race detection)
  6. `go build ./cmd/wikiforge`

This is a verification workflow only. It does not publish binaries.

## Release workflow

[`/.github/workflows/release.yml`](/.github/workflows/release.yml) runs when a `v*` tag is pushed, or can be manually dispatched with an existing tag:

- **Permissions**: `contents: write` (for GitHub Release creation)
- **Steps**:
  1. Check out tag with full history
  2. Verify tag matches `vX.Y.Z` semantic version format
  3. `go test ./...` and `go vet ./...`
  4. Run GoReleaser (`~> v2`) with `release --clean`

### Tag-based release

```bash
git pull --ff-only
git tag v<semver>
git push origin v<semver>
```

### Manual re-release

The workflow supports `workflow_dispatch` with an existing tag. It does not invent or move tags.

## GoReleaser configuration

[`/.goreleaser.yaml`](/.goreleaser.yaml) cross-compiles native binaries:

| Platform | Architectures | Archive format |
|---|---|---|
| Linux | amd64, arm64 | tar.gz |
| Windows | amd64, arm64 | zip |
| macOS | amd64, arm64 | tar.gz |

All builds use `CGO_ENABLED=0` for static linking. Archives include `README.md`, `LICENSE`, and `examples/*`.

SHA-256 checksums are uploaded as `checksums.txt`.

## Docker image

[`/Dockerfile`](/Dockerfile) provides a containerized build:

- **Stage 1**: `golang:1.23-bookworm` — Tests and builds the binary.
- **Stage 2**: `node:22-bookworm-slim` — Installs OpenWiki 0.2.0 and Mermaid CLI 11.12.0, plus Git, Chromium (for Mermaid rendering), and ca-certificates.
- **Result**: A minimal image with `wikiforge`, `openwiki`, and Mermaid CLI ready.

Environment: `PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium`, `OPENWIKI_TELEMETRY_DISABLED=1`.

## Build verification

[`/BUILD-VERIFICATION.md`](/BUILD-VERIFICATION.md) documents the verified build process for the current source revision:

- `go test -count=1 ./...` and `go test -race -count=1 ./...` passed
- `go vet ./...` passed
- OpenWiki executable runner contract tests passed
- Large-prompt transport regression tested (160 KB prompts, <4 KiB arguments)
- Cross-platform path tests on Windows, macOS, and Linux
- Doctor prompt-transport preflight tested
- Documentation contract tests for all profiles
- Cross-compilation for all 6 target platforms

## Release procedure

The [`/RELEASING.md`](/RELEASING.md) document records the full release procedure:

1. Ensure CI passes on `main`.
2. `git pull --ff-only`
3. `git tag v<semver>` and `git push origin v<semver>`
4. The release workflow tests, vets, and runs GoReleaser.
5. Archives appear on GitHub Releases.

### Pre-tag verification

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
goreleaser release --snapshot --clean
```

## Source map

| File | Role |
|---|---|
| `/.github/workflows/ci.yml` | Multi-OS CI verification |
| `/.github/workflows/release.yml` | Tag-triggered GoReleaser release |
| `/.goreleaser.yaml` | GoReleaser cross-compilation config |
| `/Dockerfile` | Multi-stage Docker build |
| `/RELEASING.md` | Release procedure documentation |
| `/BUILD-VERIFICATION.md` | Build verification evidence |

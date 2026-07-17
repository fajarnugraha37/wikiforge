# Releasing WikiForge

GitHub Releases are created from version tags. A normal push or pull-request CI run verifies the code but does not publish binaries.

## Publish a release

From an up-to-date `main` branch:

```bash
git pull --ff-only
git tag v<semver>
git push origin v<semver>
```

Replace `<semver>` with the intended release version, for example `1.3.0`. Never move or reuse an existing release tag.

The `release` workflow will:

1. check out the tagged commit with full history;
2. verify the tag format;
3. run `go test ./...`;
4. run `go vet ./...`;
5. run GoReleaser;
6. create the GitHub Release;
7. upload Windows, Linux, and macOS archives for amd64 and arm64;
8. upload `checksums.txt`.

Windows archives are ZIP files. Linux and macOS archives are `tar.gz` files, as configured in `.goreleaser.yaml`.

## Re-run an existing tagged release

The workflow also supports manual dispatch with an existing v-prefixed tag. It intentionally does not invent or move tags: the selected tag must already exist and identify the exact commit to release.

## Verify locally before tagging

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
goreleaser release --snapshot --clean
```

Only push a release tag after the branch CI succeeds and the intended commit is on `main`.

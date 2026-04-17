# sqlite-vec prebuilt binaries

These files are **placeholders** (0 bytes) in the source tree. They are
committed under their canonical filenames so that `//go:embed` in
`../embed.go` compiles on a fresh clone. At build time, the real release
artifacts from https://github.com/asg017/sqlite-vec/releases must replace
these placeholders — otherwise the runtime will detect the empty file at
`Extract()` time and return `ErrEmptyExtension`, and the caller will fall
back to pure-Go brute-force / HNSW.

## Required filenames

The mapping is `vec0-<GOOS>-<GOARCH>.<ext>` where `<ext>` is `so` on Linux
and `dylib` on macOS. Windows is **not supported**.

| OS     | Arch  | File                                  | Source archive (from sqlite-vec release) |
|--------|-------|---------------------------------------|------------------------------------------|
| linux  | amd64 | `vec0-linux-amd64.so`                 | `sqlite-vec-<ver>-loadable-linux-x86_64.tar.gz` → `vec0.so` |
| linux  | arm64 | `vec0-linux-arm64.so`                 | `sqlite-vec-<ver>-loadable-linux-aarch64.tar.gz` → `vec0.so` |
| darwin | amd64 | `vec0-darwin-amd64.dylib`             | `sqlite-vec-<ver>-loadable-macos-x86_64.tar.gz` → `vec0.dylib` |
| darwin | arm64 | `vec0-darwin-arm64.dylib`             | `sqlite-vec-<ver>-loadable-macos-aarch64.tar.gz` → `vec0.dylib` |

## How to drop in the real binaries

```bash
# Example for linux amd64, release v0.1.7-alpha.2:
VER=v0.1.7-alpha.2
curl -L -o /tmp/vec.tar.gz \
  "https://github.com/asg017/sqlite-vec/releases/download/${VER}/sqlite-vec-${VER#v}-loadable-linux-x86_64.tar.gz"
tar -xzf /tmp/vec.tar.gz -C /tmp
cp /tmp/vec0.so internal/sqlitevec/assets/vec0-linux-amd64.so
```

Repeat for each supported GOOS/GOARCH pair before cutting a release.

## Why placeholders?

`//go:embed` requires the files to exist at build time. Committing 0-byte
placeholders keeps the build green for contributors on any platform while
making it obvious these aren't the real artifacts. The runtime guards
against the empty case with a typed error (`ErrEmptyExtension`).

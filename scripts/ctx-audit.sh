#!/usr/bin/env bash
# scripts/ctx-audit.sh — Block 3.1 static check.
#
# Two guarantees across internal/{llm,embedder,extractor,crawler,store}:
#   1. No HTTP call bypasses ctx — http.Get / http.Post / client.Get /
#      client.Post / http.DefaultClient / http.NewRequest (non-ctx) are
#      all banned. Use http.NewRequestWithContext + client.Do instead.
#   2. No DB call bypasses ctx — .Query / .Exec / .QueryRow are banned.
#      Use .QueryContext / .ExecContext / .QueryRowContext instead.
#      The migrate() / open() PRAGMA paths are exempt because ctx is
#      not yet available at store construction time.
#
# Exits non-zero if any violation is found. Intended as a CI gate.
#
# Note on exported-func auditing: doing a robust first-arg-type check in
# bash against Go source requires a full parser (return tuples like
# `func F(int) (int, error)` break naive regex). The Go compiler itself
# plus `go vet` already ensures ctx-accepting function signatures are
# respected at every call site; the value this script adds is the I/O
# side-channel check that vet does not cover. We intentionally scope to
# (1) + (2).
set -euo pipefail

PACKAGES=(
  internal/llm
  internal/embedder
  internal/extractor
  internal/crawler
  internal/store
)

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

fail=0

echo "==> HTTP calls without ctx"
for pkg in "${PACKAGES[@]}"; do
  hits="$(grep -rnE \
    -e 'http\.Get\(' \
    -e 'http\.Post\(' \
    -e '(^|[^A-Za-z_])client\.Get\(' \
    -e '(^|[^A-Za-z_])client\.Post\(' \
    -e 'http\.DefaultClient\.' \
    -e 'http\.NewRequest\(' \
    "$pkg" --include='*.go' --exclude='*_test.go' 2>/dev/null || true)"
  if [ -n "$hits" ]; then
    echo "$hits"
    fail=1
  fi
done

echo "==> DB calls without ctx"
for pkg in "${PACKAGES[@]}"; do
  # Find .Query/.Exec/.QueryRow( that are not *Context variants.
  # Strip comment-only lines. Then exclude the two known-safe lines
  # inside internal/store/store.go where ctx is not yet available:
  #   - migrate()'s schema + migrations execs
  #   - open()'s PRAGMA exec (before the Store is returned)
  hits="$(grep -rnE \
    -e '\.(Query|Exec|QueryRow)\(' \
    "$pkg" --include='*.go' --exclude='*_test.go' 2>/dev/null \
    | grep -vE '\.(Query|Exec|QueryRow)Context\(' \
    | grep -vE '^\S+\.go:[0-9]+:\s*//' \
    | grep -v 'store\.go.*s\.db\.Exec(schema)' \
    | grep -v 'store\.go.*s\.db\.Exec(m)' \
    | grep -vE 'store\.go:[0-9]+:\s*if _, err := db\.Exec\(p\)' \
    || true)"
  if [ -n "$hits" ]; then
    echo "$hits"
    fail=1
  fi
done

if [ $fail -ne 0 ]; then
  echo ""
  echo "CTX AUDIT FAILED — see output above."
  exit 1
fi
echo "CTX AUDIT OK"

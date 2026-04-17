#!/bin/sh
# docsiq SessionStart hook — POSIX shell, no bashisms.
#
# Claude Code protocol: stdin is JSON from the client, stdout is injected
# into the AI context as additional context. Non-zero exit is tolerated
# but we always exit 0 so failures never block an agent.

INPUT=$(cat)

# Extract cwd + hook_event_name without requiring jq. We accept either:
#   "cwd":"/foo"           (no spaces)
#   "cwd": "/foo"          (with space)
# Path values cannot contain literal double quotes so this regex is safe.
extract() {
  key="$1"
  printf '%s' "$INPUT" | sed -n 's/.*"'"$key"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1
}

CWD=$(extract "cwd")
EVENT=$(extract "hook_event_name")

[ -z "$CWD" ] && CWD=$(pwd)
[ -n "$EVENT" ] && [ "$EVENT" != "SessionStart" ] && exit 0

REMOTE=$(git -C "$CWD" remote get-url origin 2>/dev/null) || exit 0
[ -z "$REMOTE" ] && exit 0

SERVER="${DOCSIQ_URL:-http://127.0.0.1:${DOCSIQ_SERVER_PORT:-8080}}"
BODY=$(printf '{"remote":"%s","cwd":"%s"}' "$REMOTE" "$CWD")

AUTH_HEADER=
if [ -n "${DOCSIQ_API_KEY:-}" ]; then
  AUTH_HEADER="-H Authorization: Bearer ${DOCSIQ_API_KEY}"
fi

# curl options: silent, fail on HTTP >= 400, 5s timeout, no proxy nonsense.
# We use -w '%{http_code}' to detect 204 without parsing headers.
RESPONSE_FILE=$(mktemp 2>/dev/null || echo "/tmp/docsiq-hook-$$")
trap 'rm -f "$RESPONSE_FILE"' EXIT

# shellcheck disable=SC2086
STATUS=$(curl -s -o "$RESPONSE_FILE" -w '%{http_code}' \
  --max-time 5 \
  -X POST \
  -H 'Content-Type: application/json' \
  $AUTH_HEADER \
  -d "$BODY" \
  "${SERVER}/api/hook/SessionStart" 2>/dev/null) || {
    echo "docsiq: hook server unreachable at ${SERVER}" >&2
    exit 0
  }

case "$STATUS" in
  200)
    # The docsiq API returns {"project":"...","additionalContext":"..."}.
    # Emit just additionalContext so Claude Code injects it verbatim.
    CTX=$(sed -n 's/.*"additionalContext"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$RESPONSE_FILE" | head -n1)
    [ -n "$CTX" ] && printf '%s\n' "$CTX"
    ;;
  204)
    : # known unregistered remote — silent success
    ;;
  *)
    echo "docsiq: hook server returned HTTP $STATUS" >&2
    ;;
esac

exit 0

# Hooks

docsiq registers a **SessionStart** hook with supported AI clients so
that whenever a coding session begins, the client POSTs the repo's git
remote to docsiq, and docsiq replies with a project-scoped context
blob that the client injects into the agent's prompt.

The endpoint is `POST /api/hook/SessionStart`. See [rest-api.md](./rest-api.md)
for the request / response shape.

## Install

```bash
docsiq hooks install                  # all supported clients
docsiq hooks install --client claude  # just Claude Code
docsiq hooks install --dry-run        # preview without writing files
docsiq hooks install --hook-url http://192.168.1.10:8080   # custom server URL
```

On install, docsiq:

1. Extracts `hook.sh` to `$DATA_DIR/hooks/hook.sh` (0755). The script
   reads the client's JSON payload, resolves the local git remote with
   `git -C "$CWD" remote get-url origin`, and POSTs the result to the
   configured hook URL.
2. Writes the client-specific config fragment (schema depends on the
   client — see the matrix below).

Uninstall reverses step 2 and leaves `hook.sh` in place. `docsiq hooks
status` prints which clients currently have a docsiq hook registered.

## Support matrix

docsiq registers a SessionStart hook with each AI client so GraphRAG
context is loaded when the client starts. Only Claude Code publishes a
documented SessionStart hook schema — the others are a best-effort
placeholder pinned by fixture tests. Installing an unverified hook
prints a `slog.Warn` so operators can opt out.

| Client | Config path | Schema source | Status |
|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | [docs.claude.com/en/docs/claude-code/hooks](https://docs.claude.com/en/docs/claude-code/hooks) (fetched 2026-04-17) | verified |
| Cursor | `~/.cursor/docsiq-hooks.json` | no docs (docs.cursor.com/en/agent/hooks returned empty, 2026-04-17) | unverified |
| Copilot CLI | `~/.config/github-copilot/hooks.json` | no docs (github.com/copilot CLI docs publish no hook schema, 2026-04-17) | unverified |
| Codex CLI | `~/.codex/hooks.json` | no docs (github.com/openai/codex `docs/config.md` documents only a `Notify` post-turn hook, no SessionStart, 2026-04-17) | unverified |

**Unverified** means we mirrored the shape used by the original
kgraph codebase (which was itself a best-effort guess). When a client
publishes a real schema, the corresponding installer in
`internal/hookinstaller/` should be updated along with its fixture pair
in `internal/hookinstaller/fixtures/<client>/`.

### Verified vs unverified — what changes

- Verified installers match the client's published schema exactly; a
  schema bump on the client side will break our installer and needs an
  update on our side.
- Unverified installers write the kgraph-style shape and may be
  ignored entirely by the client. An unverified install is a no-op
  until either the schema matches or the client starts honoring the
  kgraph shape.

## How the hook fires

Claude Code example flow:

1. User runs `claude` inside `~/src/my-repo`.
2. Claude Code reads `~/.claude/settings.json`, sees the docsiq hook,
   and invokes `$DATA_DIR/hooks/hook.sh` with a JSON payload on stdin.
3. `hook.sh` resolves the repo's origin remote and POSTs it to
   `http://127.0.0.1:8080/api/hook/SessionStart`.
4. docsiq normalizes the remote with `project.Slug()`, looks up the
   registry, and responds:
   - Registered → `200 {project, additionalContext}`. Claude Code
     injects `additionalContext` into the prompt.
   - Unregistered → `204 No Content`. Claude Code stays silent.

## Troubleshooting: "my hook doesn't fire"

Work the list top-to-bottom:

1. **Is the hook actually installed?**
   ```
   docsiq hooks status
   ```
   Every client listed should say `registered`. If it says `missing`,
   run `docsiq hooks install --client <name>`.

2. **Does `hook.sh` exist and is it executable?**
   ```
   ls -la $(docsiq stats --json | jq -r .data_dir)/hooks/hook.sh
   ```
   Should be mode `0755`. Re-run install to re-extract if missing.

3. **Is the docsiq server reachable from the client?**
   ```
   curl -s http://127.0.0.1:8080/health
   # {"status":"ok"}
   ```
   If not: `docsiq serve` is not running, or a firewall blocks the
   hook URL, or `--hook-url` was overridden to an unreachable address.

4. **Did the client ever call the hook?**
   Check the docsiq server logs for a line like:
   ```
   level=INFO msg=http method=POST path=/api/hook/SessionStart status=200
   ```
   If you never see that line, the client isn't firing the hook at
   all — almost always a schema mismatch (see unverified clients
   above) or a typo'd config path.

5. **Is your repo registered?**
   An unregistered remote yields `204 No Content` — the hook ran, but
   docsiq has nothing to say.
   ```
   docsiq projects list
   docsiq init               # inside the repo, if missing
   ```

6. **Is the bearer key set correctly?**
   When `server.api_key` / `DOCSIQ_API_KEY` is configured, `hook.sh`
   needs the matching key. The extracted script reads
   `$DOCSIQ_API_KEY` from the client's environment — make sure that's
   exported at the shell launching the client. A 401 response in the
   logs is the signal.

7. **Is the client running from the right cwd?**
   Claude Code, Cursor, etc. each resolve the "current repo" slightly
   differently. If `hook.sh` runs outside a git checkout, the
   `remote` field is empty and docsiq replies `400`. Launch the
   client from inside the repo tree.

8. **Does `docsiq hooks install --dry-run` show what you expect?**
   Dry-run prints the exact JSON it would write. If the config shape
   looks wrong for your client's version, capture the output and file
   a bug with the client's own hook docs attached.

## Customizing `additionalContext`

The response body is generated server-side from the project's
registered name + slug. To tailor it further, index a note named
`_session-prelude.md` in the project's notes directory — the hook
handler prepends that note's content to `additionalContext` when
present.

# Block 2 — Security & Auth Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the docsiq HTTP boundary and config loader — reject malformed config early, scrub secrets from debug logs, enforce browser-side CSP + baseline security headers, and thread the existing request ID into slog for operator-friendly log correlation.

**Architecture:** Five independent server-side changes, four of them behind an existing middleware chain (`loggingMiddleware → recoveryMiddleware → bearerAuthMiddleware → projectMiddleware → mux`). Tasks 1–2 touch `internal/config`; Tasks 3–4 add a single new `securityHeadersMiddleware` placed as the outermost wrapper so every response (including 404s and errors) carries the headers; Task 5 adds a small `ContextLogger(ctx)` helper next to the existing `RequestIDFromContext` and updates a curated set of handler log sites.

**Tech Stack:** Go 1.22+ standard `net/http`, `log/slog`, `reflect`, Viper + mapstructure. No new third-party dependencies.

**Scope check:** Five items, one subsystem (HTTP + config). No sub-plan decomposition needed.

**Self-contained:** Block 1 (PR #60) is a soft prerequisite only because Block 2 builds on the same middleware chain. None of Block 2 touches workq, the session cookie path, or the entity-fetch scoping from Block 1.

---

## File Structure

### Create
- `internal/api/security_headers.go` — `securityHeadersMiddleware(cfg *config.Config) func(http.Handler) http.Handler`; returns a handler that sets CSP + baseline headers on every response.
- `internal/api/security_headers_test.go` — header-presence assertions under `GET /health`, `OPTIONS /api/ping`, and authenticated `GET /api/stats`.
- `internal/api/log_context.go` — `ContextLogger(ctx context.Context) *slog.Logger` helper that enriches `slog.Default()` with `req_id` when the context carries one.
- `internal/api/log_context_test.go` — captures slog output via a `slog.NewTextHandler(&buf, nil)` default, asserts `req_id=xxxx` is present on emitted records.
- `internal/config/redact.go` — `(c *Config) Redact() *Config`; deep-copies and zeroes every string field tagged `secret:"true"` via `reflect`.
- `internal/config/redact_test.go` — round-trip test: load a config with known secrets, serialize the redacted copy to YAML, assert no secret substring appears.

### Modify
- `internal/config/config.go:48-128` — add `secret:"true"` struct tags on API-key fields in `ServerConfig`, `OpenAIConfig`, `AzureConfig`, `AzureServiceConfig`.
- `internal/config/config.go:256` — swap `viper.Unmarshal(&cfg)` for `viper.UnmarshalExact(&cfg)`; add call to `validateLLM(&cfg)` before returning.
- `internal/config/config.go` (new function at end) — `validateLLM(cfg *Config) error` with provider switch.
- `internal/config/config.go:145-152` — add `HSTSEnabled bool `mapstructure:"hsts_enabled"`` to `ServerConfig` (no `secret` tag).
- `internal/config/config.go:238-241` — add `viper.BindEnv("server.hsts_enabled")`.
- `internal/config/config_test.go` (existing file; append) — tests for `validateLLM` + `UnmarshalExact` rejection of unknown keys.
- `internal/api/router.go:167-170` — wrap the existing middleware chain with `securityHeadersMiddleware(cfg)` as the new outermost layer.
- `internal/api/handlers.go` (representative sites) — replace 3–5 `slog.Warn` / `slog.Error` calls in request handlers with `ContextLogger(r.Context()).Warn(...)` equivalents.

---

## Task 1: Strict config unmarshal + LLM consistency (2.4)

**Files:**
- Modify: `internal/config/config.go:256` (Unmarshal → UnmarshalExact)
- Modify: `internal/config/config.go` — append `validateLLM(cfg *Config) error`
- Modify: `internal/config/config_test.go` — append new tests

- [ ] **Step 1: Read current state**

Confirm the current load pattern:

```bash
sed -n '250,265p' internal/config/config.go
grep -n 'UnmarshalExact\|Unmarshal' internal/config/config.go
```

Expected: one existing `viper.Unmarshal(&cfg)` call at line 256 (give or take 3 lines if the file has drifted), no existing `UnmarshalExact`.

- [ ] **Step 2: Write the failing tests (TDD red)**

Append to `internal/config/config_test.go`:

```go
func TestLoad_RejectsUnknownKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	must := func(err error) { t.Helper(); if err != nil { t.Fatal(err) } }
	must(os.WriteFile(f, []byte("server:\n  api_key: s3cret\n  unknown_key: oops\n"), 0o600))

	_, err := Load(f)
	if err == nil {
		t.Fatal("Load should reject unknown_key")
	}
	if !strings.Contains(err.Error(), "unknown_key") {
		t.Fatalf("error should name the offending key; got %q", err)
	}
}

func TestLoad_ValidatesLLMProvider(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "unknown_provider",
			yaml:    "llm:\n  provider: not_a_real_one\n",
			wantErr: "provider",
		},
		{
			name:    "azure_missing_endpoint",
			yaml:    "llm:\n  provider: azure\n  azure:\n    api_key: k\n",
			wantErr: "azure",
		},
		{
			name:    "openai_missing_api_key",
			yaml:    "llm:\n  provider: openai\n  openai:\n    base_url: https://api.openai.com/v1\n",
			wantErr: "openai",
		},
		{
			name:    "ollama_missing_base_url",
			yaml:    "llm:\n  provider: ollama\n",
			wantErr: "ollama",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			f := filepath.Join(dir, "config.yaml")
			must := func(err error) { t.Helper(); if err != nil { t.Fatal(err) } }
			must(os.WriteFile(f, []byte("server:\n  api_key: s3cret\n"+tc.yaml), 0o600))

			_, err := Load(f)
			if err == nil {
				t.Fatalf("Load should have rejected %s", tc.name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.wantErr) {
				t.Fatalf("error should mention %q; got %q", tc.wantErr, err)
			}
		})
	}
}

func TestLoad_AcceptsValidProviders(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		yaml string
	}{
		{"azure", "llm:\n  provider: azure\n  azure:\n    endpoint: https://x.openai.azure.com\n    api_key: k\n    api_version: 2024-02-15-preview\n    chat:\n      model: gpt-4o\n    embed:\n      model: text-embedding-3-small\n"},
		{"openai", "llm:\n  provider: openai\n  openai:\n    api_key: k\n    chat_model: gpt-4o\n    embed_model: text-embedding-3-small\n"},
		{"ollama", "llm:\n  provider: ollama\n  ollama:\n    base_url: http://127.0.0.1:11434\n    chat_model: llama3\n    embed_model: nomic-embed-text\n"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			f := filepath.Join(dir, "config.yaml")
			must := func(err error) { t.Helper(); if err != nil { t.Fatal(err) } }
			must(os.WriteFile(f, []byte("server:\n  api_key: s3cret\n"+tc.yaml), 0o600))

			cfg, err := Load(f)
			if err != nil {
				t.Fatalf("valid %s config should load: %v", tc.name, err)
			}
			if cfg.LLM.Provider != tc.name {
				t.Fatalf("provider not round-tripped: got %q", cfg.LLM.Provider)
			}
		})
	}
}
```

Ensure the test file's import block includes `"os"`, `"path/filepath"`, `"strings"`, `"testing"`. Add any missing.

- [ ] **Step 3: Run the tests (red)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/config/ -run 'TestLoad_RejectsUnknownKey|TestLoad_ValidatesLLMProvider|TestLoad_AcceptsValidProviders' -v
```

Expected: `TestLoad_RejectsUnknownKey` FAILs (viper.Unmarshal accepts unknown keys silently). `TestLoad_ValidatesLLMProvider` subtests FAIL (no validator exists yet). Valid-provider tests should PASS already.

- [ ] **Step 4: Swap to UnmarshalExact**

Edit `internal/config/config.go` around line 256:

```go
// OLD
if err := v.Unmarshal(&cfg); err != nil {
    return nil, fmt.Errorf("unmarshal config: %w", err)
}

// NEW
if err := v.UnmarshalExact(&cfg); err != nil {
    return nil, fmt.Errorf("unmarshal config: %w", err)
}
```

- [ ] **Step 5: Add `validateLLM`**

Append to the end of `internal/config/config.go`:

```go
// validateLLM enforces that the selected LLM provider has the minimum
// fields needed to make any request. Called from Load after
// UnmarshalExact so the "unknown key" and "missing required field"
// errors land in a consistent spot. Error messages name the offending
// provider so an operator can grep logs → yaml key immediately.
func validateLLM(cfg *Config) error {
	switch cfg.LLM.Provider {
	case "":
		// Provider unset is allowed — search paths that don't need an LLM
		// (e.g. pure-FTS search) still work. Fail only if someone later
		// tries to construct an LLM client with an empty provider string.
		return nil
	case "azure":
		a := cfg.LLM.Azure
		chatOK := a.Chat.Endpoint != "" || a.Endpoint != ""
		chatOK = chatOK && (a.Chat.APIKey != "" || a.APIKey != "")
		embedOK := a.Embed.Endpoint != "" || a.Endpoint != ""
		embedOK = embedOK && (a.Embed.APIKey != "" || a.APIKey != "")
		if !chatOK && !embedOK {
			return fmt.Errorf("llm.azure: neither chat nor embed has a resolvable endpoint+api_key (set shared llm.azure.{endpoint,api_key} or per-service overrides)")
		}
		if a.APIVersion == "" && a.Chat.APIVersion == "" && a.Embed.APIVersion == "" {
			return fmt.Errorf("llm.azure.api_version: required (shared or per-service)")
		}
	case "openai":
		if cfg.LLM.OpenAI.APIKey == "" {
			return fmt.Errorf("llm.openai.api_key: required when llm.provider=openai")
		}
	case "ollama":
		if cfg.LLM.Ollama.BaseURL == "" {
			return fmt.Errorf("llm.ollama.base_url: required when llm.provider=ollama")
		}
	default:
		return fmt.Errorf("llm.provider: unknown value %q (valid: azure, openai, ollama)", cfg.LLM.Provider)
	}
	return nil
}
```

Now wire it into `Load`. Inside `Load`, after the `UnmarshalExact` call and before the final `return &cfg, nil`, add:

```go
if err := validateLLM(&cfg); err != nil {
    return nil, fmt.Errorf("config validation: %w", err)
}
```

- [ ] **Step 6: Run tests (green)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/config/ -v
```

Expected: all new tests PASS, existing config tests still PASS.

If `TestLoad_RejectsUnknownKey` still fails: viper's `UnmarshalExact` may not catch nested unknown keys in all versions. Verify the error message content. If it rejects but doesn't name the offending key, the test's `strings.Contains(err.Error(), "unknown_key")` assertion needs to match what viper actually reports — inspect the error and adjust the substring.

- [ ] **Step 7: Run full Go suite**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "$(cat <<'EOF'
feat(config): reject unknown keys and invalid LLM provider configs

- Load now uses viper.UnmarshalExact, surfacing typos and stale keys
  at startup instead of at first-use.
- validateLLM enforces per-provider minimums (azure endpoint+api_key
  and api_version; openai api_key; ollama base_url). Error messages
  name the offending yaml key so an operator can grep → fix in one
  hop.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: `Config.Redact()` + audit logged config (2.3)

**Files:**
- Create: `internal/config/redact.go`
- Create: `internal/config/redact_test.go`
- Modify: `internal/config/config.go` — add `secret:"true"` tags on API-key fields
- Modify: `internal/config/config_test.go` — minor imports if needed (no new tests in this file)
- Modify: zero-to-three call sites in `internal/config` / `internal/llm` / `internal/api` that log config (audit step; see Step 7)

- [ ] **Step 1: Tag the secret fields**

Edit `internal/config/config.go`. Add `secret:"true"` to the following fields (keep existing `mapstructure` tags):

- `ServerConfig.APIKey`:
  ```go
  APIKey string `mapstructure:"api_key" secret:"true"`
  ```
- `OpenAIConfig.APIKey`:
  ```go
  APIKey string `mapstructure:"api_key" secret:"true"`
  ```
- `AzureConfig.APIKey`:
  ```go
  APIKey string `mapstructure:"api_key" secret:"true"`
  ```
- `AzureServiceConfig.APIKey` (used by both `Chat` and `Embed`):
  ```go
  APIKey string `mapstructure:"api_key" secret:"true"`
  ```

- [ ] **Step 2: Write the failing test**

Create `internal/config/redact_test.go`:

```go
package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfig_Redact_ZeroesSecrets(t *testing.T) {
	t.Parallel()
	in := &Config{}
	in.Server.APIKey = "server-secret"
	in.LLM.Provider = "azure"
	in.LLM.Azure.APIKey = "azure-shared-secret"
	in.LLM.Azure.Chat.APIKey = "azure-chat-secret"
	in.LLM.Azure.Embed.APIKey = "azure-embed-secret"
	in.LLM.OpenAI.APIKey = "openai-secret"

	redacted := in.Redact()

	b, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"server-secret",
		"azure-shared-secret",
		"azure-chat-secret",
		"azure-embed-secret",
		"openai-secret",
	} {
		if strings.Contains(string(b), s) {
			t.Fatalf("redacted output still contains %q:\n%s", s, b)
		}
	}

	// Original must be untouched.
	if in.Server.APIKey != "server-secret" {
		t.Fatalf("Redact mutated the original Config")
	}
}

func TestConfig_Redact_PreservesNonSecretFields(t *testing.T) {
	t.Parallel()
	in := &Config{}
	in.Server.Host = "127.0.0.1"
	in.Server.Port = 8080
	in.Server.APIKey = "s3cret"
	in.LLM.Provider = "openai"
	in.LLM.OpenAI.BaseURL = "https://api.openai.com/v1"
	in.LLM.OpenAI.ChatModel = "gpt-4o"

	r := in.Redact()
	if r.Server.Host != "127.0.0.1" {
		t.Errorf("Host lost")
	}
	if r.Server.Port != 8080 {
		t.Errorf("Port lost")
	}
	if r.LLM.Provider != "openai" {
		t.Errorf("Provider lost")
	}
	if r.LLM.OpenAI.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL lost")
	}
	if r.LLM.OpenAI.ChatModel != "gpt-4o" {
		t.Errorf("ChatModel lost")
	}
	if r.Server.APIKey != "" {
		t.Errorf("Server.APIKey should be zeroed; got %q", r.Server.APIKey)
	}
}
```

- [ ] **Step 3: Run tests (red)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/config/ -run TestConfig_Redact -v
```

Expected: FAIL — `*Config has no method Redact`.

- [ ] **Step 4: Implement `Redact`**

Create `internal/config/redact.go`:

```go
package config

import "reflect"

// Redact returns a deep copy of c with every string field tagged
// secret:"true" zeroed. The original c is not mutated. Safe for logging
// and for serializing config for introspection endpoints.
//
// Nested structs are walked recursively. Slices, maps, and pointers to
// structs are supported, though config.Config uses only direct struct
// nesting today — the broader coverage is cheap insurance.
func (c *Config) Redact() *Config {
	if c == nil {
		return nil
	}
	dup := *c
	zeroSecrets(reflect.ValueOf(&dup).Elem())
	return &dup
}

func zeroSecrets(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			tag := v.Type().Field(i).Tag.Get("secret")
			if tag == "true" && f.Kind() == reflect.String && f.CanSet() {
				f.SetString("")
				continue
			}
			zeroSecrets(f)
		}
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			zeroSecrets(v.Elem())
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			zeroSecrets(v.Index(i))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			// Maps in Go reflect are not addressable; copy out, zero, put back.
			elem := reflect.New(v.Type().Elem()).Elem()
			elem.Set(v.MapIndex(k))
			zeroSecrets(elem)
			v.SetMapIndex(k, elem)
		}
	}
}
```

- [ ] **Step 5: Run tests (green)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/config/ -run TestConfig_Redact -v
```

Expected: both tests PASS.

- [ ] **Step 6: Audit — find existing config log sites**

```
grep -rn 'slog\.\(Debug\|Info\|DebugContext\|InfoContext\).*cfg\b' internal/ || true
grep -rn 'slog\.\(Debug\|Info\|DebugContext\|InfoContext\).*config\b' internal/ || true
grep -rn 'fmt\.Printf.*cfg\.' internal/ || true
grep -rn 'fmt\.Println.*cfg\.' internal/ || true
```

For each hit that logs a `*Config` value OR the full `cfg` struct:
- Replace `slog.Info("loaded config", "cfg", cfg)` → `slog.Info("loaded config", "cfg", cfg.Redact())`
- Replace `slog.Debug("config", "value", cfg)` → `slog.Debug("config", "value", cfg.Redact())`

If a site logs only specific non-secret fields (e.g., `slog.Info("host", "host", cfg.Server.Host)`), leave it alone. Only change sites that log the struct as a whole.

If NO call sites are found, record that finding in the commit message and move on — the helper is still valuable as an available tool for future code.

- [ ] **Step 7: Re-run full config + full Go suite**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/config/... -v
CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...
```

Expected: all pass. The `secret:"true"` tag addition is purely additive and should not affect any existing behavior.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go internal/config/redact.go internal/config/redact_test.go
# Plus any audited log-site edits from Step 6:
# git add internal/config/<...>.go internal/api/<...>.go internal/llm/<...>.go
git commit -m "$(cat <<'EOF'
feat(config): redact helper zeroes fields tagged secret:"true"

Config.Redact() returns a deep copy with api_key fields cleared. Uses
the new secret:"true" struct tag so future additions opt in simply by
tagging the field. Audited existing log sites that printed the full
Config; piped them through Redact() so debug logs no longer leak
keys.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Content-Security-Policy middleware (2.1)

**Files:**
- Create: `internal/api/security_headers.go`
- Create: `internal/api/security_headers_test.go`
- Modify: `internal/api/router.go:167-170` — add the wrapper

- [ ] **Step 1: Write the failing test**

Create `internal/api/security_headers_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestSecurityHeaders_CSPOnEveryResponse(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP header missing")
	}
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self' 'wasm-unsafe-eval'",
		"style-src 'self' 'unsafe-inline'",
		"connect-src 'self'",
		"img-src 'self' data:",
		"font-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing directive %q: got %q", want, csp)
		}
	}
}

func TestSecurityHeaders_SkipsOPTIONS(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/ping", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Security-Policy") != "" {
		t.Errorf("CSP should not be set on OPTIONS; got %q", rr.Header().Get("Content-Security-Policy"))
	}
	if rr.Code != http.StatusNoContent {
		t.Errorf("OPTIONS should pass through; got status %d", rr.Code)
	}
}

func TestSecurityHeaders_PreservesExistingHeaders(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "xyz")
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("X-Custom") != "xyz" {
		t.Errorf("downstream header clobbered")
	}
}
```

- [ ] **Step 2: Run tests (red)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestSecurityHeaders -v
```

Expected: FAIL — `undefined: securityHeadersMiddleware`.

- [ ] **Step 3: Implement the middleware (CSP only for now)**

Create `internal/api/security_headers.go`:

```go
package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// contentSecurityPolicy is deliberately strict for the air-gapped
// deployment posture: no CDN origins, no inline scripts, WASM allowed
// (shiki uses it for syntax highlighting), no iframing. Inline styles
// are permitted because Tailwind + shadcn/ui emit them.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'wasm-unsafe-eval'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"connect-src 'self'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'"

// securityHeadersMiddleware sets browser-side hardening headers on every
// response that actually carries a body (i.e. non-OPTIONS). Task 4
// extends this middleware with baseline security headers
// (nosniff, Referrer-Policy, Permissions-Policy, HSTS). Keeping both
// sets in one middleware keeps response-header side-effects colocated.
func securityHeadersMiddleware(_ *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run tests (green)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestSecurityHeaders -v
```

Expected: all three PASS.

- [ ] **Step 5: Wire into router**

Edit `internal/api/router.go` around line 167. Current:

```go
return loggingMiddleware(
    recoveryMiddleware(
        bearerAuthMiddleware(cfg.Server.APIKey,
            projectMiddleware(cfg, registry, mux))))
```

Change to:

```go
return securityHeadersMiddleware(cfg)(
    loggingMiddleware(
        recoveryMiddleware(
            bearerAuthMiddleware(cfg.Server.APIKey,
                projectMiddleware(cfg, registry, mux)))))
```

Rationale: security headers must be applied to every response, including panic recoveries and 401s. Wrapping outside `loggingMiddleware` also means the logged response status already reflects the final body; no functional reorder.

- [ ] **Step 6: Run full Go suite + new middleware tests**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -v
CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...
```

Expected: all pass. Any existing test that asserts on header ABSENCE (e.g. auth_test.go assertions) must still pass because we only add headers.

- [ ] **Step 7: Run UI Playwright smokes to check the SPA still loads under CSP**

```
cd ui && CI=1 ./node_modules/.bin/playwright test smoke.spec.ts --reporter=list --workers=1
```

Expected: 5/5 pass. If any smoke fails with a browser console error about CSP violation, collect the exact violation from the Playwright trace; most likely an inline `<script>` or `<style>` the bundler generates. The CSP directive includes `'unsafe-inline'` for styles but NOT for scripts — Vite's production build should not emit inline scripts (all are hashed-module), but verify.

If the smoke fails due to a CSP violation from `style-src`, the test is passing under a browser that didn't previously enforce CSP; investigate the specific violation line from the Playwright `--reporter=list` output. If it's a Vite dev-server artifact that leaked into the production build, adjust the bundler; do not widen CSP.

If Playwright is not installed in this environment, skip Step 7 and note it in the commit message. The CI Playwright job will catch any regression.

- [ ] **Step 8: Commit**

```bash
git add internal/api/security_headers.go internal/api/security_headers_test.go internal/api/router.go
git commit -m "$(cat <<'EOF'
feat(api): Content-Security-Policy middleware

Sets a strict CSP matching the air-gapped deployment posture on every
non-OPTIONS response: default-src 'self' everywhere; inline scripts
banned; WASM allowed for shiki; inline styles allowed for Tailwind/
shadcn; frame-ancestors 'none' to block clickjacking. Wired as the
outermost router wrapper so 401/500/404 responses also carry the
header.

Task 4 will extend the same middleware with baseline security headers
(nosniff, Referrer-Policy, Permissions-Policy, HSTS).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Baseline security headers + conditional HSTS (2.2)

**Files:**
- Modify: `internal/api/security_headers.go` — extend the middleware to set nosniff, Referrer-Policy, Permissions-Policy, HSTS conditional
- Modify: `internal/api/security_headers_test.go` — add assertions
- Modify: `internal/config/config.go` — add `HSTSEnabled bool` to `ServerConfig`
- Modify: `internal/config/config.go` — add `viper.BindEnv("server.hsts_enabled")` next to existing server env bindings

- [ ] **Step 1: Add HSTSEnabled config field**

Edit `internal/config/config.go` — extend the `ServerConfig` struct (currently ends around line 152):

```go
type ServerConfig struct {
    Host           string `mapstructure:"host"`
    Port           int    `mapstructure:"port"`
    APIKey         string `mapstructure:"api_key" secret:"true"`  // already tagged in Task 2
    MaxUploadBytes int64  `mapstructure:"max_upload_bytes"`
    WorkqWorkers   int    `mapstructure:"workq_workers"`
    WorkqDepth     int    `mapstructure:"workq_depth"`
    HSTSEnabled    bool   `mapstructure:"hsts_enabled"`
}
```

And bind the env var near the existing server BindEnv calls (around lines 238-241):

```go
_ = v.BindEnv("server.hsts_enabled")
```

(Use whichever prefix + delimiter pattern the neighbors use — verify with a quick `sed -n '235,245p' internal/config/config.go`.)

- [ ] **Step 2: Write the failing tests**

Append to `internal/api/security_headers_test.go`:

```go
func TestSecurityHeaders_BaselineHeaders(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: want nosniff, got %q", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy: got %q", got)
	}
	perms := rr.Header().Get("Permissions-Policy")
	for _, want := range []string{"camera=()", "microphone=()", "geolocation=()", "payment=()", "usb=()"} {
		if !strings.Contains(perms, want) {
			t.Errorf("Permissions-Policy missing %q: got %q", want, perms)
		}
	}
	if rr.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set when HSTSEnabled=false")
	}
}

func TestSecurityHeaders_HSTSOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	cfg.Server.HSTSEnabled = true
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "max-age=31536000") {
		t.Errorf("HSTS missing max-age; got %q", hsts)
	}
}
```

- [ ] **Step 3: Run tests (red)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestSecurityHeaders -v
```

Expected: two new tests FAIL (headers not yet set). Existing CSP tests still pass.

- [ ] **Step 4: Extend the middleware**

Replace the body of `securityHeadersMiddleware` in `internal/api/security_headers.go`:

```go
const (
	contentSecurityPolicy = "default-src 'self'; " +
		"script-src 'self' 'wasm-unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"connect-src 'self'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'"

	permissionsPolicy = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"

	hstsValue = "max-age=31536000; includeSubDomains"
)

func securityHeadersMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	hstsEnabled := cfg != nil && cfg.Server.HSTSEnabled
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set("Content-Security-Policy", contentSecurityPolicy)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", permissionsPolicy)
			if hstsEnabled {
				h.Set("Strict-Transport-Security", hstsValue)
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 5: Run tests (green)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestSecurityHeaders -v
CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...
```

Expected: all new + existing tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/security_headers.go internal/api/security_headers_test.go internal/config/config.go
git commit -m "$(cat <<'EOF'
feat(api): baseline security headers alongside CSP

Extends securityHeadersMiddleware with:
- X-Content-Type-Options: nosniff
- Referrer-Policy: strict-origin-when-cross-origin
- Permissions-Policy: disables camera/mic/geo/payment/usb
- Strict-Transport-Security: 1y max-age + includeSubDomains, gated by
  server.hsts_enabled (default off) so HTTP-only dev stays unaffected.

Operators behind a TLS terminator or direct TLS should set
DOCSIQ_SERVER_HSTS_ENABLED=true.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Context-aware slog helper + sample call-site refactors (2.5)

**Files:**
- Create: `internal/api/log_context.go`
- Create: `internal/api/log_context_test.go`
- Modify: `internal/api/handlers.go` — 3-5 representative `slog.*` call sites refactored to use `ContextLogger`

- [ ] **Step 1: Write the failing test**

Create `internal/api/log_context_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestContextLogger_AddsReqIDFromContext(t *testing.T) {
	// Cannot t.Parallel() because we mutate slog.Default.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ctx := context.WithValue(context.Background(), ctxRequestIDKey{}, "abc123")
	ContextLogger(ctx).Info("hello", "k", "v")

	out := buf.String()
	if !strings.Contains(out, "req_id=abc123") {
		t.Fatalf("expected req_id=abc123 in log output; got %q", out)
	}
	if !strings.Contains(out, "k=v") {
		t.Fatalf("expected k=v to survive; got %q", out)
	}
}

func TestContextLogger_NoReqIDWhenMissing(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ContextLogger(context.Background()).Info("hello")

	out := buf.String()
	if strings.Contains(out, "req_id=") {
		t.Fatalf("req_id should be absent when context has no ID; got %q", out)
	}
}
```

- [ ] **Step 2: Run tests (red)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestContextLogger -v
```

Expected: FAIL — `undefined: ContextLogger`.

- [ ] **Step 3: Implement the helper**

Create `internal/api/log_context.go`:

```go
package api

import (
	"context"
	"log/slog"
)

// ContextLogger returns slog.Default() enriched with the per-request ID
// when the context carries one. Handler trees that funnel log calls
// through this helper get free request-level log correlation; downstream
// code that needs the ID for metric labels should still read it via
// RequestIDFromContext(ctx) directly.
//
// Callers that don't need the enrichment can keep using slog.Default()
// — the helper is additive, not mandatory.
func ContextLogger(ctx context.Context) *slog.Logger {
	if id := RequestIDFromContext(ctx); id != "" {
		return slog.Default().With("req_id", id)
	}
	return slog.Default()
}
```

- [ ] **Step 4: Run tests (green)**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/api/ -run TestContextLogger -v
```

Expected: both PASS.

- [ ] **Step 5: Refactor a representative set of handler log sites**

Goal: prove the helper composes with existing patterns and give operators immediate value for the most-hit error paths. Scope the refactor to 3–5 sites. Prefer sites that:
- Sit inside an HTTP handler (have `r *http.Request` in scope)
- Currently use `slog.Warn` / `slog.Error` (the signal worth correlating)
- Are on hot or error paths

Good candidates to look for in `internal/api/handlers.go`:
- The `upload` handler's error paths (e.g., the `writeTooLarge`, `ErrQueueFull`, `ErrClosed` branches landed in Block 1)
- The `search` handler's error-branch slog calls
- The `writeError` function itself if it has a `slog.Error` inside

Process:
```bash
grep -nE 'slog\.(Warn|Error)' internal/api/handlers.go | head -20
```

Pick 3-5 sites. For each, change:
```go
slog.Warn("upload failed", "slug", slug, "err", err)
```
to:
```go
ContextLogger(r.Context()).Warn("upload failed", "slug", slug, "err", err)
```

**Do not attempt a full sweep.** Remaining sites are tech debt; a future Block 4 (observability) sweep can cover them. Note the deferral in the commit message.

If a site is inside a helper that doesn't have `r *http.Request` but does have `ctx context.Context`, use `ContextLogger(ctx).Warn(...)`.

- [ ] **Step 6: Run full Go suite to confirm no regression**

```
CGO_ENABLED=1 go test -tags sqlite_fts5 -timeout 300s ./...
```

Expected: all pass. The refactor is behavior-preserving (same log level, same message, same key/value pairs; just one extra field attached).

- [ ] **Step 7: Commit**

```bash
git add internal/api/log_context.go internal/api/log_context_test.go internal/api/handlers.go
git commit -m "$(cat <<'EOF'
feat(api): ContextLogger threads req_id into slog for correlation

Adds an additive helper next to the existing RequestIDFromContext:
ContextLogger(ctx) returns slog.Default() enriched with req_id=<id>
when the context carries one from loggingMiddleware. Refactored
<N> representative error-path log sites in handlers.go to use it.

The rest of the codebase keeps using slog.Default() directly — a
wider sweep is tracked as tech debt for the observability sweep.

Note: the roadmap's 2.5 bullet mentioned ULID. The existing
request_id.go generates 16-char hex from crypto/rand which is
collision-safe for the air-gapped single-binary deploy. Not
worth changing as part of this PR.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

### Spec coverage

- **2.1 CSP header** → Task 3 (full directive list matches spec).
- **2.2 Baseline security headers** → Task 4 (nosniff, Referrer-Policy, Permissions-Policy, HSTS with `server.hsts_enabled` gate; spec allowed `server.tls_cert` as the gate — we use an explicit boolean which is simpler and operator-controllable without adding a TLS-cert loading path in this block).
- **2.3 Secret scrubbing** → Task 2 (`Redact()` helper + `secret:"true"` tagging + audit of existing config-log sites).
- **2.4 Config validation** → Task 1 (`UnmarshalExact` + `validateLLM` with provider-triple checks).
- **2.5 Request-ID middleware** → Task 5 (the ID infrastructure already exists from Block 1's error-shape work; this task closes the slog-threading gap and explicitly defers ULID).

All five items have dedicated tasks.

### Placeholder check

No `TBD`, no `TODO`, no "similar to", no "add appropriate error handling". Every code step has full code. Commands are exact.

Step 5 in Task 5 lists "3-5 sites" rather than naming specific lines — this is intentional because exact line numbers drift (Block 1 moved things around) and the plan author cannot pre-commit to specific sites without reading the tree at implementation time. The step's selection criteria are concrete ("sits inside an HTTP handler", "currently slog.Warn/Error", "error path"), the identification command is exact (`grep -nE 'slog\.(Warn|Error)' internal/api/handlers.go`), and the transformation pattern is shown with a before/after example. This is the most precise the plan can be without becoming brittle against drift.

### Type consistency

- `securityHeadersMiddleware(cfg *config.Config) func(http.Handler) http.Handler` — same signature Task 3 and Task 4. Wired identically at `router.go`.
- `ContextLogger(ctx context.Context) *slog.Logger` — same signature Task 5 throughout.
- `validateLLM(cfg *Config) error` — defined once in Task 1.
- `(c *Config) Redact() *Config` — defined once in Task 2.
- `secret:"true"` struct tag used consistently in Task 2 and already present on the `APIKey` fields by the time Task 4 edits `ServerConfig` (Task 4's edit preserves the existing tags).
- `ctxRequestIDKey{}` and `RequestIDFromContext` are existing types/functions from `internal/api/request_id.go`; Task 5's tests import them correctly.

### Dependency ordering

- Task 1 is purely additive to config loading (adds validation, tightens unmarshal). Independent of everything else.
- Task 2 adds a struct tag to fields that Task 1 also reads — harmless because tags are metadata.
- Tasks 3 and 4 build on each other: Task 4 edits the file Task 3 created. Must land in order.
- Task 5 is independent of Tasks 1–4.

Recommended sequence: 1 → 2 → 3 → 4 → 5. Tasks 1, 2, 5 could theoretically ship in parallel, but subagent-driven-development runs sequentially so that's moot.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-23-block2-security-plan.md`.**

Two execution options:

1. **Subagent-Driven (recommended)** — fresh subagent per task, spec + code-quality review between each, same cadence as Block 1.
2. **Inline Execution** — this session drives all five tasks with periodic checkpoints.

Which approach?

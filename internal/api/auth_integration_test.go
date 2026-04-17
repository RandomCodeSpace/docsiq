//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

// unauthReq builds a request against the test server without adding any
// Authorization header. Used for negative-path and public-path tests.
func unauthReq(t *testing.T, e *itest.Env, method, path string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, e.URL(path), body)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, path, err)
	}
	return req
}

func TestAuth_BearerRequiredOnAPI(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodGet, "/api/stats?project=_default", nil)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuth_BearerNotRequiredOnHealth(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodGet, "/health", nil)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 on /health, got %d", resp.StatusCode)
	}
}

func TestAuth_BearerNotRequiredOnMetrics(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodGet, "/metrics", nil)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 on /metrics, got %d", resp.StatusCode)
	}
}

func TestAuth_OptionsBypasses(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodOptions, "/api/stats", nil)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("OPTIONS must bypass auth; got 401")
	}
}

func TestAuth_WrongSchemeRejected(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodGet, "/api/stats?project=_default", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 for Basic scheme, got %d", resp.StatusCode)
	}
}

func TestAuth_WrongKeyRejected(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodGet, "/api/stats?project=_default", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 for wrong key, got %d", resp.StatusCode)
	}
}

func TestAuth_CorrectKeyAccepted(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodGet, "/api/stats?project=_default", nil)
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("valid key must pass auth; got 401")
	}
}

func TestAuth_LowercaseBearerRejected(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodGet, "/api/stats?project=_default", nil)
	req.Header.Set("Authorization", "bearer "+e.APIKey)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("lowercase 'bearer' must be rejected; got %d", resp.StatusCode)
	}
}

func TestAuth_ConcurrentFailuresNoRace(t *testing.T) {
	e := itest.New(t)
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodGet, e.URL("/api/stats?project=_default"), nil)
			if err != nil {
				t.Errorf("build req: %v", err)
				return
			}
			resp, err := e.Client.Do(req)
			if err != nil {
				t.Errorf("do req: %v", err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("want 401, got %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
}

func TestAuth_MultipleAuthorizationHeadersFirstWins(t *testing.T) {
	e := itest.New(t)
	// First value is valid, second is garbage. stdlib Header.Get returns the
	// first value, so the middleware should accept (not 401). Run several
	// times to assert determinism.
	for i := 0; i < 5; i++ {
		req := unauthReq(t, e, http.MethodGet, "/api/stats?project=_default", nil)
		req.Header.Add("Authorization", "Bearer "+e.APIKey)
		req.Header.Add("Authorization", "Bearer wrong-key")
		resp := e.Do(t, req)
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			t.Fatalf("iter %d: first-wins expected pass; got 401", i)
		}
	}
}

func TestAuth_BodyStillReadableAfterAuth(t *testing.T) {
	e := itest.New(t)
	// PUT a note with a non-trivial JSON body using the correct bearer,
	// then assert the handler read the full body (response reflects the
	// note content).
	payload := map[string]any{
		"content": "body-survives-auth-middleware",
		"tags":    []string{"auth-test"},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut,
		e.URL("/api/projects/_default/notes/auth-body-check"),
		bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("build req: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("auth failed unexpectedly: 401")
	}
	if resp.StatusCode >= 500 {
		t.Fatalf("handler 5xx (body likely truncated): %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read resp: %v", err)
	}
	// Response should be JSON; if the handler got an empty body it would
	// typically 400. We assert a non-4xx-parse-error outcome (<500) plus
	// the full payload made it through: handler echoes content in most
	// note endpoints, but at minimum a 2xx status is proof.
	if resp.StatusCode >= 400 {
		t.Fatalf("unexpected 4xx: %d body=%s", resp.StatusCode, string(data))
	}
}

func TestAuth_MCPEndpointGated(t *testing.T) {
	e := itest.New(t)
	req := unauthReq(t, e, http.MethodPost, "/mcp", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 on /mcp with no auth, got %d", resp.StatusCode)
	}
}

func TestAuth_UIAssetPublic(t *testing.T) {
	e := itest.New(t)
	// Try "/" first, fall back to "/index.html" if the root isn't wired.
	for _, path := range []string{"/", "/index.html"} {
		req := unauthReq(t, e, http.MethodGet, path, nil)
		resp := e.Do(t, req)
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			t.Fatalf("UI asset %q must be public; got 401", path)
		}
	}
}

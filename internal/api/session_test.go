package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSession_PostExchangesBearerForCookie(t *testing.T) {
	t.Parallel()
	h := newSessionHandler("s3cret")
	req := httptest.NewRequest(http.MethodPost, "/api/session", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rr.Code)
	}
	setCookie := rr.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, sessionCookieName+"=") {
		t.Fatalf("missing session cookie: %q", setCookie)
	}
	for _, attr := range []string{"HttpOnly", "Secure", "SameSite=Strict", "Path=/"} {
		if !strings.Contains(setCookie, attr) {
			t.Fatalf("cookie missing %s: %q", attr, setCookie)
		}
	}
}

func TestSession_PostRejectsBadKey(t *testing.T) {
	t.Parallel()
	h := newSessionHandler("s3cret")
	req := httptest.NewRequest(http.MethodPost, "/api/session", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
	if rr.Header().Get("Set-Cookie") != "" {
		t.Fatal("cookie must not be set on failure")
	}
}

func TestSession_DeleteClearsCookie(t *testing.T) {
	t.Parallel()
	h := newSessionDeleteHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rr.Code)
	}
	setCookie := rr.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "Max-Age=0") {
		t.Fatalf("cookie should be cleared (Max-Age=0); got %q", setCookie)
	}
}

package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// sessionCookieName is the name of the httpOnly cookie that carries the
// bearer token after a successful POST /api/session exchange. The value
// is identical to cfg.Server.APIKey — we do not (yet) rotate or sign it;
// the cookie is a transport-hardening layer, not a session store.
const sessionCookieName = "docsiq_session"

// newSessionHandler returns the POST /api/session handler. Accepts an
// Authorization: Bearer <apiKey> header and on match sets the session
// cookie. 401 on any other shape.
func newSessionHandler(apiKey string) http.HandlerFunc {
	keyBytes := []byte(apiKey)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, DELETE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw := strings.TrimSpace(r.Header.Get("Authorization"))
		const prefix = "Bearer "
		if !strings.HasPrefix(raw, prefix) {
			writeJSON401(w)
			return
		}
		token := raw[len(prefix):]
		if apiKey == "" || subtle.ConstantTimeCompare([]byte(token), keyBytes) != 1 {
			slog.Warn("🔒 session: auth failure", "remote_addr", r.RemoteAddr, "reason", "wrong_key")
			writeJSON401(w)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    apiKey,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   86400 * 30, // 30 days
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// newSessionDeleteHandler returns the DELETE /api/session handler,
// which clears the session cookie (client-initiated logout).
func newSessionDeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.Header().Set("Allow", "POST, DELETE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

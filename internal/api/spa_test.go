package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestSpaHandler_DoesNotInjectAPIKey(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte(`<html><head></head><body></body></html>`),
		},
	}
	cfg := &config.Config{}
	cfg.Server.APIKey = "s3cret"

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	spaHandler(fsys, cfg).ServeHTTP(rr, req)

	body, _ := io.ReadAll(rr.Body)
	if bytes.Contains(body, []byte("docsiq-api-key")) {
		t.Fatalf("served HTML still contains api-key meta tag:\n%s", body)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200; got %d", rr.Code)
	}
}

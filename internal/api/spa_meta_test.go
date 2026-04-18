package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/ui"
)

func TestSPA_InjectsMetaWhenAPIKeySet(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.APIKey = "secret-key-abc"
	h := spaHandler(ui.Assets, cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `name="docsiq-api-key"`) {
		t.Fatalf("expected meta tag, body:\n%s", body)
	}
	if !strings.Contains(string(body), `content="secret-key-abc"`) {
		t.Fatalf("expected API key in content attr, body:\n%s", body)
	}
}

func TestSPA_OmitsMetaWhenAPIKeyUnset(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.APIKey = ""
	h := spaHandler(ui.Assets, cfg)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `name="docsiq-api-key"`) {
		t.Fatalf("meta tag should not be present when APIKey empty")
	}
}

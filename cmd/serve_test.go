package cmd

import (
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestValidateServeSecurity_RefusesNonLoopbackWithEmptyKey(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080
	cfg.Server.APIKey = ""

	err := validateServeSecurity(cfg)
	if err == nil {
		t.Fatal("expected error for empty api_key on non-loopback bind; got nil")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("error should mention api_key; got %v", err)
	}
}

func TestValidateServeSecurity_AllowsLoopbackWithEmptyKey(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		cfg := &config.Config{}
		cfg.Server.Host = host
		cfg.Server.APIKey = ""
		if err := validateServeSecurity(cfg); err != nil {
			t.Fatalf("host=%s: expected nil; got %v", host, err)
		}
	}
}

func TestValidateServeSecurity_AllowsNonLoopbackWithKey(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.APIKey = "s3cret"
	if err := validateServeSecurity(cfg); err != nil {
		t.Fatalf("expected nil; got %v", err)
	}
}

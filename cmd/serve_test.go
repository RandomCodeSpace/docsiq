package cmd

import (
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestValidateServeSecurity_RefusesNonLoopbackWithEmptyKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		host        string
		mustContain []string
	}{
		{
			name:        "empty host binds all interfaces",
			host:        "",
			mustContain: []string{"binds all interfaces", "DOCSIQ_SERVER_API_KEY"},
		},
		{
			name:        "0.0.0.0 is wildcard",
			host:        "0.0.0.0",
			mustContain: []string{"api_key", "DOCSIQ_SERVER_API_KEY"},
		},
		{
			name:        "public IPv4",
			host:        "10.0.0.5",
			mustContain: []string{"api_key", "DOCSIQ_SERVER_API_KEY"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{}
			cfg.Server.Host = tc.host
			cfg.Server.Port = 8080
			cfg.Server.APIKey = ""

			err := validateServeSecurity(cfg)
			if err == nil {
				t.Fatalf("host=%q: expected error for empty api_key on non-loopback bind; got nil", tc.host)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("host=%q: error should contain %q; got %v", tc.host, want, err)
				}
			}
		})
	}
}

func TestValidateServeSecurity_AllowsLoopbackWithEmptyKey(t *testing.T) {
	t.Parallel()
	hosts := []string{
		"127.0.0.1",
		"localhost",
		"::1",
		"[::1]",            // bracketed IPv6
		"127.0.0.2",        // other address in 127.0.0.0/8
		"::ffff:127.0.0.1", // IPv6-mapped IPv4
	}
	for _, host := range hosts {
		host := host
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{}
			cfg.Server.Host = host
			cfg.Server.APIKey = ""
			if err := validateServeSecurity(cfg); err != nil {
				t.Fatalf("host=%s: expected nil; got %v", host, err)
			}
		})
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

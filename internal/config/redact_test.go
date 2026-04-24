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

package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// Claim is a factual covariate extracted from text.
type Claim struct {
	EntityName string `json:"entity_name"`
	Claim      string `json:"claim"`
	Status     string `json:"status"` // CONFIRMED | REFUTED | SPECULATIVE
}

const claimPrompt = `You are an expert at extracting factual claims from text. Extract factual claims about entities.

Return ONLY valid JSON (no markdown):
{
  "claims": [
    {"entity_name": "...", "claim": "...", "status": "CONFIRMED|REFUTED|SPECULATIVE"}
  ]
}

Rules:
- claims must be specific, verifiable statements
- entity_name should match a known entity in the text
- status: CONFIRMED (stated as fact), REFUTED (contradicted), SPECULATIVE (uncertain/hedged)
- extract up to 10 claims

TEXT:
%s`

// ExtractClaims extracts factual claims from chunks.
func ExtractClaims(ctx context.Context, provider llm.Provider, chunks []string) ([]Claim, error) {
	combined := strings.Join(chunks, "\n\n---\n\n")
	if len(combined) > 6000 {
		combined = combined[:6000]
	}

	prompt := fmt.Sprintf(claimPrompt, combined)
	resp, err := provider.Complete(ctx, prompt, llm.WithJSONMode(), llm.WithMaxTokens(1024), llm.WithTemperature(0.0))
	if err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	resp = stripCodeFences(resp)

	var result struct {
		Claims []Claim `json:"claims"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("parse claims JSON: %w\nresponse: %s", err, resp)
	}
	return result.Claims, nil
}


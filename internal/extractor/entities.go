package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/llm"
)

// Entity extracted from document text.
type Entity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Relationship between two entities.
type Relationship struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Predicate   string  `json:"predicate"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}

// ExtractionResult holds entities and relationships for a chunk.
type ExtractionResult struct {
	Entities      []Entity       `json:"entities"`
	Relationships []Relationship `json:"relationships"`
}

const entityPrompt = `You are an expert knowledge graph extractor. Extract entities and relationships from the text below.

Return ONLY valid JSON in this exact format (no markdown, no explanation):
{
  "entities": [
    {"name": "...", "type": "Person|Organization|Concept|Location|Event|Technology|Other", "description": "..."}
  ],
  "relationships": [
    {"source": "entity name", "target": "entity name", "predicate": "relation", "description": "...", "weight": 1.0}
  ]
}

Rules:
- entity names must be exact strings (used as keys)
- relationship source/target must match an entity name exactly
- weight is 0.0-1.0 (confidence/importance)
- extract 3-10 entities and up to 15 relationships per chunk

TEXT:
%s`

// ExtractEntities calls the LLM to extract entities and relationships from chunks.
func ExtractEntities(ctx context.Context, provider llm.Provider, chunks []string) (*ExtractionResult, error) {
	combined := strings.Join(chunks, "\n\n---\n\n")
	if len(combined) > 8000 {
		combined = combined[:8000]
	}

	prompt := fmt.Sprintf(entityPrompt, combined)
	resp, err := provider.Complete(ctx, prompt, llm.WithJSONMode(), llm.WithMaxTokens(2048), llm.WithTemperature(0.0))
	if err != nil {
		return nil, fmt.Errorf("extract entities: %w", err)
	}

	// Strip markdown code fences if present
	resp = stripCodeFences(resp)

	var result ExtractionResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("parse entity JSON: %w\nresponse: %s", err, resp)
	}
	return &result, nil
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) == 2 {
			s = lines[1]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

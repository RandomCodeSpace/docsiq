package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
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

const entityPrompt = `You are an expert knowledge graph analyst. Your task is to extract a comprehensive knowledge graph from the text below.

Extract ALL significant entities and the relationships between them. Be thorough — look for both explicitly stated and implied connections.

Return ONLY valid JSON in this exact format (no markdown, no explanation):
{
  "entities": [
    {"name": "...", "type": "Person|Organization|Concept|Location|Event|Technology|Document|Metric|Process|Other", "description": "..."}
  ],
  "relationships": [
    {"source": "entity name", "target": "entity name", "predicate": "relation", "description": "...", "weight": 1.0}
  ]
}

Rules:
- Entity names must be exact, canonical strings (used as graph keys). Use full proper names (e.g. "Microsoft Corporation" not "Microsoft").
- Entity types: Person, Organization, Concept, Location, Event, Technology, Document, Metric, Process, Other
- Relationship source/target must match an entity name exactly
- Weight indicates confidence: 1.0 = explicitly stated, 0.7 = strongly implied, 0.4 = weakly implied
- Extract both explicit relationships ("A acquired B") and implicit ones ("The Q3 report shows increased revenue" implies report→revenue relationship)
- Extract 5-15 entities and up to 20 relationships per chunk

Example input: "OpenAI released GPT-4 in March 2023, which significantly improved reasoning capabilities over GPT-3.5."
Example output:
{
  "entities": [
    {"name": "OpenAI", "type": "Organization", "description": "AI research company that develops GPT models"},
    {"name": "GPT-4", "type": "Technology", "description": "Large language model released in March 2023 with improved reasoning"},
    {"name": "GPT-3.5", "type": "Technology", "description": "Previous generation large language model by OpenAI"},
    {"name": "March 2023", "type": "Event", "description": "Release date of GPT-4"}
  ],
  "relationships": [
    {"source": "OpenAI", "target": "GPT-4", "predicate": "released", "description": "OpenAI released GPT-4", "weight": 1.0},
    {"source": "GPT-4", "target": "March 2023", "predicate": "released_on", "description": "GPT-4 was released in March 2023", "weight": 1.0},
    {"source": "GPT-4", "target": "GPT-3.5", "predicate": "improves_upon", "description": "GPT-4 significantly improved reasoning capabilities over GPT-3.5", "weight": 1.0},
    {"source": "OpenAI", "target": "GPT-3.5", "predicate": "developed", "description": "OpenAI developed GPT-3.5", "weight": 0.7}
  ]
}

TEXT:
%s`

const gleanPrompt = `You previously extracted entities and relationships from a text. Review the text again carefully — many entities and relationships were missed in the first pass.

Previously extracted entity names: %s

Extract ONLY the additional entities and relationships NOT already listed above.
Return the same JSON format. If truly nothing was missed, return {"entities":[],"relationships":[]}.

Return ONLY valid JSON (no markdown, no explanation):
{
  "entities": [
    {"name": "...", "type": "Person|Organization|Concept|Location|Event|Technology|Document|Metric|Process|Other", "description": "..."}
  ],
  "relationships": [
    {"source": "entity name", "target": "entity name", "predicate": "relation", "description": "...", "weight": 1.0}
  ]
}

TEXT:
%s`

// ExtractOption configures entity extraction.
type ExtractOption func(*extractOptions)

type extractOptions struct {
	maxGleanings int
}

// WithMaxGleanings sets the number of gleaning passes (default: 1).
func WithMaxGleanings(n int) ExtractOption {
	return func(o *extractOptions) { o.maxGleanings = n }
}

func applyExtractOptions(opts []ExtractOption) *extractOptions {
	o := &extractOptions{maxGleanings: 1}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// ExtractEntities calls the LLM to extract entities and relationships from chunks,
// with optional gleaning passes to catch missed entities (inspired by Microsoft GraphRAG).
func ExtractEntities(ctx context.Context, provider llm.Provider, chunks []string, opts ...ExtractOption) (*ExtractionResult, error) {
	o := applyExtractOptions(opts)

	combined := strings.Join(chunks, "\n\n---\n\n")
	if len(combined) > 8000 {
		combined = combined[:8000]
	}

	// Initial extraction
	prompt := fmt.Sprintf(entityPrompt, combined)
	resp, err := provider.Complete(ctx, prompt, llm.WithJSONMode(), llm.WithMaxTokens(2048), llm.WithTemperature(0.0))
	if err != nil {
		return nil, fmt.Errorf("extract entities: %w", err)
	}

	resp = stripCodeFences(resp)
	var result ExtractionResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("parse entity JSON: %w\nresponse: %s", err, resp)
	}

	// Gleaning passes: ask the LLM to extract entities it missed
	for i := 0; i < o.maxGleanings; i++ {
		prevNames := collectEntityNames(&result)
		if len(prevNames) == 0 {
			break
		}

		glean := fmt.Sprintf(gleanPrompt, strings.Join(prevNames, ", "), combined)
		gleanResp, err := provider.Complete(ctx, glean, llm.WithJSONMode(), llm.WithMaxTokens(2048), llm.WithTemperature(0.0))
		if err != nil {
			break // Gleaning failure is non-fatal
		}

		gleanResp = stripCodeFences(gleanResp)
		var additional ExtractionResult
		if err := json.Unmarshal([]byte(gleanResp), &additional); err != nil {
			slog.Warn("⚠️ gleaning JSON parse failed", "pass", i+1, "err", err)
			break
		}

		if len(additional.Entities) == 0 && len(additional.Relationships) == 0 {
			break // Nothing new found
		}

		result = mergeResults(&result, &additional)
	}

	return &result, nil
}

// collectEntityNames returns all entity names from a result.
func collectEntityNames(r *ExtractionResult) []string {
	names := make([]string, 0, len(r.Entities))
	for _, e := range r.Entities {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	return names
}

// mergeResults combines two extraction results, deduplicating entities by
// normalized (lowercased) name and relationships by (source, target, predicate).
func mergeResults(base, additional *ExtractionResult) ExtractionResult {
	seenEntities := make(map[string]bool, len(base.Entities))
	for _, e := range base.Entities {
		seenEntities[strings.ToLower(e.Name)] = true
	}

	type relKey struct{ src, tgt, pred string }
	seenRels := make(map[relKey]bool, len(base.Relationships))
	for _, r := range base.Relationships {
		key := relKey{strings.ToLower(r.Source), strings.ToLower(r.Target), strings.ToLower(r.Predicate)}
		seenRels[key] = true
	}

	merged := ExtractionResult{
		Entities:      append([]Entity{}, base.Entities...),
		Relationships: append([]Relationship{}, base.Relationships...),
	}

	for _, e := range additional.Entities {
		norm := strings.ToLower(e.Name)
		if norm != "" && !seenEntities[norm] {
			seenEntities[norm] = true
			merged.Entities = append(merged.Entities, e)
		}
	}
	for _, r := range additional.Relationships {
		key := relKey{strings.ToLower(r.Source), strings.ToLower(r.Target), strings.ToLower(r.Predicate)}
		if !seenRels[key] {
			seenRels[key] = true
			merged.Relationships = append(merged.Relationships, r)
		}
	}

	return merged
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

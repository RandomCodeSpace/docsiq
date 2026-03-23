package community

import (
	"context"
	"fmt"
	"strings"

	"github.com/RandomCodeSpace/docscontext/internal/llm"
)

// CommunityReport holds LLM-generated community metadata.
type CommunityReport struct {
	Title   string
	Summary string
}

const communityPrompt = `You are a knowledge graph analyst. Summarize the following community of entities and their relationships into a concise report.

Community entities:
%s

Relationships:
%s

Provide:
1. A short title (5-10 words) describing this community's theme
2. A 2-4 sentence summary explaining what this community represents, who the key entities are, and what connects them.

Format your response as:
TITLE: <title here>
SUMMARY: <summary here>`

// Summarize generates a community report using the LLM.
func Summarize(ctx context.Context, provider llm.Provider, entityDescriptions []string, relDescriptions []string) (*CommunityReport, error) {
	entities := strings.Join(entityDescriptions, "\n")
	rels := strings.Join(relDescriptions, "\n")

	if len(entities) > 3000 {
		entities = entities[:3000]
	}
	if len(rels) > 2000 {
		rels = rels[:2000]
	}

	prompt := fmt.Sprintf(communityPrompt, entities, rels)
	resp, err := provider.Complete(ctx, prompt, llm.WithMaxTokens(512), llm.WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("community summarize: %w", err)
	}

	report := parseCommunityReport(resp)
	return report, nil
}

// parseCommunityReport extracts TITLE: and SUMMARY: from the LLM response.
// The summary may span multiple lines — everything after the SUMMARY: prefix
// until the end (or the next known prefix) is captured.
func parseCommunityReport(resp string) *CommunityReport {
	report := &CommunityReport{}
	lines := strings.Split(resp, "\n")
	var summaryLines []string
	inSummary := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "TITLE:"):
			inSummary = false
			report.Title = strings.TrimSpace(trimmed[len("TITLE:"):])
		case strings.HasPrefix(upper, "SUMMARY:"):
			inSummary = true
			first := strings.TrimSpace(trimmed[len("SUMMARY:"):])
			if first != "" {
				summaryLines = append(summaryLines, first)
			}
		default:
			if inSummary && trimmed != "" {
				summaryLines = append(summaryLines, trimmed)
			}
		}
	}

	if len(summaryLines) > 0 {
		report.Summary = strings.Join(summaryLines, " ")
	}
	if report.Title == "" {
		report.Title = "Community"
	}
	if report.Summary == "" {
		report.Summary = resp
	}
	return report
}


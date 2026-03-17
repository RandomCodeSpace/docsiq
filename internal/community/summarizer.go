package community

import (
	"context"
	"fmt"
	"strings"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/llm"
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

	report := &CommunityReport{}
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TITLE:") {
			report.Title = strings.TrimSpace(strings.TrimPrefix(line, "TITLE:"))
		} else if strings.HasPrefix(line, "SUMMARY:") {
			report.Summary = strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
		}
	}
	if report.Title == "" {
		report.Title = "Community"
	}
	if report.Summary == "" {
		report.Summary = resp
	}
	return report, nil
}

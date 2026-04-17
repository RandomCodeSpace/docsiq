package search

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// GlobalSearchResult aggregates community summaries.
type GlobalSearchResult struct {
	Answer      string
	Communities []*store.Community
}

const globalSearchPrompt = `You are a knowledge graph analyst. Answer the following question using the community summaries provided.

Question: %s

Community Summaries:
%s

Provide a comprehensive answer that synthesizes information from across the communities. Be specific and cite which communities support your answer.`

// GlobalSearch performs community summary aggregation (GraphRAG global search).
func GlobalSearch(ctx context.Context, st *store.Store, emb *embedder.Embedder, prov llm.Provider, query string, communityLevel int) (*GlobalSearchResult, error) {
	// Embed query
	qVec, err := emb.EmbedOne(ctx, query)
	if err != nil {
		return nil, err
	}

	// Load communities at requested level
	communities, err := st.ListCommunities(ctx, communityLevel)
	if err != nil {
		return nil, err
	}
	if len(communities) == 0 {
		// Fall back to all communities
		communities, err = st.AllCommunities(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Rank communities by vector similarity
	type commScore struct {
		c     *store.Community
		score float32
	}
	var scored []commScore
	for _, c := range communities {
		if c.Vector != nil && len(qVec) > 0 {
			sc := store.CosineSimilarity(qVec, c.Vector)
			scored = append(scored, commScore{c, sc})
		} else if c.Summary != "" {
			scored = append(scored, commScore{c, 0.5})
		}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })

	// Take top 10 communities
	topN := 10
	if topN > len(scored) {
		topN = len(scored)
	}

	var summaryParts []string
	var topCommunities []*store.Community
	for i, cs := range scored[:topN] {
		if cs.c.Summary == "" {
			continue
		}
		summaryParts = append(summaryParts, fmt.Sprintf("[%d] %s: %s", i+1, cs.c.Title, cs.c.Summary))
		topCommunities = append(topCommunities, cs.c)
	}

	if len(summaryParts) == 0 {
		return &GlobalSearchResult{
			Answer:      "No community summaries available. Run `docsiq index --finalize` first.",
			Communities: nil,
		}, nil
	}

	// Ask LLM to synthesize
	prompt := fmt.Sprintf(globalSearchPrompt, query, strings.Join(summaryParts, "\n\n"))
	answer, err := prov.Complete(ctx, prompt, llm.WithMaxTokens(1024), llm.WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("global search LLM: %w", err)
	}

	return &GlobalSearchResult{
		Answer:      answer,
		Communities: topCommunities,
	}, nil
}



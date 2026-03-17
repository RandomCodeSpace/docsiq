package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/amit/docsgraphcontext/internal/chunker"
	"github.com/amit/docsgraphcontext/internal/community"
	"github.com/amit/docsgraphcontext/internal/config"
	"github.com/amit/docsgraphcontext/internal/embedder"
	"github.com/amit/docsgraphcontext/internal/extractor"
	"github.com/amit/docsgraphcontext/internal/llm"
	"github.com/amit/docsgraphcontext/internal/loader"
	"github.com/amit/docsgraphcontext/internal/store"
	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
)

// ProgressEvent sent over progress channel.
type ProgressEvent struct {
	Phase   string
	Message string
	Done    bool
	Error   error
}

// Pipeline orchestrates the 5-phase GraphRAG pipeline.
type Pipeline struct {
	store    *store.Store
	provider llm.Provider
	embedder *embedder.Embedder
	chunker  *chunker.Chunker
	cfg      *config.Config
}

// New creates a new Pipeline.
func New(st *store.Store, prov llm.Provider, cfg *config.Config) *Pipeline {
	return &Pipeline{
		store:    st,
		provider: prov,
		embedder: embedder.New(prov, cfg.Indexing.BatchSize),
		chunker:  chunker.New(cfg.Indexing.ChunkSize, cfg.Indexing.ChunkOverlap),
		cfg:      cfg,
	}
}

// IndexOptions controls indexing behavior.
type IndexOptions struct {
	Force    bool
	Workers  int
	Verbose  bool
	Progress chan<- ProgressEvent
}

// IndexPath indexes a file or directory.
func (p *Pipeline) IndexPath(ctx context.Context, path string, opts IndexOptions) error {
	workers := opts.Workers
	if workers <= 0 {
		workers = p.cfg.Indexing.Workers
	}

	files, err := collectFiles(path)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no supported files found in %s", path)
	}

	bar := progressbar.NewOptions(len(files),
		progressbar.OptionSetDescription("Indexing"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWriter(os.Stderr),
	)

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	for _, f := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(filePath string) {
			defer wg.Done()
			defer func() { <-sem }()
			defer bar.Add(1)

			if err := p.indexFile(ctx, filePath, opts); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: %v", filePath, err))
				mu.Unlock()
			}
		}(f)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("indexing errors:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// indexFile processes a single file through Phases 1-2.
func (p *Pipeline) indexFile(ctx context.Context, path string, opts IndexOptions) error {
	// Hash for dedup
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])

	// Incremental versioning: look up the latest version at this path.
	existing, err := p.store.GetDocumentByPath(ctx, path)
	if err != nil {
		return err
	}
	if existing != nil && existing.FileHash == hash && !opts.Force {
		return nil // unchanged — skip
	}

	// Determine version number and canonical ID for this (possibly new) version.
	nextVersion := 1
	canonicalID := ""
	if existing != nil {
		nextVersion = existing.Version + 1
		canonicalID = existing.CanonicalOrID()
		// Mark the previous version as superseded (keep all its data in the graph).
		if err := p.store.SupersedeDocument(ctx, existing.ID); err != nil {
			return fmt.Errorf("supersede old version: %w", err)
		}
	}

	// Phase 1a: Load
	doc, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}

	chunks := p.chunker.Split(doc.Content)
	if len(chunks) == 0 {
		return nil
	}

	docID := uuid.New().String()
	if err := p.store.UpsertDocument(ctx, &store.Document{
		ID:          docID,
		Path:        path,
		Title:       doc.Title,
		DocType:     doc.DocType,
		FileHash:    hash,
		Version:     nextVersion,
		CanonicalID: canonicalID,
		IsLatest:    true,
	}); err != nil {
		return fmt.Errorf("upsert doc: %w", err)
	}

	// Phase 1b: Build chunk objects
	texts := make([]string, len(chunks))
	chunkIDs := make([]string, len(chunks))
	storeChunks := make([]*store.Chunk, len(chunks))
	for i, c := range chunks {
		id := uuid.New().String()
		chunkIDs[i] = id
		texts[i] = c.Content
		storeChunks[i] = &store.Chunk{
			ID:         id,
			DocID:      docID,
			ChunkIndex: c.Index,
			Content:    c.Content,
			TokenCount: c.Tokens,
		}
	}

	// Phase 1c: Batch insert chunks
	if err := p.store.BatchInsertChunks(ctx, storeChunks); err != nil {
		return fmt.Errorf("batch insert chunks: %w", err)
	}

	// Phase 1d: Embed chunks (concurrent batches internally)
	vecs, err := p.embedder.EmbedTexts(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}

	// Phase 1e: Batch store embeddings
	if err := p.store.BatchUpsertEmbeddings(ctx, p.provider.ModelID(), chunkIDs, vecs); err != nil {
		return fmt.Errorf("batch store embeddings: %w", err)
	}

	// Phase 2: Run graph extraction, claims, and structured doc in parallel
	var (
		graphErr     error
		claimsErr    error
		structureErr error
		wg2          sync.WaitGroup
	)

	if p.cfg.Indexing.ExtractGraph {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			graphErr = p.extractGraph(ctx, docID, texts)
		}()
	}

	if p.cfg.Indexing.ExtractClaims {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			claimsErr = p.extractClaims(ctx, docID, texts)
		}()
	}

	wg2.Add(1)
	go func() {
		defer wg2.Done()
		structureErr = p.structureDocument(ctx, docID, doc.Content)
	}()

	wg2.Wait()

	// Log non-fatal errors
	for _, e := range []error{graphErr, claimsErr, structureErr} {
		if e != nil && opts.Verbose {
			fmt.Fprintf(os.Stderr, "  warning [%s]: %v\n", path, e)
		}
	}

	return nil
}

// extractGraph runs entity/relationship extraction over chunk text batches.
// It parallelizes batch LLM calls and uses a single batch DB write per document.
func (p *Pipeline) extractGraph(ctx context.Context, docID string, texts []string) error {
	const batchChunks = 3
	type batchResult struct {
		result *extractor.ExtractionResult
		err    error
	}

	numBatches := (len(texts) + batchChunks - 1) / batchChunks
	results := make([]batchResult, numBatches)

	// Parallel LLM extraction across chunk batches
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for bi := 0; bi < numBatches; bi++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			start := idx * batchChunks
			end := start + batchChunks
			if end > len(texts) {
				end = len(texts)
			}
			res, err := extractor.ExtractEntities(ctx, p.provider, texts[start:end])
			results[idx] = batchResult{res, err}
		}(bi)
	}
	wg.Wait()

	// Collect all extracted names for a single bulk DB lookup
	nameSet := map[string]struct{}{}
	for _, br := range results {
		if br.err != nil || br.result == nil {
			continue
		}
		for _, e := range br.result.Entities {
			if e.Name != "" {
				nameSet[e.Name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}

	// Single query to fetch all existing entities by name
	existingEntities, err := p.store.GetEntitiesByNames(ctx, names)
	if err != nil {
		return err
	}

	// Build entity ID map: name → ID (merge or create)
	entityIDMap := make(map[string]string, len(names))
	var toUpsert []*store.Entity

	for _, br := range results {
		if br.err != nil || br.result == nil {
			continue
		}
		for _, e := range br.result.Entities {
			if e.Name == "" {
				continue
			}
			if _, seen := entityIDMap[e.Name]; seen {
				continue
			}
			if existing, ok := existingEntities[e.Name]; ok {
				entityIDMap[e.Name] = existing.ID
				// Merge description if new one is longer
				if len(e.Description) > len(existing.Description) {
					existing.Description = e.Description
					toUpsert = append(toUpsert, existing)
				}
			} else {
				eid := uuid.New().String()
				entityIDMap[e.Name] = eid
				toUpsert = append(toUpsert, &store.Entity{
					ID:          eid,
					Name:        e.Name,
					Type:        e.Type,
					Description: e.Description,
				})
			}
		}
	}

	// Batch upsert entities
	if err := p.store.BatchUpsertEntities(ctx, toUpsert); err != nil {
		return fmt.Errorf("batch upsert entities: %w", err)
	}

	// Collect all relationships
	var rels []*store.Relationship
	for _, br := range results {
		if br.err != nil || br.result == nil {
			continue
		}
		for _, r := range br.result.Relationships {
			srcID, ok1 := entityIDMap[r.Source]
			tgtID, ok2 := entityIDMap[r.Target]
			if !ok1 || !ok2 {
				continue
			}
			rels = append(rels, &store.Relationship{
				ID:          uuid.New().String(),
				SourceID:    srcID,
				TargetID:    tgtID,
				Predicate:   r.Predicate,
				Description: r.Description,
				Weight:      r.Weight,
				DocID:       docID,
			})
		}
	}

	return p.store.BatchInsertRelationships(ctx, rels)
}

// extractClaims extracts factual claims and batch-inserts them.
func (p *Pipeline) extractClaims(ctx context.Context, docID string, texts []string) error {
	limit := min(len(texts), 5)
	claimList, err := extractor.ExtractClaims(ctx, p.provider, texts[:limit])
	if err != nil {
		return err
	}

	// Lookup entity IDs in batch
	names := make([]string, 0, len(claimList))
	for _, c := range claimList {
		if c.EntityName != "" {
			names = append(names, c.EntityName)
		}
	}
	entityMap, err := p.store.GetEntitiesByNames(ctx, names)
	if err != nil {
		return err
	}

	claims := make([]*store.Claim, 0, len(claimList))
	for _, c := range claimList {
		var entityID string
		if e, ok := entityMap[c.EntityName]; ok {
			entityID = e.ID
		}
		claims = append(claims, &store.Claim{
			ID:       uuid.New().String(),
			EntityID: entityID,
			Claim:    c.Claim,
			Status:   c.Status,
			DocID:    docID,
		})
	}
	return p.store.BatchInsertClaims(ctx, claims)
}

const structurePrompt = `Analyze this document and provide a structured summary.

Return JSON:
{
  "main_topic": "...",
  "key_points": ["...", "..."],
  "document_type": "report|article|manual|specification|other",
  "sections": [{"title": "...", "summary": "..."}]
}

DOCUMENT:
%s`

func (p *Pipeline) structureDocument(ctx context.Context, docID, content string) error {
	if len(content) > 6000 {
		content = content[:6000]
	}
	prompt := fmt.Sprintf(structurePrompt, content)
	resp, err := p.provider.Complete(ctx, prompt, llm.WithJSONMode(), llm.WithMaxTokens(1024))
	if err != nil {
		return err
	}
	return p.store.UpdateDocumentStructured(ctx, docID, resp)
}

// Finalize runs Phases 3-4: community detection + parallel summaries.
func (p *Pipeline) Finalize(ctx context.Context, verbose bool) error {
	entities, err := p.store.AllEntities(ctx)
	if err != nil {
		return fmt.Errorf("load entities: %w", err)
	}
	if len(entities) == 0 {
		return fmt.Errorf("no entities to cluster")
	}

	rels, err := p.store.AllRelationships(ctx)
	if err != nil {
		return fmt.Errorf("load relationships: %w", err)
	}

	// Phase 3: Build graph + Louvain
	nodes := make([]string, len(entities))
	entityIDtoIdx := map[string]int{}
	for i, e := range entities {
		nodes[i] = e.ID
		entityIDtoIdx[e.ID] = i
	}

	edges := make([][3]any, 0, len(rels))
	for _, r := range rels {
		edges = append(edges, [3]any{r.SourceID, r.TargetID, r.Weight})
	}

	g := community.NewGraph(nodes, edges)
	levels := community.HierarchicalLouvain(g, p.cfg.Community.MaxLevels, 100)

	if err := p.store.ClearCommunities(ctx); err != nil {
		return err
	}

	communityIDMap := map[string]string{}

	// ── Phase 4: Parallel community summarization ─────────────────────────────
	type commWork struct {
		commID   string
		parentID string
		level    int
		rank     int
		entityIDs []string
		entityDescs []string
		relDescs    []string
	}

	// Collect all community work items
	var workItems []commWork
	for level, assignments := range levels {
		communityEntities := map[int][]string{}
		for nodeIdx, commNum := range assignments {
			communityEntities[commNum] = append(communityEntities[commNum], nodes[nodeIdx])
		}
		for commNum, entityIDs := range communityEntities {
			if len(entityIDs) < p.cfg.Community.MinCommunitySize {
				continue
			}
			commKey := fmt.Sprintf("%d:%d", level, commNum)
			commID, exists := communityIDMap[commKey]
			if !exists {
				commID = uuid.New().String()
				communityIDMap[commKey] = commID
			}
			var parentID string
			if level > 0 && len(entityIDs) > 0 {
				nodeIdx := entityIDtoIdx[entityIDs[0]]
				parentCommNum := levels[level-1][nodeIdx]
				parentKey := fmt.Sprintf("%d:%d", level-1, parentCommNum)
				parentID = communityIDMap[parentKey]
			}

			// Build descriptions
			entityDescs := make([]string, 0, len(entityIDs))
			for _, eid := range entityIDs {
				e, err := p.store.GetEntity(ctx, eid)
				if err != nil || e == nil {
					continue
				}
				entityDescs = append(entityDescs, fmt.Sprintf("- %s (%s): %s", e.Name, e.Type, e.Description))
			}
			relDescs := []string{}
			entitySet := map[string]bool{}
			for _, eid := range entityIDs {
				entitySet[eid] = true
			}
			for _, r := range rels {
				if entitySet[r.SourceID] && entitySet[r.TargetID] && r.Description != "" {
					relDescs = append(relDescs, "- "+r.Description)
				}
			}

			workItems = append(workItems, commWork{
				commID:      commID,
				parentID:    parentID,
				level:       level,
				rank:        len(entityIDs),
				entityIDs:   entityIDs,
				entityDescs: entityDescs,
				relDescs:    relDescs,
			})
		}
	}

	// Parallel summarization with bounded concurrency
	type commResult struct {
		work    commWork
		title   string
		summary string
		vector  []float32
	}
	results := make([]commResult, len(workItems))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for i, w := range workItems {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, work commWork) {
			defer wg.Done()
			defer func() { <-sem }()

			res := commResult{work: work}
			report, err := community.Summarize(ctx, p.provider, work.entityDescs, work.relDescs)
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "  community summary error: %v\n", err)
				}
				res.title = fmt.Sprintf("Community %d (Level %d)", idx, work.level)
				res.summary = fmt.Sprintf("Contains %d entities.", work.rank)
			} else {
				res.title = report.Title
				res.summary = report.Summary
			}

			// Embed summary
			if res.summary != "" {
				vec, err := p.embedder.EmbedOne(ctx, res.summary)
				if err == nil {
					res.vector = vec
				}
			}
			results[idx] = res
		}(i, w)
	}
	wg.Wait()

	// Sequential DB writes (SQLite single writer)
	communityAssignments := map[string]string{} // entityID → communityID
	for _, res := range results {
		if err := p.store.UpsertCommunity(ctx, &store.Community{
			ID:       res.work.commID,
			Level:    res.work.level,
			ParentID: res.work.parentID,
			Title:    res.title,
			Summary:  res.summary,
			Rank:     res.work.rank,
			Vector:   res.vector,
		}); err != nil {
			return err
		}
		if err := p.store.BatchInsertCommunityMembers(ctx, res.work.commID, res.work.entityIDs); err != nil {
			return err
		}
		for _, eid := range res.work.entityIDs {
			communityAssignments[eid] = res.work.commID
		}
	}

	// Batch update entity community assignments
	if err := p.store.BatchUpdateEntityCommunities(ctx, communityAssignments); err != nil {
		return err
	}

	// Batch update entity ranks (degree centrality)
	degreeCounts := map[string]int{}
	for _, r := range rels {
		degreeCounts[r.SourceID]++
		degreeCounts[r.TargetID]++
	}
	if err := p.store.BatchUpdateEntityRanks(ctx, degreeCounts); err != nil {
		return err
	}

	// Embed entity descriptions in parallel, then batch upsert
	toEmbed := make([]*store.Entity, 0, len(entities))
	for _, e := range entities {
		if e.Description != "" {
			toEmbed = append(toEmbed, e)
		}
	}
	if len(toEmbed) > 0 {
		descTexts := make([]string, len(toEmbed))
		for i, e := range toEmbed {
			descTexts[i] = e.Description
		}
		vecs, err := p.embedder.EmbedTexts(ctx, descTexts)
		if err == nil {
			for i, e := range toEmbed {
				if i < len(vecs) {
					e.Vector = vecs[i]
				}
			}
			p.store.BatchUpsertEntities(ctx, toEmbed)
		}
	}

	return nil
}

// collectFiles recursively finds all supported files under path.
func collectFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{root}, nil
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".pdf", ".docx", ".txt", ".md", ".markdown", ".text":
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

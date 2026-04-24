package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/RandomCodeSpace/docsiq/internal/chunker"
	"github.com/RandomCodeSpace/docsiq/internal/community"
	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/crawler"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/extractor"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/loader"
	"github.com/RandomCodeSpace/docsiq/internal/obs"
	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
)

// timeStage is a nil-safe wrapper around obs.Pipeline.TimeStage. The
// indexer CLI does not initialise obs (obs.Init is only called from
// cmd/serve.go), so the CLI path must not blow up on a nil
// obs.Pipeline.
func timeStage(stage string, fn func() error) error {
	if obs.Pipeline == nil {
		return fn()
	}
	return obs.Pipeline.TimeStage(stage, fn)
}

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
	Force       bool
	Workers     int
	Verbose     bool
	Progress    chan<- ProgressEvent
	// Web crawl options (used by IndexURL)
	MaxPages    int
	MaxDepth    int
	SkipSitemap bool
}

// IndexPath indexes a file or directory.
func (p *Pipeline) IndexPath(ctx context.Context, path string, opts IndexOptions) error {
	return timeStage("index_path", func() error {
		return p.indexPath(ctx, path, opts)
	})
}

func (p *Pipeline) indexPath(ctx context.Context, path string, opts IndexOptions) error {
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

	slog.Info("📄 indexing files", "path", path, "count", len(files), "workers", workers)

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
				slog.Warn("⚠️ failed to index file", "path", filePath, "err", err)
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: %v", filePath, err))
				mu.Unlock()
			}
		}(f)
	}
	wg.Wait()

	if len(errs) > 0 {
		slog.Error("❌ indexing finished with errors", "failed", len(errs), "total", len(files))
		return fmt.Errorf("indexing errors:\n%s", strings.Join(errs, "\n"))
	}
	slog.Info("✅ indexing complete", "files", len(files))
	return nil
}

// IndexURL crawls a documentation website and indexes all discovered pages.
func (p *Pipeline) IndexURL(ctx context.Context, rootURL string, opts IndexOptions) error {
	return timeStage("index_url", func() error {
		return p.indexURL(ctx, rootURL, opts)
	})
}

func (p *Pipeline) indexURL(ctx context.Context, rootURL string, opts IndexOptions) error {
	workers := opts.Workers
	if workers <= 0 {
		workers = p.cfg.Indexing.Workers
	}

	slog.Info("🌐 crawling site", "url", rootURL)
	pages, err := crawler.Crawl(ctx, rootURL, crawler.Options{
		MaxPages:    opts.MaxPages,
		MaxDepth:    opts.MaxDepth,
		Concurrency: workers,
		SkipSitemap: opts.SkipSitemap,
	})
	if err != nil {
		return fmt.Errorf("crawl: %w", err)
	}
	if len(pages) == 0 {
		return fmt.Errorf("no pages found at %s", rootURL)
	}
	slog.Info("✅ crawl complete, indexing pages", "url", rootURL, "pages", len(pages))

	bar := progressbar.NewOptions(len(pages),
		progressbar.OptionSetDescription("Indexing pages"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWriter(os.Stderr),
	)

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	for _, page := range pages {
		wg.Add(1)
		sem <- struct{}{}
		go func(doc *loader.RawDocument) {
			defer wg.Done()
			defer func() { <-sem }()
			defer bar.Add(1)
			if err := p.indexRawDoc(ctx, doc, opts); err != nil {
				slog.Warn("⚠️ failed to index page", "url", doc.Path, "err", err)
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: %v", doc.Path, err))
				mu.Unlock()
			}
		}(page)
	}
	wg.Wait()

	if len(errs) > 0 {
		slog.Error("❌ web indexing finished with errors", "failed", len(errs), "total", len(pages))
		return fmt.Errorf("indexing errors:\n%s", strings.Join(errs, "\n"))
	}
	slog.Info("✅ web indexing complete", "url", rootURL, "pages", len(pages))
	return nil
}

// indexFile processes a single file through Phases 1-2.
func (p *Pipeline) indexFile(ctx context.Context, path string, opts IndexOptions) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}
	path = absPath

	// Phase-5 cheap short-circuit: if mtime matches a previously-indexed
	// snapshot, skip hashing AND reading the whole file. Only useful when
	// --force is off and the row has a non-null indexed_mtime (pre-Phase-5
	// rows fall through to the hash compare below).
	fi, statErr := os.Stat(path)
	if statErr != nil {
		return statErr
	}
	mtime := fi.ModTime().Unix()

	existing, err := p.store.GetDocumentByPath(ctx, path)
	if err != nil {
		return err
	}
	if !opts.Force && existing != nil && existing.IndexedMtime != 0 && existing.IndexedMtime == mtime {
		slog.Info("⏭️ skipping unchanged file (mtime cache hit)", "path", path)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])

	if !opts.Force && existing != nil && existing.FileHash == hash {
		// Content identical, mtime diverged (touch, restore, etc.).
		// Refresh the stored mtime so future scans can use the fast
		// path, but skip re-ingest.
		if existing.IndexedMtime != mtime {
			existing.IndexedMtime = mtime
			if err := p.store.UpsertDocument(ctx, existing); err != nil {
				slog.Warn("⚠️ mtime refresh failed", "path", path, "err", err)
			}
		}
		slog.Info("⏭️ skipping unchanged file (hash match)", "path", path)
		return nil
	}
	if existing != nil {
		slog.Info("📄 superseding existing document version", "path", path, "old_version", existing.Version)
		if err := p.store.SupersedeDocument(ctx, existing.ID); err != nil {
			return fmt.Errorf("supersede old version: %w", err)
		}
	}

	// Phase 1a: Load — skip binary files gracefully
	doc, err := loader.Load(path)
	if err != nil {
		if errors.Is(err, loader.ErrBinaryFile) {
			slog.Warn("⏭️ skipping binary file", "path", path)
			return nil
		}
		return fmt.Errorf("load: %w", err)
	}

	nextVersion, canonicalID := versionInfo(existing)

	chunks := p.chunker.Split(doc.Content)
	if len(chunks) == 0 {
		slog.Warn("❌ no chunks produced for file", "path", path)
		return fmt.Errorf("no chunks produced for file: %s", path)
	}

	slog.Debug("📄 indexing file", "path", path, "version", nextVersion, "chunks", len(chunks), "doc_type", doc.DocType)

	docID := uuid.New().String()
	if err := p.store.UpsertDocument(ctx, &store.Document{
		ID:           docID,
		Path:         path,
		Title:        doc.Title,
		DocType:      doc.DocType,
		FileHash:     hash,
		Version:      nextVersion,
		CanonicalID:  canonicalID,
		IsLatest:     true,
		IndexedMtime: mtime,
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

	if err := p.store.BatchInsertChunks(ctx, storeChunks); err != nil {
		return fmt.Errorf("batch insert chunks: %w", err)
	}

	// Phase 1c: Embed chunks
	vecs, err := p.embedder.EmbedTexts(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	slog.Debug("📊 chunks embedded", "path", path, "chunks", len(vecs))

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

	for label, e := range map[string]error{"graph": graphErr, "claims": claimsErr, "structure": structureErr} {
		if e != nil {
			slog.Warn("⚠️ extraction failed", "phase", label, "path", path, "err", e)
		}
	}

	return nil
}

// normalizeEntityName returns a canonical form for entity name matching.
// Lowercases, trims whitespace, and collapses multiple spaces.
var spaceCollapser = regexp.MustCompile(`\s+`)

func normalizeEntityName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = spaceCollapser.ReplaceAllString(name, " ")
	return name
}

// extractGraph runs entity/relationship extraction over chunk text batches.
func (p *Pipeline) extractGraph(ctx context.Context, docID string, texts []string) error {
	const batchChunks = 3
	type batchResult struct {
		result *extractor.ExtractionResult
		err    error
	}

	maxGleanings := p.cfg.Indexing.MaxGleanings
	numBatches := (len(texts) + batchChunks - 1) / batchChunks
	results := make([]batchResult, numBatches)

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
			res, err := extractor.ExtractEntities(ctx, p.provider, texts[start:end],
				extractor.WithMaxGleanings(maxGleanings))
			if err != nil {
				slog.Warn("⚠️ entity extraction batch failed", "doc_id", docID, "batch", idx, "err", err)
			}
			results[idx] = batchResult{res, err}
		}(bi)
	}
	wg.Wait()

	// Collect all extracted names for a single bulk DB lookup.
	// Use normalized names for deduplication but preserve the original
	// (first-seen) name as the canonical display name.
	type nameEntry struct {
		normalized  string
		displayName string
	}
	normalizedSet := map[string]nameEntry{} // normalized → entry
	for _, br := range results {
		if br.err != nil || br.result == nil {
			continue
		}
		for _, e := range br.result.Entities {
			if e.Name == "" {
				continue
			}
			norm := normalizeEntityName(e.Name)
			if _, exists := normalizedSet[norm]; !exists {
				normalizedSet[norm] = nameEntry{normalized: norm, displayName: e.Name}
			}
		}
	}
	names := make([]string, 0, len(normalizedSet))
	for _, entry := range normalizedSet {
		names = append(names, entry.displayName)
	}

	existingEntities, err := p.store.GetEntitiesByNames(ctx, names)
	if err != nil {
		return err
	}
	// Also build a normalized lookup for existing entities
	existingByNorm := make(map[string]*store.Entity, len(existingEntities))
	for name, ent := range existingEntities {
		existingByNorm[normalizeEntityName(name)] = ent
	}

	entityIDMap := make(map[string]string, len(normalizedSet)) // normalized name → ID
	var toUpsert []*store.Entity

	for _, br := range results {
		if br.err != nil || br.result == nil {
			continue
		}
		for _, e := range br.result.Entities {
			if e.Name == "" {
				continue
			}
			norm := normalizeEntityName(e.Name)
			if _, seen := entityIDMap[norm]; seen {
				continue
			}
			if existing, ok := existingByNorm[norm]; ok {
				entityIDMap[norm] = existing.ID
				if len(e.Description) > len(existing.Description) {
					existing.Description = e.Description
					toUpsert = append(toUpsert, existing)
				}
			} else {
				eid := uuid.New().String()
				entityIDMap[norm] = eid
				displayName := normalizedSet[norm].displayName
				toUpsert = append(toUpsert, &store.Entity{
					ID:          eid,
					Name:        displayName,
					Type:        e.Type,
					Description: e.Description,
				})
			}
		}
	}

	if err := p.store.BatchUpsertEntities(ctx, toUpsert); err != nil {
		return fmt.Errorf("batch upsert entities: %w", err)
	}

	// Collect relationships with deduplication by (sourceID, targetID, predicate).
	type relKey struct{ src, tgt, pred string }
	seenRels := map[relKey]bool{}
	var rels []*store.Relationship
	for _, br := range results {
		if br.err != nil || br.result == nil {
			continue
		}
		for _, r := range br.result.Relationships {
			srcNorm := normalizeEntityName(r.Source)
			tgtNorm := normalizeEntityName(r.Target)
			srcID, ok1 := entityIDMap[srcNorm]
			tgtID, ok2 := entityIDMap[tgtNorm]
			if !ok1 || !ok2 {
				continue
			}
			predNorm := strings.ToLower(strings.TrimSpace(r.Predicate))
			key := relKey{srcID, tgtID, predNorm}
			if seenRels[key] {
				continue
			}
			seenRels[key] = true
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

	slog.Debug("🔗 graph extraction complete", "doc_id", docID,
		"entities", len(toUpsert), "relationships", len(rels))

	return p.store.BatchInsertRelationships(ctx, rels)
}

// extractClaims extracts factual claims and batch-inserts them.
func (p *Pipeline) extractClaims(ctx context.Context, docID string, texts []string) error {
	limit := min(len(texts), 5)
	claimList, err := extractor.ExtractClaims(ctx, p.provider, texts[:limit])
	if err != nil {
		return err
	}

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

	slog.Debug("📋 claims extracted", "doc_id", docID, "claims", len(claims))
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
// If force is true, the graph fingerprint cache is ignored and communities
// are always regenerated.
func (p *Pipeline) Finalize(ctx context.Context, verbose bool, force ...bool) error {
	return timeStage("finalize", func() error {
		return p.finalize(ctx, verbose, force...)
	})
}

func (p *Pipeline) finalize(ctx context.Context, verbose bool, force ...bool) error {
	slog.Info("🧩 Phase 3: loading entities and relationships")
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

	// Check if finalization can be skipped: communities already exist and
	// entity/relationship counts haven't changed since last run.
	forceFinalize := len(force) > 0 && force[0]
	if !forceFinalize {
		existingEntities, existingRels, existingComms, fpErr := p.store.GraphFingerprint(ctx)
		if fpErr == nil && existingComms > 0 &&
			existingEntities == len(entities) && existingRels == len(rels) {
			slog.Info("⏭️ skipping finalization — graph unchanged since last run",
				"entities", len(entities), "relationships", len(rels), "communities", existingComms)
			return nil
		}
	}

	slog.Info("🧩 Phase 3: running Louvain community detection",
		"entities", len(entities), "relationships", len(rels))

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
	slog.Info("✅ Phase 3: community detection complete", "levels", len(levels))

	if err := p.store.ClearCommunities(ctx); err != nil {
		return err
	}

	communityIDMap := map[string]string{}

	type commWork struct {
		commID      string
		parentID    string
		level       int
		rank        int
		entityIDs   []string
		entityDescs []string
		relDescs    []string
	}

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

	if len(workItems) == 0 {
		slog.Warn("⚠️ all communities filtered out by min_community_size",
			"min_community_size", p.cfg.Community.MinCommunitySize,
			"hint", "lower community.min_community_size in config or index more documents")
		return nil
	}
	slog.Info("🧩 Phase 4: summarising communities", "communities", len(workItems))

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
				slog.Warn("⚠️ community summary failed", "community_idx", idx, "level", work.level, "err", err)
				res.title = fmt.Sprintf("Community %d (Level %d)", idx, work.level)
				res.summary = fmt.Sprintf("Contains %d entities.", work.rank)
			} else {
				res.title = report.Title
				res.summary = report.Summary
				slog.Debug("✅ community summarised", "idx", idx, "level", work.level, "title", report.Title)
			}

			if res.summary != "" {
				vec, err := p.embedder.EmbedOne(ctx, res.summary)
				if err == nil {
					res.vector = vec
				} else {
					slog.Warn("⚠️ community summary embedding failed", "community_idx", idx, "err", err)
				}
			}
			results[idx] = res
		}(i, w)
	}
	wg.Wait()

	slog.Info("💾 Phase 4: writing communities to store")
	communityAssignments := map[string]string{}
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

	if err := p.store.BatchUpdateEntityCommunities(ctx, communityAssignments); err != nil {
		return err
	}

	degreeCounts := map[string]int{}
	for _, r := range rels {
		degreeCounts[r.SourceID]++
		degreeCounts[r.TargetID]++
	}
	if err := p.store.BatchUpdateEntityRanks(ctx, degreeCounts); err != nil {
		return err
	}

	// Embed entity descriptions
	toEmbed := make([]*store.Entity, 0, len(entities))
	for _, e := range entities {
		if e.Description != "" {
			toEmbed = append(toEmbed, e)
		}
	}
	if len(toEmbed) > 0 {
		slog.Info("📊 embedding entity descriptions", "count", len(toEmbed))
		descTexts := make([]string, len(toEmbed))
		for i, e := range toEmbed {
			descTexts[i] = e.Description
		}
		vecs, err := p.embedder.EmbedTexts(ctx, descTexts)
		if err == nil {
			if len(vecs) != len(toEmbed) {
				slog.Warn("⚠️ embedding count mismatch", "expected", len(toEmbed), "got", len(vecs))
			}
			for i, e := range toEmbed {
				if i < len(vecs) {
					e.Vector = vecs[i]
				}
			}
			if err := p.store.BatchUpsertEntities(ctx, toEmbed); err != nil {
				return fmt.Errorf("batch upsert entity embeddings: %w", err)
			}
		} else {
			slog.Warn("⚠️ entity description embedding failed", "err", err)
		}
	}

	slog.Info("✅ Finalize complete",
		"communities", len(workItems),
		"entities_updated", len(communityAssignments))
	return nil
}

// versionInfo returns the next version number and canonical ID.
// Prune removes documents whose source file no longer exists on disk.
// Returns the number of rows deleted. Only "real" file-backed documents
// (absolute filesystem paths) are considered — web-crawled rows with
// http(s):// paths are left alone.
func (p *Pipeline) Prune(ctx context.Context) (int, error) {
	docs, err := p.store.AllDocuments(ctx)
	if err != nil {
		return 0, fmt.Errorf("load documents: %w", err)
	}
	removed := 0
	for _, d := range docs {
		if d.Path == "" {
			continue
		}
		// Skip URL-scheme paths — remote pages can't be stat()ed locally.
		if strings.HasPrefix(d.Path, "http://") || strings.HasPrefix(d.Path, "https://") {
			continue
		}
		if _, err := os.Stat(d.Path); err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("⚠️ prune stat error", "path", d.Path, "err", err)
				continue
			}
			n, err := p.store.DeleteDocument(ctx, d.ID)
			if err != nil {
				return removed, fmt.Errorf("delete %s: %w", d.ID, err)
			}
			removed += int(n)
			slog.Info("🗑️ pruned missing document", "path", d.Path, "id", d.ID)
		}
	}
	return removed, nil
}

func versionInfo(existing *store.Document) (int, string) {
	if existing == nil {
		return 1, uuid.New().String()
	}
	canonicalID := existing.CanonicalID
	if canonicalID == "" {
		canonicalID = existing.ID
	}
	return existing.Version + 1, canonicalID
}

// indexRawDoc indexes a pre-loaded RawDocument (used by IndexURL for web pages).
func (p *Pipeline) indexRawDoc(ctx context.Context, doc *loader.RawDocument, opts IndexOptions) error {
	h := sha256.Sum256([]byte(doc.Content))
	hash := hex.EncodeToString(h[:])

	existing, err := p.store.GetDocumentByPath(ctx, doc.Path)
	if err != nil {
		return err
	}
	if existing != nil && existing.FileHash == hash && !opts.Force {
		slog.Info("⏭️ skipping unchanged page", "url", doc.Path)
		return nil
	}
	if existing != nil {
		slog.Info("📄 superseding existing page version", "url", doc.Path, "old_version", existing.Version)
		if err := p.store.SupersedeDocument(ctx, existing.ID); err != nil {
			return fmt.Errorf("supersede old version: %w", err)
		}
	}

	nextVersion, canonicalID := versionInfo(existing)
	chunks := p.chunker.Split(doc.Content)
	if len(chunks) == 0 {
		slog.Warn("⏭️ no chunks produced for page, skipping", "url", doc.Path)
		return nil
	}

	slog.Debug("🌐 indexing page", "url", doc.Path, "version", nextVersion, "chunks", len(chunks))

	docID := uuid.New().String()
	if err := p.store.UpsertDocument(ctx, &store.Document{
		ID:          docID,
		Path:        doc.Path,
		Title:       doc.Title,
		DocType:     doc.DocType,
		FileHash:    hash,
		Version:     nextVersion,
		CanonicalID: canonicalID,
		IsLatest:    true,
	}); err != nil {
		return fmt.Errorf("upsert doc: %w", err)
	}

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

	if err := p.store.BatchInsertChunks(ctx, storeChunks); err != nil {
		return fmt.Errorf("batch insert chunks: %w", err)
	}

	vecs, err := p.embedder.EmbedTexts(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	slog.Debug("📊 chunks embedded", "url", doc.Path, "chunks", len(vecs))

	if err := p.store.BatchUpsertEmbeddings(ctx, p.provider.ModelID(), chunkIDs, vecs); err != nil {
		return fmt.Errorf("batch store embeddings: %w", err)
	}

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

	for label, e := range map[string]error{"graph": graphErr, "claims": claimsErr, "structure": structureErr} {
		if e != nil {
			slog.Warn("⚠️ extraction warning", "phase", label, "url", doc.Path, "err", e)
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


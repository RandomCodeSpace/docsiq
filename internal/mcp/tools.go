package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/RandomCodeSpace/docscontext/internal/search"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(s *Server) {
	// 1. search_documents
	s.mcpServer.AddTool(mcpgo.NewTool("search_documents",
		mcpgo.WithDescription("Vector similarity search over indexed document chunks"),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Search query")),
		mcpgo.WithNumber("top_k", mcpgo.Description("Number of results (default 5)")),
		mcpgo.WithString("doc_type", mcpgo.Description("Filter by doc type: pdf|docx|txt|md")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		query := stringArg(args, "query", "")
		topK := intArg(args, "top_k", 5)
		if query == "" {
			return toolError(fmt.Errorf("query required")), nil
		}
		result, err := search.LocalSearch(ctx, s.store, s.embedder, s.vecIndex, query, topK, 0)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(result.Chunks)
		return toolText(string(b)), nil
	}))

	// 2. local_search
	s.mcpServer.AddTool(mcpgo.NewTool("local_search",
		mcpgo.WithDescription("GraphRAG local search: vector similarity + graph walk"),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Search query")),
		mcpgo.WithNumber("top_k", mcpgo.Description("Number of chunk results (default 5)")),
		mcpgo.WithNumber("graph_depth", mcpgo.Description("Graph walk depth (default 2)")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		query := stringArg(args, "query", "")
		topK := intArg(args, "top_k", 5)
		depth := intArg(args, "graph_depth", 2)
		if query == "" {
			return toolError(fmt.Errorf("query required")), nil
		}
		result, err := search.LocalSearch(ctx, s.store, s.embedder, s.vecIndex, query, topK, depth)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(result)
		return toolText(string(b)), nil
	}))

	// 3. global_search
	s.mcpServer.AddTool(mcpgo.NewTool("global_search",
		mcpgo.WithDescription("GraphRAG global search: community summary aggregation with LLM synthesis"),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Search query")),
		mcpgo.WithNumber("community_level", mcpgo.Description("Community hierarchy level (default 0)")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		query := stringArg(args, "query", "")
		level := intArg(args, "community_level", 0)
		if query == "" {
			return toolError(fmt.Errorf("query required")), nil
		}
		result, err := search.GlobalSearch(ctx, s.store, s.embedder, s.provider, query, level)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(result)
		return toolText(string(b)), nil
	}))

	// 4. query_entity
	s.mcpServer.AddTool(mcpgo.NewTool("query_entity",
		mcpgo.WithDescription("Get entity details and relationships by name"),
		mcpgo.WithString("entity_name", mcpgo.Required(), mcpgo.Description("Entity name")),
		mcpgo.WithNumber("depth", mcpgo.Description("Relationship depth (default 1)")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		name := stringArg(args, "entity_name", "")
		depth := intArg(args, "depth", 1)
		if name == "" {
			return toolError(fmt.Errorf("entity_name required")), nil
		}
		entity, err := s.store.GetEntityByName(ctx, name)
		if err != nil {
			return toolError(err), nil
		}
		if entity == nil {
			return toolText(fmt.Sprintf(`{"error":"entity not found: %s"}`, name)), nil
		}
		rels, err := s.store.RelationshipsForEntity(ctx, entity.ID, depth)
		if err != nil {
			return toolError(err), nil
		}
		result := map[string]any{"entity": entity, "relationships": rels}
		b, _ := json.Marshal(result)
		return toolText(string(b)), nil
	}))

	// 5. find_relationships
	s.mcpServer.AddTool(mcpgo.NewTool("find_relationships",
		mcpgo.WithDescription("Find relationships by source, target, or predicate"),
		mcpgo.WithString("from", mcpgo.Description("Source entity ID")),
		mcpgo.WithString("to", mcpgo.Description("Target entity ID")),
		mcpgo.WithString("predicate", mcpgo.Description("Relationship predicate filter")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		from := stringArg(args, "from", "")
		to := stringArg(args, "to", "")
		pred := stringArg(args, "predicate", "")
		rels, err := s.store.FindRelationships(ctx, from, to, pred)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(rels)
		return toolText(string(b)), nil
	}))

	// 6. get_graph_neighborhood
	s.mcpServer.AddTool(mcpgo.NewTool("get_graph_neighborhood",
		mcpgo.WithDescription("Get subgraph nodes+edges JSON for visualization"),
		mcpgo.WithString("entity_name", mcpgo.Required(), mcpgo.Description("Center entity name")),
		mcpgo.WithNumber("depth", mcpgo.Description("BFS depth (default 2)")),
		mcpgo.WithNumber("max_nodes", mcpgo.Description("Max nodes to return (default 50)")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		name := stringArg(args, "entity_name", "")
		depth := intArg(args, "depth", 2)
		maxNodes := intArg(args, "max_nodes", 50)
		if name == "" {
			return toolError(fmt.Errorf("entity_name required")), nil
		}
		entity, err := s.store.GetEntityByName(ctx, name)
		if err != nil {
			return toolError(err), nil
		}
		if entity == nil {
			return toolText(`{"nodes":[],"edges":[]}`), nil
		}
		rels, err := s.store.RelationshipsForEntity(ctx, entity.ID, depth)
		if err != nil {
			return toolError(err), nil
		}
		// Collect unique node IDs
		nodeIDs := map[string]bool{entity.ID: true}
		for _, r := range rels {
			nodeIDs[r.SourceID] = true
			nodeIDs[r.TargetID] = true
		}
		var nodes []map[string]any
		count := 0
		for nid := range nodeIDs {
			if count >= maxNodes {
				break
			}
			e, err := s.store.GetEntity(ctx, nid)
			if err != nil || e == nil {
				continue
			}
			nodes = append(nodes, map[string]any{
				"id": e.ID, "label": e.Name, "type": e.Type,
				"description": e.Description, "rank": e.Rank,
			})
			count++
		}
		var edges []map[string]any
		for _, r := range rels {
			edges = append(edges, map[string]any{
				"id": r.ID, "from": r.SourceID, "to": r.TargetID,
				"label": r.Predicate, "weight": r.Weight,
			})
		}
		result := map[string]any{"nodes": nodes, "edges": edges}
		b, _ := json.Marshal(result)
		return toolText(string(b)), nil
	}))

	// 7. get_document_structure
	s.mcpServer.AddTool(mcpgo.NewTool("get_document_structure",
		mcpgo.WithDescription("Get LLM-generated structured summary of a document"),
		mcpgo.WithString("doc_id", mcpgo.Description("Document ID")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		docID := stringArg(args, "doc_id", "")
		if docID == "" {
			return toolError(fmt.Errorf("doc_id required")), nil
		}
		doc, err := s.store.GetDocument(ctx, docID)
		if err != nil {
			return toolError(err), nil
		}
		if doc == nil {
			return toolText(`{"error":"document not found"}`), nil
		}
		if doc.Structured == "" {
			return toolText(`{"error":"no structured data available"}`), nil
		}
		return toolText(doc.Structured), nil
	}))

	// 8. list_entities
	s.mcpServer.AddTool(mcpgo.NewTool("list_entities",
		mcpgo.WithDescription("Browse graph entities with optional type filter"),
		mcpgo.WithString("type", mcpgo.Description("Entity type filter")),
		mcpgo.WithNumber("limit", mcpgo.Description("Max results (default 20)")),
		mcpgo.WithNumber("offset", mcpgo.Description("Pagination offset")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		typ := stringArg(args, "type", "")
		limit := intArg(args, "limit", 20)
		offset := intArg(args, "offset", 0)
		entities, err := s.store.ListEntities(ctx, typ, limit, offset)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(entities)
		return toolText(string(b)), nil
	}))

	// 9. list_documents
	s.mcpServer.AddTool(mcpgo.NewTool("list_documents",
		mcpgo.WithDescription("Browse indexed documents"),
		mcpgo.WithString("doc_type", mcpgo.Description("Filter by type: pdf|docx|txt|md")),
		mcpgo.WithNumber("limit", mcpgo.Description("Max results (default 20)")),
		mcpgo.WithNumber("offset", mcpgo.Description("Pagination offset")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		docType := stringArg(args, "doc_type", "")
		limit := intArg(args, "limit", 20)
		offset := intArg(args, "offset", 0)
		docs, err := s.store.ListDocuments(ctx, docType, limit, offset)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(docs)
		return toolText(string(b)), nil
	}))

	// 10. get_community_report
	s.mcpServer.AddTool(mcpgo.NewTool("get_community_report",
		mcpgo.WithDescription("Get community summary and member entities"),
		mcpgo.WithString("community_id", mcpgo.Required(), mcpgo.Description("Community ID")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		commID := stringArg(args, "community_id", "")
		if commID == "" {
			return toolError(fmt.Errorf("community_id required")), nil
		}
		comm, err := s.store.GetCommunity(ctx, commID)
		if err != nil {
			return toolError(err), nil
		}
		if comm == nil {
			return toolText(`{"error":"community not found"}`), nil
		}
		members, err := s.store.CommunityMembers(ctx, commID)
		if err != nil {
			return toolError(err), nil
		}
		result := map[string]any{"community": comm, "members": members}
		b, _ := json.Marshal(result)
		return toolText(string(b)), nil
	}))

	// 11. get_chunk
	s.mcpServer.AddTool(mcpgo.NewTool("get_chunk",
		mcpgo.WithDescription("Retrieve a specific chunk by ID"),
		mcpgo.WithString("chunk_id", mcpgo.Required(), mcpgo.Description("Chunk ID")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		chunkID := stringArg(args, "chunk_id", "")
		if chunkID == "" {
			return toolError(fmt.Errorf("chunk_id required")), nil
		}
		chunk, err := s.store.GetChunk(ctx, chunkID)
		if err != nil {
			return toolError(err), nil
		}
		if chunk == nil {
			return toolText(`{"error":"chunk not found"}`), nil
		}
		b, _ := json.Marshal(chunk)
		return toolText(string(b)), nil
	}))

	// 12. stats
	s.mcpServer.AddTool(mcpgo.NewTool("stats",
		mcpgo.WithDescription("Get full index statistics"),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		stats, err := s.store.GetStats(ctx)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(stats)
		return toolText(string(b)), nil
	}))

	// 13. get_entity_claims
	s.mcpServer.AddTool(mcpgo.NewTool("get_entity_claims",
		mcpgo.WithDescription("List all claims extracted for a given entity"),
		mcpgo.WithString("entity_id", mcpgo.Required(), mcpgo.Description("Entity ID")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		entityID := stringArg(args, "entity_id", "")
		if entityID == "" {
			return toolError(fmt.Errorf("entity_id required")), nil
		}
		claims, err := s.store.ClaimsForEntity(ctx, entityID)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(claims)
		return toolText(string(b)), nil
	}))
}


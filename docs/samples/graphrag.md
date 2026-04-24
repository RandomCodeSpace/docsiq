# GraphRAG

GraphRAG is a retrieval-augmented generation (RAG) technique introduced
by Microsoft Research in 2024 that augments vector search with an
explicit knowledge graph. Instead of retrieving only the top-k chunks
semantically similar to a query, GraphRAG extracts entities, relations,
and claims from each chunk, builds a graph, runs Louvain community
detection on it, and then serves queries against either local (entity
neighbourhood) or global (community summary) views.

The key claim is that graph-derived structure recovers global context
that pure vector search cannot: "who are the main actors in this
corpus", "what are the dominant themes", questions that require a view
of the whole rather than a handful of passages.

docsiq is a Go implementation of this technique, shipping as a single
binary with an embedded React UI. It supports Azure OpenAI, OpenAI, and
Ollama as LLM providers, storing everything in SQLite with FTS5 and the
sqlite-vec extension for ANN vector search.

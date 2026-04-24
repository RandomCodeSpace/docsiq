# Quickstart

Go from zero to a queryable knowledge graph in under three minutes.

## What you'll do

1. Install the `docsiq` binary.
2. Register the current directory as a docsiq project.
3. Index a small sample corpus of three markdown documents.
4. Ask a question.
5. Open the UI and see the graph.

The sample corpus lives at [`docs/samples/`](samples/); it's three
short markdown files about Roman aqueducts, GraphRAG, and Louvain
community detection. Small enough to index in ~30 seconds, dense enough
to produce interesting entities and a multi-community graph.

## 1. Install

Download the latest release for your platform. Replace
`docsiq-linux-amd64` with the asset name matching your OS if needed
(macOS arm64, Windows amd64 assets are published alongside).

```bash
curl -LO https://github.com/RandomCodeSpace/docsiq/releases/latest/download/docsiq-linux-amd64
chmod +x docsiq-linux-amd64
mv docsiq-linux-amd64 ~/.local/bin/docsiq   # or any directory on your PATH
```

Verify:

```bash
docsiq version
```

Building from source is also supported and takes about a minute
end-to-end; see [CONTRIBUTING.md](../CONTRIBUTING.md) for the build
instructions.

## 2. Register a project

```bash
cd ~/path/to/any/directory     # or stay in the docsiq repo for the demo
docsiq init
```

`docsiq init` registers the current directory as a project and creates a
scope-specific SQLite store at `~/.docsiq/data/projects/<slug>/`. If
you're in a git repo, the slug is derived from the repo's remote origin;
otherwise you'll be prompted for a name.

## 3. Index the sample corpus

From the repository root (so that `docs/samples/` resolves):

```bash
docsiq index docs/samples/
```

You will see log lines for each phase:

```
⚙️ loaded config file path=/home/you/.docsiq/config.yaml
📄 loading documents count=3
🧩 chunking chunks=12
🌐 embedding batches=1
🔗 extracting entities entities=18 relationships=24
🧩 detecting communities levels=3 communities=5
✅ index complete duration=21.4s
```

If you are running without an LLM configured
(`DOCSIQ_LLM_PROVIDER=none` or `llm.provider: none` in the config),
entity extraction and embedding steps are skipped; you'll still get a
keyword-searchable corpus and a notes graph.

## 4. Ask a question

```bash
docsiq search "Who built the first Roman aqueduct?"
```

Expected (with an LLM configured):

```
Answer: Appius Claudius Caecus built the first Roman aqueduct, the
Aqua Appia, in 312 BCE in his role as censor.

Sources:
  roman-aqueducts.md (chunk 0)
```

For a corpus-scale question, try:

```bash
docsiq search "What are the main themes in this corpus?"
```

This triggers the global search path, which consults community
summaries rather than individual chunks.

## 5. Open the UI

```bash
docsiq serve
# → http://localhost:8080
```

Navigate to `http://localhost:8080`. You should see:

- **Home** — project picker, recent indexing activity.
- **Notes** — wikilinked markdown, even without any LLM configured.
- **Documents** — the three sample files with chunk counts.
- **Graph** — force-directed entity/community visualisation.
- **MCP** — inspector-style console for the 12+ MCP tools docsiq
  exposes at `/mcp`.

Screenshots of each view are in [`docs/screenshots/`](screenshots/).

## Where to next

- **Configure an LLM** — see [`configs/docsiq.example.yaml`](../configs/docsiq.example.yaml)
  for every option, default, and env-var override.
- **Integrate with Claude Desktop / Cursor** — run
  `docsiq hooks install --client claude-desktop`.
- **Index a real corpus** — `docsiq index /path/to/your/docs` accepts
  PDF, DOCX, TXT, and Markdown. Web pages can be fetched with
  `docsiq crawl <url>`.
- **Read the architecture overview** — [README.md](../README.md#architecture).
- **Contribute** — [CONTRIBUTING.md](../CONTRIBUTING.md).

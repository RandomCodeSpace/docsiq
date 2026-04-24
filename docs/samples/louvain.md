# Louvain community detection

Louvain is a greedy algorithm for community detection in large networks,
published by Blondel, Guillaume, Lambiotte, and Lefebvre in 2008. It
optimises modularity — a scalar that rewards dense within-community
links and sparse between-community links — through two alternating
phases: local move (every node is considered for reassignment to the
community that most increases modularity) and aggregation (each detected
community becomes a super-node in the next iteration).

The algorithm terminates when no move increases modularity. Runtime is
roughly linear in the number of edges, which is why Louvain scales to
graphs of tens of millions of nodes where spectral methods do not.

GraphRAG (see graphrag.md) uses Louvain to partition its entity graph
into nested communities. Each community gets an LLM-generated summary,
which is what the "global" search mode retrieves.

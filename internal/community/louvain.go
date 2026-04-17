package community

import (
	"math/rand"
)

// Graph represents an undirected weighted graph for community detection.
type Graph struct {
	Nodes     []string
	nodeIndex map[string]int
	Edges     []Edge
	adjMatrix [][]float64 // dense for simplicity
	totalWeight float64
}

// Edge in the graph.
type Edge struct {
	Source, Target int
	Weight         float64
}

// NewGraph builds a graph from node IDs and weighted edges.
func NewGraph(nodes []string, edges [][3]any) *Graph {
	g := &Graph{
		Nodes:     nodes,
		nodeIndex: make(map[string]int, len(nodes)),
	}
	for i, n := range nodes {
		g.nodeIndex[n] = i
	}
	n := len(nodes)
	g.adjMatrix = make([][]float64, n)
	for i := range g.adjMatrix {
		g.adjMatrix[i] = make([]float64, n)
	}
	for _, e := range edges {
		src, _ := e[0].(string)
		tgt, _ := e[1].(string)
		w, _ := e[2].(float64)
		if w == 0 {
			w = 1.0
		}
		si, ok1 := g.nodeIndex[src]
		ti, ok2 := g.nodeIndex[tgt]
		if !ok1 || !ok2 {
			continue
		}
		g.adjMatrix[si][ti] += w
		g.adjMatrix[ti][si] += w
		g.totalWeight += w
		g.Edges = append(g.Edges, Edge{si, ti, w})
	}
	return g
}

// NodeIndex returns the index for a node ID.
func (g *Graph) NodeIndex(id string) (int, bool) {
	idx, ok := g.nodeIndex[id]
	return idx, ok
}

// Louvain runs the Louvain community detection algorithm.
// Returns a map from node index → community ID (integer).
//
// Uses the standard modularity gain formula:
//
//	ΔQ = [k_i_in / (2m)] - [sigma_tot * k_i / (2m²)]
//
// where:
//   - k_i_in  = sum of edge weights from node i to nodes in community C
//   - sigma_tot = sum of all edge weights incident to nodes in community C
//   - k_i    = weighted degree of node i
//   - m      = total edge weight of the graph
func Louvain(g *Graph, maxIter int) []int {
	n := len(g.Nodes)
	if n == 0 {
		return nil
	}

	// Initialize: each node in its own community
	comm := make([]int, n)
	for i := range comm {
		comm[i] = i
	}

	if g.totalWeight == 0 {
		return comm
	}

	m := g.totalWeight // total edge weight
	m2 := 2.0 * m      // 2m, used frequently

	// Precompute node degrees
	degree := make([]float64, n)
	for i := range n {
		degree[i] = g.nodeDegree(i)
	}

	// Community total degree (sigma_tot): sum of degrees of all nodes in community
	sigmaTot := make(map[int]float64, n)
	for i := range n {
		sigmaTot[comm[i]] += degree[i]
	}

	improved := true
	for iter := 0; iter < maxIter && improved; iter++ {
		improved = false
		order := rand.Perm(n)
		for _, i := range order {
			bestComm := comm[i]
			bestGain := 0.0
			ki := degree[i]
			oldComm := comm[i]

			// Compute weights from node i to each neighboring community
			neighborComms := map[int]float64{}
			for j := range n {
				if g.adjMatrix[i][j] > 0 {
					neighborComms[comm[j]] += g.adjMatrix[i][j]
				}
			}

			// Remove node i from its current community for gain calculation
			sigmaTot[oldComm] -= ki

			// Gain of removing node i from its current community
			kiOld := neighborComms[oldComm] // edges from i to old community (after removal)
			removeLoss := kiOld/m2 - (sigmaTot[oldComm]*ki)/(m2*m2)

			for c, kiIn := range neighborComms {
				// Gain of adding node i to community c
				addGain := kiIn/m2 - (sigmaTot[c]*ki)/(m2*m2)
				gain := addGain - removeLoss
				if gain > bestGain {
					bestGain = gain
					bestComm = c
				}
			}

			// Move node i to best community
			comm[i] = bestComm
			sigmaTot[bestComm] += ki

			if bestComm != oldComm {
				improved = true
			}
		}
	}

	// Renumber communities 0..k-1
	renumber := map[int]int{}
	next := 0
	result := make([]int, n)
	for i, c := range comm {
		if _, ok := renumber[c]; !ok {
			renumber[c] = next
			next++
		}
		result[i] = renumber[c]
	}
	return result
}

func (g *Graph) nodeDegree(i int) float64 {
	var d float64
	for j := range g.adjMatrix[i] {
		d += g.adjMatrix[i][j]
	}
	return d
}

// HierarchicalLouvain runs Louvain at multiple levels.
// Returns a slice of levels, each level is a map nodeID → communityLabel.
func HierarchicalLouvain(g *Graph, maxLevels, maxIter int) [][]int {
	var levels [][]int
	current := Louvain(g, maxIter)
	levels = append(levels, current)

	for level := 1; level < maxLevels; level++ {
		// Count communities at current level
		commSet := map[int]bool{}
		for _, c := range current {
			commSet[c] = true
		}
		if len(commSet) <= 1 {
			break // Can't go higher
		}

		// Build super-graph where nodes = communities
		superNodes := make([]string, 0, len(commSet))
		superNodeIdx := map[int]int{}
		for c := range commSet {
			superNodeIdx[c] = len(superNodes)
			superNodes = append(superNodes, "")
		}

		superAdj := make([][]float64, len(superNodes))
		for i := range superAdj {
			superAdj[i] = make([]float64, len(superNodes))
		}

		for _, e := range g.Edges {
			ci := current[e.Source]
			cj := current[e.Target]
			if ci != cj {
				si := superNodeIdx[ci]
				sj := superNodeIdx[cj]
				superAdj[si][sj] += e.Weight
				superAdj[sj][si] += e.Weight
			}
		}

		superEdges := [][3]any{}
		for i := range superAdj {
			for j := i + 1; j < len(superAdj); j++ {
				if superAdj[i][j] > 0 {
					superEdges = append(superEdges, [3]any{superNodes[i], superNodes[j], superAdj[i][j]})
				}
			}
		}

		superGraph := NewGraph(superNodes, superEdges)
		superComm := Louvain(superGraph, maxIter)

		// Map back to original nodes
		nextLevel := make([]int, len(g.Nodes))
		for i, c := range current {
			si := superNodeIdx[c]
			nextLevel[i] = superComm[si]
		}

		// Check if we actually merged
		nextSet := map[int]bool{}
		for _, c := range nextLevel {
			nextSet[c] = true
		}
		if len(nextSet) >= len(commSet) {
			break
		}

		current = nextLevel
		levels = append(levels, current)
	}

	return levels
}

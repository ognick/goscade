package goscade

import (
	"fmt"
	"os"
)

// GraphNode represents a node in the dependency graph.
type GraphNode struct {
	ID string `json:"id"`
}

// GraphEdge represents an edge between two nodes in the dependency graph.
type GraphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// Graph represents the complete dependency graph structure.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// BuildGraph constructs a visual graph representation based on component dependencies.
// Returns a Graph structure containing all nodes (components) and edges (dependencies).
func (lc *lifecycle) BuildGraph() Graph {
	dependencies := lc.Dependencies()

	graph := Graph{
		Nodes: make([]GraphNode, 0, len(dependencies)),
		Edges: make([]GraphEdge, 0),
	}

	// Add all components as nodes
	for comp := range dependencies {
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID: lc.componentName(comp),
		})
	}

	// Add dependencies as edges
	for comp, parents := range dependencies {
		compName := lc.componentName(comp)
		for _, parent := range parents {
			parentName := lc.componentName(parent)
			graph.Edges = append(graph.Edges, GraphEdge{
				From: parentName,
				To:   compName,
			})
		}
	}

	return graph
}

// ToDOT converts the graph to Graphviz DOT format.
// Returns a string in DOT format that can be visualized with Graphviz tools.
func (g Graph) ToDOT() string {
	var result string
	result += "digraph G {\n"
	result += "  rankdir=TB;\n\n"

	// Add nodes
	for _, node := range g.Nodes {
		result += fmt.Sprintf("  %q [label=%q, shape=box];\n", node.ID, node.ID)
	}

	result += "\n"

	// Add edges
	for _, edge := range g.Edges {
		if edge.Label != "" {
			result += fmt.Sprintf("  %q -> %q [label=%q];\n", edge.From, edge.To, edge.Label)
		} else {
			result += fmt.Sprintf("  %q -> %q;\n", edge.From, edge.To)
		}
	}

	result += "}\n"
	return result
}

// writeGraphToFile writes the dependency graph to a file in DOT format.
func (lc *lifecycle) writeGraphToFile() error {
	if lc.graphOutputFile == "" {
		return nil
	}

	graph := lc.BuildGraph()
	dotContent := graph.ToDOT()

	file, err := os.Create(lc.graphOutputFile)
	if err != nil {
		return fmt.Errorf("failed to create graph output file: %w", err)
	}
	defer file.Close()

	// coverage: ignore - WriteString error is extremely rare (disk full, I/O error)
	// and cannot be reliably tested without system-level mocks
	if _, err := file.WriteString(dotContent); err != nil {
		return fmt.Errorf("failed to write graph: %w", err)
	}

	return nil
}

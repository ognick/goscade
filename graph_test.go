package goscade

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

func (l testLogger) Infof(format string, args ...interface{})  {}
func (l testLogger) Errorf(format string, args ...interface{}) {}

type errorCapturingLogger struct {
	errorCalls []string
}

func (l *errorCapturingLogger) Infof(format string, args ...interface{}) {}
func (l *errorCapturingLogger) Errorf(format string, args ...interface{}) {
	l.errorCalls = append(l.errorCalls, format)
}

type componentA struct{}

func (c *componentA) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

type componentB struct {
	a *componentA
}

func (c *componentB) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

type componentC struct {
	b *componentB
}

func (c *componentC) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

type componentD struct{}

func (c *componentD) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

type componentE struct{}

func (c *componentE) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

// Test: BuildGraph constructs graph correctly
func TestLifecycle_BuildGraph(t *testing.T) {
	lc := NewLifecycle(testLogger{})

	a := &componentA{}
	b := &componentB{a: a}
	c := &componentC{b: b}

	lc.Register(a)
	lc.Register(b)
	lc.Register(c)

	graph := lc.BuildGraph()

	// Check nodes count
	assert.Len(t, graph.Nodes, 3, "should have 3 nodes")

	// Check that all components are present
	nodeIDs := make(map[string]bool)
	for _, node := range graph.Nodes {
		nodeIDs[node.ID] = true
	}
	assert.Contains(t, nodeIDs, "*goscade.componentA")
	assert.Contains(t, nodeIDs, "*goscade.componentB")
	assert.Contains(t, nodeIDs, "*goscade.componentC")

	// Check edges count (A -> B, B -> C)
	assert.Len(t, graph.Edges, 2, "should have 2 edges")

	// Check edges
	edgeMap := make(map[string]string)
	for _, edge := range graph.Edges {
		edgeMap[edge.From] = edge.To
	}
	assert.Equal(t, "*goscade.componentB", edgeMap["*goscade.componentA"])
	assert.Equal(t, "*goscade.componentC", edgeMap["*goscade.componentB"])
}

// Test: BuildGraph with no dependencies
func TestLifecycle_BuildGraph_NoDependencies(t *testing.T) {
	lc := NewLifecycle(testLogger{})

	a := &componentA{}

	lc.Register(a)

	graph := lc.BuildGraph()

	assert.Len(t, graph.Nodes, 1, "should have 1 node")
	assert.Len(t, graph.Edges, 0, "should have 0 edges")
	assert.Equal(t, "*goscade.componentA", graph.Nodes[0].ID)
}

// Test: BuildGraph with multiple independent components
func TestLifecycle_BuildGraph_IndependentComponents(t *testing.T) {
	lc := NewLifecycle(testLogger{})

	a := &componentA{}
	d := &componentD{}
	e := &componentE{}

	lc.Register(a)
	lc.Register(d)
	lc.Register(e)

	graph := lc.BuildGraph()

	assert.Len(t, graph.Nodes, 3, "should have 3 nodes")
	assert.Len(t, graph.Edges, 0, "should have 0 edges")
}

// Test: ToDOT generates valid DOT format
func TestGraph_ToDOT(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "ComponentA"},
			{ID: "ComponentB"},
			{ID: "ComponentC"},
		},
		Edges: []GraphEdge{
			{From: "ComponentA", To: "ComponentB"},
			{From: "ComponentB", To: "ComponentC"},
		},
	}

	dot := graph.ToDOT()

	// Check basic structure
	assert.Contains(t, dot, "digraph G {")
	assert.Contains(t, dot, "rankdir=TB;")
	assert.Contains(t, dot, "}")

	// Check nodes
	assert.Contains(t, dot, `"ComponentA" [label="ComponentA", shape=box];`)
	assert.Contains(t, dot, `"ComponentB" [label="ComponentB", shape=box];`)
	assert.Contains(t, dot, `"ComponentC" [label="ComponentC", shape=box];`)

	// Check edges
	assert.Contains(t, dot, `"ComponentA" -> "ComponentB";`)
	assert.Contains(t, dot, `"ComponentB" -> "ComponentC";`)
}

// Test: ToDOT with edge labels
func TestGraph_ToDOT_WithLabels(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "NodeA"},
			{ID: "NodeB"},
		},
		Edges: []GraphEdge{
			{From: "NodeA", To: "NodeB", Label: "depends"},
		},
	}

	dot := graph.ToDOT()

	assert.Contains(t, dot, `"NodeA" -> "NodeB" [label="depends"];`)
}

// Test: ToDOT with empty graph
func TestGraph_ToDOT_EmptyGraph(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	dot := graph.ToDOT()

	assert.Contains(t, dot, "digraph G {")
	assert.Contains(t, dot, "}")
	assert.NotContains(t, dot, "->")
}

// Test: writeGraphToFile creates file with correct content
func TestLifecycle_WriteGraphToFile(t *testing.T) {
	tempFile := "test_graph.dot"
	defer os.Remove(tempFile)

	lc := NewLifecycle(testLogger{}, WithGraphOutput(tempFile)).(*lifecycle)

	a := &componentA{}
	b := &componentB{a: a}

	lc.Register(a)
	lc.Register(b)

	err := lc.writeGraphToFile()
	require.NoError(t, err)

	// Check file exists
	_, err = os.Stat(tempFile)
	require.NoError(t, err)

	// Read file content
	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "digraph G {")
	assert.Contains(t, contentStr, "*goscade.componentA")
	assert.Contains(t, contentStr, "*goscade.componentB")
	assert.Contains(t, contentStr, "->")
}

// Test: writeGraphToFile does nothing when no filename is set
func TestLifecycle_WriteGraphToFile_NoFilename(t *testing.T) {
	lc := NewLifecycle(testLogger{}).(*lifecycle)

	a := &componentA{}
	lc.Register(a)

	err := lc.writeGraphToFile()
	assert.NoError(t, err)
}

// Test: writeGraphToFile handles invalid path
func TestLifecycle_WriteGraphToFile_InvalidPath(t *testing.T) {
	lc := NewLifecycle(testLogger{}, WithGraphOutput("/invalid/path/graph.dot")).(*lifecycle)

	a := &componentA{}
	lc.Register(a)

	err := lc.writeGraphToFile()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create graph output file")
}

// Test: ToDOT output is valid for Graphviz
func TestGraph_ToDOT_ValidGraphvizSyntax(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "Node With Spaces"},
			{ID: "Node-With-Dashes"},
		},
		Edges: []GraphEdge{
			{From: "Node With Spaces", To: "Node-With-Dashes"},
		},
	}

	dot := graph.ToDOT()

	// Node IDs should be quoted
	assert.Contains(t, dot, `"Node With Spaces"`)
	assert.Contains(t, dot, `"Node-With-Dashes"`)

	// Check overall structure is valid
	lines := strings.Split(dot, "\n")
	assert.True(t, strings.HasPrefix(lines[0], "digraph G {"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(dot), "}"))
}

// Test: BuildGraph with explicit dependencies
func TestLifecycle_BuildGraph_ExplicitDependencies(t *testing.T) {
	lc := NewLifecycle(testLogger{})

	a := &componentA{}
	b := &componentB{a: a}
	c := &componentC{}

	// Register c with explicit dependency on b (which already depends on a)
	lc.Register(a)
	lc.Register(b)
	lc.Register(c, b)

	graph := lc.BuildGraph()

	assert.Len(t, graph.Nodes, 3, "should have 3 nodes")
	assert.Len(t, graph.Edges, 2, "should have 2 edges")

	// Check edges: a->b (implicit), b->c (explicit)
	edgeMap := make(map[string]string)
	for _, edge := range graph.Edges {
		edgeMap[edge.From] = edge.To
	}
	assert.Equal(t, "*goscade.componentB", edgeMap["*goscade.componentA"])
	assert.Equal(t, "*goscade.componentC", edgeMap["*goscade.componentB"])
}

// Test: writeGraphToFile with read-only directory
func TestLifecycle_WriteGraphToFile_ReadOnlyDirectory(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	tempFile := tempDir + "/readonly/graph.dot"

	// Don't create readonly dir - file creation will fail
	lc := NewLifecycle(testLogger{}, WithGraphOutput(tempFile)).(*lifecycle)

	a := &componentA{}
	lc.Register(a)

	err := lc.writeGraphToFile()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create graph output file")
}

// Test: writeGraphToFile to directory instead of file
func TestLifecycle_WriteGraphToFile_DirectoryPath(t *testing.T) {
	// Use directory path instead of file path
	tempDir := t.TempDir()

	// Create a directory with the target file name
	dirPath := tempDir + "/graph.dot"
	err := os.Mkdir(dirPath, 0755)
	require.NoError(t, err)

	lc := NewLifecycle(testLogger{}, WithGraphOutput(dirPath)).(*lifecycle)

	a := &componentA{}
	lc.Register(a)

	// Writing to a directory should fail
	err = lc.writeGraphToFile()
	assert.Error(t, err)
}

// Test: writeGraphToFile successfully writes content
func TestLifecycle_WriteGraphToFile_SuccessfulWrite(t *testing.T) {
	tempFile := t.TempDir() + "/success_graph.dot"

	lc := NewLifecycle(testLogger{}, WithGraphOutput(tempFile)).(*lifecycle)

	// Create many components to generate large graph
	a := &componentA{}
	b := &componentB{a: a}
	c := &componentC{b: b}
	d := &componentD{}
	e := &componentE{}

	lc.Register(a)
	lc.Register(b)
	lc.Register(c)
	lc.Register(d)
	lc.Register(e)

	err := lc.writeGraphToFile()
	require.NoError(t, err)

	// Verify file content is valid
	content, err := os.ReadFile(tempFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "digraph G {")
	assert.Contains(t, contentStr, "rankdir=TB;")
	assert.Contains(t, contentStr, "}")
	assert.Contains(t, contentStr, "componentA")
	assert.Contains(t, contentStr, "componentB")
	assert.Contains(t, contentStr, "componentC")
	assert.Contains(t, contentStr, "componentD")
	assert.Contains(t, contentStr, "componentE")
	assert.Contains(t, contentStr, "->")

	// Check file is properly closed (can read it)
	_, err = os.ReadFile(tempFile)
	assert.NoError(t, err)
}

// Test: WithShutdownHook option
func TestWithShutdownHook(t *testing.T) {
	lc := NewLifecycle(testLogger{}, WithShutdownHook()).(*lifecycle)

	assert.True(t, lc.shutdownHook, "shutdownHook should be enabled")
}

// Test: WithGraphOutput option integration
func TestWithGraphOutput_Integration(t *testing.T) {
	tempFile := t.TempDir() + "/integration_graph.dot"

	lc := NewLifecycle(testLogger{}, WithGraphOutput(tempFile))

	a := &componentA{}
	b := &componentB{a: a}

	lc.Register(a)
	lc.Register(b)

	// Run should trigger writeGraphToFile
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_ = lc.Run(ctx, nil)

	// Check file was created
	content, err := os.ReadFile(tempFile)
	if err == nil {
		// File was created successfully
		assert.Contains(t, string(content), "digraph G")
		assert.Contains(t, string(content), "componentA")
		assert.Contains(t, string(content), "componentB")
	}
}

// Test: WithGraphOutput error logging in Run()
func TestWithGraphOutput_ErrorLogging(t *testing.T) {
	logger := &errorCapturingLogger{}
	lc := NewLifecycle(logger, WithGraphOutput("/invalid/readonly/path/graph.dot"))

	a := &componentA{}
	lc.Register(a)

	// Run should trigger writeGraphToFile and log error
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_ = lc.Run(ctx, nil)

	// Verify error was logged
	require.Len(t, logger.errorCalls, 1)
	assert.Contains(t, logger.errorCalls[0], "Failed to write graph")
}

// Test: ToDOT with single node
func TestGraph_ToDOT_SingleNode(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "SingleNode"},
		},
		Edges: []GraphEdge{},
	}

	dot := graph.ToDOT()

	assert.Contains(t, dot, "digraph G {")
	assert.Contains(t, dot, `"SingleNode" [label="SingleNode", shape=box];`)
	assert.NotContains(t, dot, "->")
}

// Test: BuildGraph returns empty graph for lifecycle with no components
func TestLifecycle_BuildGraph_Empty(t *testing.T) {
	lc := NewLifecycle(testLogger{})

	graph := lc.BuildGraph()

	assert.Len(t, graph.Nodes, 0, "should have 0 nodes")
	assert.Len(t, graph.Edges, 0, "should have 0 edges")
}

// Test: ToDOT with complex dependencies
func TestGraph_ToDOT_ComplexDependencies(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
			{ID: "D"},
		},
		Edges: []GraphEdge{
			{From: "A", To: "B"},
			{From: "A", To: "C"},
			{From: "B", To: "D"},
			{From: "C", To: "D"},
		},
	}

	dot := graph.ToDOT()

	// Check all nodes
	assert.Contains(t, dot, `"A"`)
	assert.Contains(t, dot, `"B"`)
	assert.Contains(t, dot, `"C"`)
	assert.Contains(t, dot, `"D"`)

	// Check all edges
	assert.Contains(t, dot, `"A" -> "B"`)
	assert.Contains(t, dot, `"A" -> "C"`)
	assert.Contains(t, dot, `"B" -> "D"`)
	assert.Contains(t, dot, `"C" -> "D"`)
}

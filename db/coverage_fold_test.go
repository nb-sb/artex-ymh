package db

import (
	"strconv"
	"strings"
	"testing"
)

func TestFoldCoverageGraphKeepsRootsAndLimitsChildren(t *testing.T) {
	g := &CoverageGraphData{
		Nodes: []CoverageGraphNode{
			{Key: "root", Kind: "root_domain", Label: "example.com", InScope: true},
		},
		Edges: nil,
	}
	for i := 0; i < 50; i++ {
		key := "ep-" + strconv.Itoa(i)
		g.Nodes = append(g.Nodes, CoverageGraphNode{
			Key: key, Kind: "endpoint", Label: key, InScope: true, Tested: i%2 == 0,
		})
		g.Edges = append(g.Edges, CoverageGraphEdge{Src: key, Dst: "root"})
	}
	folded := FoldCoverageGraph(g, 20)
	if len(folded.Nodes) != 21 { // root + 20 endpoints
		t.Fatalf("nodes=%d want 21", len(folded.Nodes))
	}
	if len(folded.Edges) != 20 {
		t.Fatalf("edges=%d want 20", len(folded.Edges))
	}
	if !folded.Truncated {
		t.Fatal("expected truncated=true")
	}
}

func TestActivitySessionFilterSystemCond(t *testing.T) {
	cond, args := (ActivitySessionFilter{System: true}).cond(2)
	if cond == "" || len(args) != 0 {
		t.Fatalf("system cond=%q args=%v", cond, args)
	}
	for _, part := range []string{"worker='system'", "llm_switch", "llm_failover"} {
		if !strings.Contains(cond, part) {
			t.Fatalf("system cond missing %q: %s", part, cond)
		}
	}
}

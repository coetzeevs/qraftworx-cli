package cerebro

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/coetzeevs/cerebro/brain"
)

// testClient creates a Client backed by a temporary brain with no embeddings.
func testClient(t *testing.T) *Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sqlite")
	b, err := brain.Init(path, brain.EmbedConfig{Provider: "none"})
	if err != nil {
		t.Fatalf("brain.Init: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return &Client{brain: b}
}

// testClientWithGlobal creates a Client with both project and global brains.
func testClientWithGlobal(t *testing.T) *Client {
	t.Helper()
	projPath := filepath.Join(t.TempDir(), "project.sqlite")
	proj, err := brain.Init(projPath, brain.EmbedConfig{Provider: "none"})
	if err != nil {
		t.Fatalf("brain.Init (project): %v", err)
	}

	globalPath := filepath.Join(t.TempDir(), "global.sqlite")
	global, err := brain.Init(globalPath, brain.EmbedConfig{Provider: "none"})
	if err != nil {
		t.Fatalf("brain.Init (global): %v", err)
	}

	t.Cleanup(func() {
		_ = proj.Close()
		_ = global.Close()
	})

	return &Client{brain: proj, global: global}
}

// Task 1.4: open project brain

func TestNewClient_OpensProjectBrain(t *testing.T) {
	c := testClient(t)
	if c.brain == nil {
		t.Fatal("expected non-nil brain")
	}
}

// Task 1.5: Add + Search round-trip
// With noop embedder, Search will fail (requires embeddings).
// We test Add + List as the round-trip verification.

func TestClient_AddAndList(t *testing.T) {
	c := testClient(t)

	id, err := c.Add("test memory content", brain.Episode)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	nodes, err := c.List(brain.ListNodesOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Content != "test memory content" {
		t.Errorf("content=%q, want %q", nodes[0].Content, "test memory content")
	}
}

// Task 1.6: Add with options

func TestClient_Add_WithImportance(t *testing.T) {
	c := testClient(t)

	id, err := c.Add("important memory", brain.Concept, brain.WithImportance(0.9))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	nwe, err := c.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if nwe.Importance != 0.9 {
		t.Errorf("importance=%v, want 0.9", nwe.Importance)
	}
}

func TestClient_Add_WithSubtype(t *testing.T) {
	c := testClient(t)

	id, err := c.Add("debug session notes", brain.Episode, brain.WithSubtype("debug_session"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	nwe, err := c.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if nwe.Subtype != "debug_session" {
		t.Errorf("subtype=%q, want %q", nwe.Subtype, "debug_session")
	}
}

// Task 1.7: List by type

func TestClient_List_ByType(t *testing.T) {
	c := testClient(t)

	if _, err := c.Add("episode one", brain.Episode); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Add("concept one", brain.Concept); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Add("concept two", brain.Concept); err != nil {
		t.Fatal(err)
	}

	concepts, err := c.List(brain.ListNodesOpts{Type: brain.Concept})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %d", len(concepts))
	}
	for _, n := range concepts {
		if n.Type != brain.Concept {
			t.Errorf("expected type=%q, got %q", brain.Concept, n.Type)
		}
	}
}

// Task 1.8: Get with edges

func TestClient_Get_WithEdges(t *testing.T) {
	c := testClient(t)

	id1, err := c.Add("node one", brain.Concept)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := c.Add("node two", brain.Concept)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.AddEdge(id1, id2, "relates_to"); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	nwe, err := c.Get(id1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(nwe.Edges) == 0 {
		t.Fatal("expected at least 1 edge")
	}
	if nwe.Edges[0].TargetID != id2 {
		t.Errorf("edge target=%q, want %q", nwe.Edges[0].TargetID, id2)
	}
}

// Task 1.9: Stats

func TestClient_Stats(t *testing.T) {
	c := testClient(t)

	if _, err := c.Add("node one", brain.Episode); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Add("node two", brain.Concept); err != nil {
		t.Fatal(err)
	}

	stats, err := c.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalNodes != 2 {
		t.Errorf("total_nodes=%d, want 2", stats.TotalNodes)
	}
	if stats.ActiveNodes != 2 {
		t.Errorf("active_nodes=%d, want 2", stats.ActiveNodes)
	}
}

// Task 1.10: handles missing DB

func TestNewClient_MissingDB(t *testing.T) {
	_, err := NewClient("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for missing DB path")
	}
}

// Task 1.11: global store search
// With noop embedder, SearchWithGlobal will fail on vector search.
// We test that the client correctly wires both brains by adding to
// both and verifying List works on each independently.

func TestClient_WithGlobal_BothAccessible(t *testing.T) {
	c := testClientWithGlobal(t)

	// Add to project brain
	_, err := c.Add("project memory", brain.Concept)
	if err != nil {
		t.Fatalf("Add to project: %v", err)
	}

	// Add to global brain
	_, err = c.AddToGlobal("global memory", brain.Concept)
	if err != nil {
		t.Fatalf("AddToGlobal: %v", err)
	}

	// Project has 1
	projNodes, err := c.List(brain.ListNodesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(projNodes) != 1 {
		t.Errorf("project nodes=%d, want 1", len(projNodes))
	}

	// Global has 1
	globalNodes, err := c.ListGlobal(brain.ListNodesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(globalNodes) != 1 {
		t.Errorf("global nodes=%d, want 1", len(globalNodes))
	}
}

func TestClient_Close(t *testing.T) {
	c := testClientWithGlobal(t)

	// Override cleanup — we'll close manually
	err := c.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestClient_SearchWithGlobal_RequiresEmbeddings(t *testing.T) {
	c := testClientWithGlobal(t)

	// With noop embedder, search should return an error
	_, err := c.Search(context.Background(), "test query", 5, 0.3)
	if err == nil {
		t.Fatal("expected error from Search with noop embedder")
	}
}

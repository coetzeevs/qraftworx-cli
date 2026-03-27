package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coetzeevs/cerebro/brain"
	cerebroclient "github.com/coetzeevs/qraftworx-cli/internal/cerebro"
)

// MemoryAddTool adds a memory node to Cerebro.
type MemoryAddTool struct {
	client *cerebroclient.Client
}

// NewMemoryAddTool creates a MemoryAddTool.
func NewMemoryAddTool(client *cerebroclient.Client) *MemoryAddTool {
	return &MemoryAddTool{client: client}
}

func (t *MemoryAddTool) Name() string        { return "memory_add" }
func (t *MemoryAddTool) Description() string { return "Add a memory node to persistent storage" }
func (t *MemoryAddTool) Parameters() map[string]any {
	return map[string]any{
		"content": map[string]any{
			"type":        "STRING",
			"description": "content to remember",
			"required":    true,
		},
		"type": map[string]any{
			"type":        "STRING",
			"description": "node type: episode, concept, procedure, or reflection",
		},
	}
}
func (t *MemoryAddTool) RequiresConfirmation() bool  { return false }
func (t *MemoryAddTool) Permissions() ToolPermission { return ToolPermission{FileSystem: true} }

type memoryAddArgs struct {
	Content  string `json:"content"`
	NodeType string `json:"type"`
}

func (t *MemoryAddTool) Execute(_ context.Context, args json.RawMessage) (any, error) {
	var a memoryAddArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("memory_add: invalid args: %w", err)
	}
	if a.Content == "" {
		return nil, fmt.Errorf("memory_add: content is required")
	}

	var nodeType brain.NodeType
	switch a.NodeType {
	case "concept":
		nodeType = brain.Concept
	case "procedure":
		nodeType = brain.Procedure
	case "reflection":
		nodeType = brain.Reflection
	case "episode", "":
		nodeType = brain.Episode
	default:
		return nil, fmt.Errorf("memory_add: unknown type %q", a.NodeType)
	}

	id, err := t.client.Add(a.Content, nodeType)
	if err != nil {
		return nil, fmt.Errorf("memory_add: %w", err)
	}
	return map[string]any{"id": id, "status": "added"}, nil
}

// MemorySearchTool searches Cerebro for relevant memories.
type MemorySearchTool struct {
	client *cerebroclient.Client
}

// NewMemorySearchTool creates a MemorySearchTool.
func NewMemorySearchTool(client *cerebroclient.Client) *MemorySearchTool {
	return &MemorySearchTool{client: client}
}

func (t *MemorySearchTool) Name() string        { return "memory_search" }
func (t *MemorySearchTool) Description() string { return "Search persistent memory for relevant nodes" }
func (t *MemorySearchTool) Parameters() map[string]any {
	return map[string]any{
		"query": map[string]any{
			"type":        "STRING",
			"description": "search query",
			"required":    true,
		},
	}
}
func (t *MemorySearchTool) RequiresConfirmation() bool  { return false }
func (t *MemorySearchTool) Permissions() ToolPermission { return ToolPermission{FileSystem: true} }

type memorySearchArgs struct {
	Query string `json:"query"`
}

func (t *MemorySearchTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var a memorySearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("memory_search: invalid args: %w", err)
	}
	if a.Query == "" {
		return nil, fmt.Errorf("memory_search: query is required")
	}

	results, err := t.client.Search(ctx, a.Query, 5, 0.3)
	if err != nil {
		return nil, fmt.Errorf("memory_search: %w", err)
	}

	// Convert to serializable format
	items := make([]map[string]any, len(results))
	for i := range results {
		items[i] = map[string]any{
			"id":      results[i].ID,
			"content": results[i].Content,
			"type":    string(results[i].Type),
			"score":   results[i].Score,
		}
	}
	return map[string]any{"results": items, "count": len(items)}, nil
}

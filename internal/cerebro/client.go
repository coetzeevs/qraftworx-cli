package cerebro

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/coetzeevs/cerebro/brain"
)

// Client wraps brain.Brain with the operations Qraft needs.
// It is the sole interface between Qraft and Cerebro.
type Client struct {
	brain  *brain.Brain
	global *brain.Brain // nil if no global store
}

// NewClient opens a project brain at the given path.
// The path should be the project directory; the actual SQLite file
// is derived by brain.ProjectPath().
func NewClient(projectDir string) (*Client, error) {
	dbPath := brain.ProjectPath(projectDir)
	b, err := brain.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("cerebro: opening project brain at %q: %w", dbPath, err)
	}
	return &Client{brain: b}, nil
}

// NewClientWithGlobal opens both a project brain and the global brain.
func NewClientWithGlobal(projectDir string) (*Client, error) {
	c, err := NewClient(projectDir)
	if err != nil {
		return nil, err
	}

	globalPath := brain.GlobalPath()
	g, err := brain.Open(globalPath)
	if err != nil {
		_ = c.brain.Close()
		return nil, fmt.Errorf("cerebro: opening global brain at %q: %w", globalPath, err)
	}
	c.global = g
	return c, nil
}

// NewClientFromPath opens a brain directly from a SQLite file path.
// Used primarily for testing.
func NewClientFromPath(dbPath string) (*Client, error) {
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("cerebro: resolving path %q: %w", dbPath, err)
	}
	b, err := brain.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("cerebro: opening brain at %q: %w", absPath, err)
	}
	return &Client{brain: b}, nil
}

// NewClientForTest creates a Client from an existing Brain instance.
// This is intended for use in tests outside the cerebro package.
func NewClientForTest(b *brain.Brain) *Client {
	return &Client{brain: b}
}

// Close closes all brain connections.
func (c *Client) Close() error {
	var errs []error
	if err := c.brain.Close(); err != nil {
		errs = append(errs, fmt.Errorf("closing project brain: %w", err))
	}
	if c.global != nil {
		if err := c.global.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing global brain: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cerebro: %v", errs)
	}
	return nil
}

// Search returns scored memory nodes matching the query.
func (c *Client) Search(ctx context.Context, query string, limit int, threshold float64) ([]brain.ScoredNode, error) {
	return c.brain.Search(ctx, query, limit, threshold)
}

// SearchWithGlobal searches both project and global stores, merging results.
func (c *Client) SearchWithGlobal(ctx context.Context, query string, limit int, threshold float64) ([]brain.ScoredNode, error) {
	if c.global == nil {
		return c.Search(ctx, query, limit, threshold)
	}
	return c.brain.SearchWithGlobal(ctx, query, limit, threshold, c.global)
}

// Add stores a new memory node in the project brain.
func (c *Client) Add(content string, nodeType brain.NodeType, opts ...brain.AddOption) (string, error) {
	return c.brain.Add(content, nodeType, opts...)
}

// AddToGlobal stores a new memory node in the global brain.
func (c *Client) AddToGlobal(content string, nodeType brain.NodeType, opts ...brain.AddOption) (string, error) {
	if c.global == nil {
		return "", fmt.Errorf("cerebro: no global brain configured")
	}
	return c.global.Add(content, nodeType, opts...)
}

// List returns nodes matching filters from the project brain.
func (c *Client) List(opts brain.ListNodesOpts) ([]brain.Node, error) {
	return c.brain.List(opts)
}

// ListGlobal returns nodes matching filters from the global brain.
func (c *Client) ListGlobal(opts brain.ListNodesOpts) ([]brain.Node, error) {
	if c.global == nil {
		return nil, fmt.Errorf("cerebro: no global brain configured")
	}
	return c.global.List(opts)
}

// Get retrieves a single node with edges from the project brain.
func (c *Client) Get(id string) (*brain.NodeWithEdges, error) {
	return c.brain.Get(id)
}

// AddEdge creates a relationship between two nodes in the project brain.
func (c *Client) AddEdge(sourceID, targetID, relation string) (int64, error) {
	return c.brain.AddEdge(sourceID, targetID, relation)
}

// Stats returns brain health metrics for the project brain.
func (c *Client) Stats() (*brain.Stats, error) {
	return c.brain.Stats()
}

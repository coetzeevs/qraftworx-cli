package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface every Qraft tool must implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args json.RawMessage) (any, error)
	RequiresConfirmation() bool
	Permissions() ToolPermission
}

// ToolPermission declares what capabilities a tool needs.
type ToolPermission struct {
	Network      bool
	FileSystem   bool
	Hardware     bool
	MediaCapture bool
	Upload       bool
}

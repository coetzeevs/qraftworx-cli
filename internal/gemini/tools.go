package gemini

import "google.golang.org/genai"

// ToolDeclarer is implemented by types that can declare themselves as Gemini tools.
// This interface is satisfied by internal/tools.Tool in Phase 3.
type ToolDeclarer interface {
	Name() string
	Description() string
	Parameters() map[string]any
}

// BuildToolDeclarations converts a slice of ToolDeclarers into Gemini tool declarations.
func BuildToolDeclarations(tools []ToolDeclarer) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	defs := make([]*genai.FunctionDeclaration, len(tools))
	for i, t := range tools {
		defs[i] = &genai.FunctionDeclaration{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  schemaFromMap(t.Parameters()),
		}
	}

	return []*genai.Tool{{FunctionDeclarations: defs}}
}

// ToolNames extracts tool names from a slice of ToolDeclarers.
func ToolNames(tools []ToolDeclarer) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name()
	}
	return names
}

// schemaFromMap converts a simple parameter map to a genai.Schema.
func schemaFromMap(params map[string]any) *genai.Schema {
	if len(params) == 0 {
		return nil
	}

	properties := make(map[string]*genai.Schema)
	var required []string

	for name, spec := range params {
		prop := &genai.Schema{}
		switch v := spec.(type) {
		case map[string]any:
			if t, ok := v["type"].(string); ok {
				prop.Type = genai.Type(t)
			}
			if d, ok := v["description"].(string); ok {
				prop.Description = d
			}
			if r, ok := v["required"].(bool); ok && r {
				required = append(required, name)
			}
		case string:
			prop.Type = genai.Type(v)
		}
		properties[name] = prop
	}

	return &genai.Schema{
		Type:       genai.TypeObject,
		Properties: properties,
		Required:   required,
	}
}

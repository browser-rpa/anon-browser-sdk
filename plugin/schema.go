package plugin

import "encoding/json"

// Schema covers the common JSON Schema fields used by plugin nodes. Authors
// needing custom extensions can assign json.RawMessage to NodeSchema directly.
type Schema struct {
	Type                 string            `json:"type,omitempty"`
	Title                string            `json:"title,omitempty"`
	Description          string            `json:"description,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	AdditionalProperties *bool             `json:"additionalProperties,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Enum                 []string          `json:"enum,omitempty"`
}

func String(title string) Schema { return Schema{Type: "string", Title: title} }
func Object(properties map[string]Schema, required ...string) Schema {
	additional := false
	return Schema{Type: "object", Properties: properties, Required: required, AdditionalProperties: &additional}
}
func Describe(metadata NodeMetadata, input, output Schema, policy NodePolicy) (NodeDescriptor, error) {
	in, err := json.Marshal(input)
	if err != nil {
		return NodeDescriptor{}, err
	}
	out, err := json.Marshal(output)
	if err != nil {
		return NodeDescriptor{}, err
	}
	return NodeDescriptor{Metadata: metadata, Schema: NodeSchema{Version: 1, Input: in, Output: out, Ports: []NodePort{{Name: "in", Kind: "input"}, {Name: "success", Kind: "output"}, {Name: "error", Kind: "error"}}}, Policy: policy}, nil
}

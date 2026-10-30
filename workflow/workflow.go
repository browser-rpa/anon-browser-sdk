// Package workflow defines public workflow JSON and Go authoring helpers.
// It contains no executors, browser implementation or storage access.
package workflow

import (
	"encoding/json"
	"fmt"
)

type NodeInstance struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Version  int             `json:"version"`
	Config   json.RawMessage `json:"config"`
	Settings NodeSettings    `json:"settings,omitzero"`
	UI       NodeUI          `json:"ui"`
}

// NodeSettings contains runtime behavior shared by multiple node types. It is
// kept outside Config so node-specific JSON Schemas remain focused on business
// inputs and common behavior can evolve without duplicating fields everywhere.
// Pointer values distinguish "inherit the workflow default" from an explicit
// zero, which is meaningful for delays and typing intervals.
type NodeSettings struct {
	TimeoutMS     *int `json:"timeout_ms,omitempty"`
	PostDelayMS   *int `json:"post_delay_ms,omitempty"`
	KeyIntervalMS *int `json:"key_interval_ms,omitempty"`
}

type NodeUI struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Edge struct {
	From     string `json:"from"`
	FromPort string `json:"from_port"`
	To       string `json:"to"`
	ToPort   string `json:"to_port"`
}

type WorkflowInputType string

const (
	WorkflowInputString  WorkflowInputType = "string"
	WorkflowInputNumber  WorkflowInputType = "number"
	WorkflowInputInteger WorkflowInputType = "integer"
	WorkflowInputBoolean WorkflowInputType = "boolean"
	WorkflowInputObject  WorkflowInputType = "object"
	WorkflowInputArray   WorkflowInputType = "array"
)

type WorkflowInputValidation struct {
	MinLength *int     `json:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty"`
	Minimum   *float64 `json:"minimum,omitempty"`
	Maximum   *float64 `json:"maximum,omitempty"`
}

// WorkflowInputDefinition is the transport-neutral contract used by the
// editor, HTTP API and future MCP adapter to render and validate run inputs.
// It is task configuration, not a node schema, so changing it does not mutate
// an already published graph version.
type WorkflowInputDefinition struct {
	Name        string                  `json:"name"`
	Type        WorkflowInputType       `json:"type"`
	Title       string                  `json:"title"`
	Description string                  `json:"description,omitempty"`
	Required    bool                    `json:"required,omitempty"`
	Default     any                     `json:"default,omitempty"`
	Options     []string                `json:"options,omitempty"`
	Validation  WorkflowInputValidation `json:"validation,omitempty"`
}

type Graph struct {
	SchemaVersion int                 `json:"schema_version"`
	StartNodeID   string              `json:"start_node_id"`
	Nodes         []NodeInstance      `json:"nodes"`
	Edges         []Edge              `json:"edges"`
	Subflows      []SubflowDefinition `json:"subflows,omitempty"`
	ReturnValues  map[string]any      `json:"return_values,omitempty"`
}
type SubflowDefinition struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	WorkflowVersionID string                    `json:"workflow_version_id,omitempty"`
	Inputs            []WorkflowInputDefinition `json:"inputs,omitempty"`
	Graph             Graph                     `json:"graph"`
}
type Workflow struct {
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Graph       Graph  `json:"graph"`
}

// Spec is an authoring instruction, not a wire type. Build encodes Config and
// propagates encoding errors; no helper panics or silently drops bad config.
type Spec struct {
	ID, Type string
	Version  int
	Config   any
	Settings NodeSettings
	UI       *NodeUI
}

func N(id, typ string, config any) Spec { return Spec{ID: id, Type: typ, Version: 1, Config: config} }

type Builder struct {
	value Workflow
	err   error
}

func New(projectID, name string) *Builder {
	return &Builder{value: Workflow{ProjectID: projectID, Name: name, Graph: Graph{SchemaVersion: 3, Edges: []Edge{}}}}
}
func (b *Builder) Description(text string) *Builder { b.value.Description = text; return b }
func (b *Builder) Add(specs ...Spec) *Builder {
	for _, spec := range specs {
		if b.err != nil {
			break
		}
		if spec.Config == nil {
			spec.Config = struct{}{}
		}
		data, err := json.Marshal(spec.Config)
		if err != nil {
			b.err = fmt.Errorf("node %s config: %w", spec.ID, err)
			break
		}
		var object map[string]any
		if json.Unmarshal(data, &object) != nil || object == nil {
			b.err = fmt.Errorf("node %s config must be an object", spec.ID)
			break
		}
		node := NodeInstance{ID: spec.ID, Type: spec.Type, Version: spec.Version, Config: data, Settings: spec.Settings, UI: NodeUI{X: 40 + float64(len(b.value.Graph.Nodes))*240, Y: 160}}
		if spec.UI != nil {
			node.UI = *spec.UI
		}
		b.value.Graph.Nodes = append(b.value.Graph.Nodes, node)
		if b.value.Graph.StartNodeID == "" {
			b.value.Graph.StartNodeID = node.ID
		}
	}
	return b
}
func (b *Builder) Link(from, port, to string) *Builder {
	b.value.Graph.Edges = append(b.value.Graph.Edges, Edge{From: from, FromPort: port, To: to, ToPort: "in"})
	return b
}

// Then connects success ports only. Explicitly wire timeout/failed/rejected
// branches with Link; a failed operation must not be silently marked success.
func (b *Builder) Then(ids ...string) *Builder {
	for i := 1; i < len(ids); i++ {
		b.Link(ids[i-1], "success", ids[i])
	}
	return b
}
func (b *Builder) Subflow(def SubflowDefinition) *Builder {
	b.value.Graph.Subflows = append(b.value.Graph.Subflows, def)
	return b
}
func (b *Builder) Returns(values map[string]any) *Builder {
	b.value.Graph.ReturnValues = values
	return b
}
func (b *Builder) Build() (Workflow, error) {
	if b.err != nil {
		return Workflow{}, b.err
	}
	if err := b.value.Graph.ValidateShape(); err != nil {
		return Workflow{}, err
	}
	// Return an independent snapshot, not the builder's mutable slices/maps.
	data, err := json.Marshal(b.value)
	if err != nil {
		return Workflow{}, err
	}
	var result Workflow
	err = json.Unmarshal(data, &result)
	return result, err
}
func (g Graph) ValidateShape() error { return g.validateShape(0) }
func (g Graph) validateShape(depth int) error {
	if depth > 8 {
		return fmt.Errorf("subflow nesting exceeds 8")
	}
	if g.SchemaVersion < 1 || g.SchemaVersion > 3 || len(g.Nodes) == 0 {
		return fmt.Errorf("invalid graph schema or empty nodes")
	}
	seen := map[string]bool{}
	for _, node := range g.Nodes {
		if node.ID == "" || node.Type == "" || node.Version < 1 || seen[node.ID] {
			return fmt.Errorf("invalid or duplicate node %q", node.ID)
		}
		seen[node.ID] = true
	}
	if !seen[g.StartNodeID] {
		return fmt.Errorf("start node does not exist")
	}
	for _, edge := range g.Edges {
		if !seen[edge.From] || !seen[edge.To] || edge.FromPort == "" || edge.ToPort == "" {
			return fmt.Errorf("invalid edge from %s to %s", edge.From, edge.To)
		}
	}
	subs := map[string]bool{}
	for _, sub := range g.Subflows {
		if sub.ID == "" || subs[sub.ID] {
			return fmt.Errorf("invalid or duplicate subflow %q", sub.ID)
		}
		subs[sub.ID] = true
		if err := sub.Graph.validateShape(depth + 1); err != nil {
			return err
		}
	}
	return nil
}

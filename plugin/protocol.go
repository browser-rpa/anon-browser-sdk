// Package plugin provides the public stdio contract and Go runtime for
// anon-browser automation plugins. It depends only on the standard library.
package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

const Protocol = "anon-browser.plugin.node.v1"
const MaxInvocationBytes = 1 << 20
const MaxMessageBytes = 4 << 20
const MaxCapabilityCalls = 64

type Invocation struct {
	Protocol string          `json:"protocol"`
	PluginID string          `json:"plugin_id"`
	NodeType string          `json:"node_type"`
	NodeVer  int             `json:"node_version"`
	Run      json.RawMessage `json:"run"`
	NodeID   string          `json:"node_id"`
	Attempt  int             `json:"attempt"`
	Input    map[string]any  `json:"input,omitempty"`
	Settings json.RawMessage `json:"settings,omitempty"`
}
type NodeResult struct {
	Output     map[string]any `json:"output,omitempty"`
	Variables  map[string]any `json:"variables,omitempty"`
	NextPort   string         `json:"next_port"`
	Artifacts  []ArtifactRef  `json:"artifacts,omitempty"`
	Checkpoint *Checkpoint    `json:"checkpoint,omitempty"`
}
type ArtifactRef struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}
type Checkpoint struct {
	Cursor string `json:"cursor"`
	Value  any    `json:"value,omitempty"`
}
type CapabilityCall struct {
	Type       string         `json:"type,omitempty"`
	ID         string         `json:"id"`
	Capability string         `json:"capability"`
	Action     string         `json:"action"`
	Arguments  map[string]any `json:"arguments,omitempty"`
}
type CapabilityResult struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}
type Response struct {
	OK     bool        `json:"ok"`
	Result *NodeResult `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}
type NodeCall struct {
	Invocation Invocation
	Client     *Client
}

func (c NodeCall) String(name string) string {
	text, _ := c.Invocation.Input[name].(string)
	return text
}

// DecodeInput lets a plugin define its own business input once, without wire structs.
func (c NodeCall) DecodeInput(target any) error {
	data, err := json.Marshal(c.Invocation.Input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

type Handler func(context.Context, NodeCall) (NodeResult, error)
type nodeKey struct {
	name    string
	version int
}
type Server struct{ handlers map[nodeKey]Handler }

func NewServer() *Server { return &Server{handlers: make(map[nodeKey]Handler)} }
func (s *Server) Register(nodeType string, version int, handler Handler) error {
	if strings.TrimSpace(nodeType) == "" || version < 1 || handler == nil {
		return errors.New("node type, version and handler are required")
	}
	key := nodeKey{nodeType, version}
	if _, exists := s.handlers[key]; exists {
		return fmt.Errorf("node %s@%d already registered", nodeType, version)
	}
	s.handlers[key] = handler
	return nil
}

// Serve handles exactly one invocation and returns after writing the result.
// The host waits for process exit before closing stdout; reading a second invocation
// here would deadlock. The host enforces deadlines and cancellation by terminating the
// child. A generic io.Reader cannot be interrupted by context alone.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), MaxMessageBytes+1)
	client := &Client{scanner: scanner, writer: out}
	defer func() { client.mu.Lock(); client.closed = true; client.mu.Unlock() }()
	var invocation Invocation
	var result NodeResult
	err := ctx.Err()
	if err == nil {
		if !scanner.Scan() {
			err = scanner.Err()
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
		} else if len(scanner.Bytes()) > MaxInvocationBytes {
			err = errors.New("invocation exceeds 1 MiB")
		} else {
			err = json.Unmarshal(scanner.Bytes(), &invocation)
		}
	}
	if err == nil && (invocation.Protocol != Protocol || invocation.PluginID == "" || invocation.NodeID == "" || invocation.Attempt < 1) {
		err = errors.New("invalid plugin invocation")
	}
	if err == nil {
		handler := s.handlers[nodeKey{invocation.NodeType, invocation.NodeVer}]
		if handler == nil {
			err = fmt.Errorf("unsupported node %s@%d", invocation.NodeType, invocation.NodeVer)
		} else {
			result, err = invoke(ctx, handler, NodeCall{Invocation: invocation, Client: client})
		}
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && (strings.TrimSpace(result.NextPort) == "" || len(result.Output) > 256 || len(result.Variables) > 256 || len(result.Artifacts) > 256) {
		err = errors.New("invalid node result")
	}
	response := Response{OK: err == nil}
	if err != nil {
		response.Error = err.Error()
		if len(response.Error) > 4096 {
			response.Error = response.Error[:4096]
		}
	} else {
		response.Result = &result
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	client.closed = true
	return client.write(response)
}
func invoke(ctx context.Context, handler Handler, call NodeCall) (result NodeResult, err error) {
	defer func() {
		if recover() != nil {
			result = NodeResult{}
			err = errors.New("plugin handler panicked")
		}
	}()
	return handler(ctx, call)
}

type Client struct {
	mu      sync.Mutex
	scanner *bufio.Scanner
	writer  io.Writer
	nextID  int
	written int
	closed  bool
}

func (c *Client) write(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if c.written+len(data)+1 > MaxMessageBytes {
		return errors.New("plugin output exceeds 4 MiB")
	}
	data = append(data, '\n')
	n, err := c.writer.Write(data)
	c.written += n
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

// Call is the low-level escape hatch for capabilities supported by the host.
// Calls are serialized; no automatic retry is made for side-effecting actions.
func (c *Client) Call(ctx context.Context, capability, action string, arguments map[string]any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.closed {
		return nil, errors.New("node invocation has finished")
	}
	if c.nextID >= MaxCapabilityCalls {
		return nil, errors.New("plugin capability call limit exceeded")
	}
	c.nextID++
	id := fmt.Sprintf("call-%d", c.nextID)
	if err := c.write(CapabilityCall{Type: "capability_call", ID: id, Capability: capability, Action: action, Arguments: arguments}); err != nil {
		return nil, err
	}
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.ErrUnexpectedEOF
	}
	var response CapabilityResult
	if err := json.Unmarshal(c.scanner.Bytes(), &response); err != nil {
		return nil, err
	}
	if response.Type != "capability_result" || response.ID != id {
		return nil, errors.New("capability response correlation mismatch")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "capability call failed"
		}
		return nil, errors.New(response.Error)
	}
	return response.Data, nil
}

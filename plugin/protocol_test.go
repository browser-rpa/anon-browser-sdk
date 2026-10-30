package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func invocationLine() string {
	data, _ := json.Marshal(Invocation{Protocol: Protocol, PluginID: "test", NodeType: "test.echo", NodeVer: 1, NodeID: "n1", Attempt: 1, Input: map[string]any{"text": "hello"}})
	return string(data) + "\n"
}
func TestServeExitsAfterOneInvocation(t *testing.T) {
	server := NewServer()
	if err := server.Register("test.echo", 1, func(_ context.Context, call NodeCall) (NodeResult, error) {
		return NodeResult{NextPort: "success", Output: map[string]any{"text": call.String("text")}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), reader, &out) }()
	if _, err := io.WriteString(writer, invocationLine()); err != nil {
		t.Fatal(err)
	}
	// stdin stays open, just like the host waiting for process exit.
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server blocked waiting for another invocation")
	}
	var response Response
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || !response.OK || response.Result.Output["text"] != "hello" {
		t.Fatalf("%s %v", out.String(), err)
	}
}
func TestCapabilitiesUseUniqueIDsAndTypedResults(t *testing.T) {
	server := NewServer()
	var saved *Client
	if err := server.Register("test.echo", 1, func(ctx context.Context, call NodeCall) (NodeResult, error) {
		saved = call.Client
		title, err := call.Client.ReadTitle(ctx)
		if err != nil || title != "page" {
			return NodeResult{}, errors.New("bad title")
		}
		response, err := call.Client.HTTPRequest(ctx, HTTPRequest{Method: "GET", URL: "https://example.test"})
		if err != nil {
			return NodeResult{}, err
		}
		if response.Headers["X-Test"][0] != "one" {
			return NodeResult{}, errors.New("bad response headers")
		}
		return NodeResult{NextPort: "success", Output: map[string]any{"status": response.StatusCode}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	in := invocationLine() + `{"type":"capability_result","id":"call-1","ok":true,"data":"page"}` + "\n" + `{"type":"capability_result","id":"call-2","ok":true,"data":{"status_code":200,"headers":{"X-Test":["one","two"]},"body":"ok"}}` + "\n"
	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&out)
	for _, want := range []string{"call-1", "call-2"} {
		var call CapabilityCall
		if err := decoder.Decode(&call); err != nil || call.ID != want {
			t.Fatal(call, err)
		}
		if _, exists := call.Arguments["body"]; exists {
			t.Fatal("GET unexpectedly sent a null body")
		}
	}
	var response Response
	if err := decoder.Decode(&response); err != nil || !response.OK {
		t.Fatal(response, err)
	}
	if _, err := saved.ReadTitle(context.Background()); err == nil {
		t.Fatal("call after completion accepted")
	}
}
func TestMalformedInvocationsAndCapabilitiesFailClosed(t *testing.T) {
	for name, input := range map[string]string{
		"protocol": strings.Replace(invocationLine(), Protocol, "unsupported", 1),
		"version":  strings.Replace(invocationLine(), `"node_version":1`, `"node_version":2`, 1),
		"bad-json": "{\n", "oversize": strings.Repeat(" ", MaxInvocationBytes) + "{}\n",
		"wrong-id": invocationLine() + `{"type":"capability_result","id":"wrong","ok":true}` + "\n",
		"denied":   invocationLine() + `{"type":"capability_result","id":"call-1","ok":false,"error":"denied"}` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			server := NewServer()
			if err := server.Register("test.echo", 1, func(ctx context.Context, call NodeCall) (NodeResult, error) {
				_, err := call.Client.ReadTitle(ctx)
				return NodeResult{NextPort: "success"}, err
			}); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := server.Serve(context.Background(), strings.NewReader(input), &out); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			var response Response
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &response); err != nil || response.OK || response.Error == "" {
				t.Fatal(out.String(), err)
			}
		})
	}
}

func TestServeRejectsTooManyArtifacts(t *testing.T) {
	server := NewServer()
	if err := server.Register("test.artifacts", 1, func(context.Context, NodeCall) (NodeResult, error) {
		artifacts := make([]ArtifactRef, 257)
		return NodeResult{NextPort: "success", Artifacts: artifacts}, nil
	}); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Invocation{Protocol: Protocol, PluginID: "test", NodeType: "test.artifacts", NodeVer: 1, NodeID: "n1", Attempt: 1})
	var out bytes.Buffer
	if err := server.Serve(context.Background(), bytes.NewReader(append(data, '\n')), &out); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.OK {
		t.Fatalf("too many artifacts accepted: %s %v", out.String(), err)
	}
}
func TestCancellationPanicAndDuplicateRegistration(t *testing.T) {
	server := NewServer()
	handler := func(context.Context, NodeCall) (NodeResult, error) { panic("must not expose stack or credentials") }
	if err := server.Register("test.echo", 1, handler); err != nil {
		t.Fatal(err)
	}
	if err := server.Register("test.echo", 1, handler); err == nil {
		t.Fatal("duplicate accepted")
	}
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		var out bytes.Buffer
		if err := server.Serve(ctx, strings.NewReader(invocationLine()), &out); err != nil {
			t.Fatal(err)
		}
		cancel()
		var result Response
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.OK {
			t.Fatal(out.String(), err)
		}
		if strings.Contains(out.String(), "credentials") {
			t.Fatal("panic leaked")
		}
	}
}

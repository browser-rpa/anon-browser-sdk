package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuilderSnapshotAndExplicitPorts(t *testing.T) {
	builder := New("local", "sample").Add(Start("start"), Wait("wait", 100), End("end")).Then("start", "wait", "end")
	result, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	result.Graph.Nodes[0].ID = "changed"
	result.Graph.Edges[0].From = "changed"
	next, err := builder.Build()
	if err != nil || next.Graph.Nodes[0].ID != "start" || next.Graph.Edges[0].From != "start" {
		t.Fatal(next, err)
	}
	data, _ := json.Marshal(next)
	if strings.Contains(string(data), `"mode"`) {
		t.Fatal("flow.wait does not accept mode", string(data))
	}
}
func TestBuilderRejectsInvalidGraphsAndUnencodableConfig(t *testing.T) {
	for _, builder := range []*Builder{
		New("local", "empty"), New("local", "dup").Add(Start("x"), End("x")),
		New("local", "badedge").Add(Start("s")).Link("s", "success", "missing"),
		New("local", "badconfig").Add(N("x", "data.set", map[string]any{"value": make(chan int)})),
		New("local", "badobject").Add(N("x", "data.set", []string{"bad"})),
	} {
		if _, err := builder.Build(); err == nil {
			t.Fatal("invalid graph accepted")
		}
	}
}
func TestTypedReferencesAndSubflows(t *testing.T) {
	sub, err := New("local", "sub").Add(Start("start"), End("end")).Then("start", "end").Returns(map[string]any{"status": "ok"}).Build()
	if err != nil {
		t.Fatal(err)
	}
	result, err := New("local", "main").Subflow(SubflowDefinition{ID: "shared", Graph: sub.Graph}).Add(Start("start"), Call("call", CallConfig{SubflowID: "shared"}), End("end")).Then("start", "call", "end").Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Graph.Subflows) != 1 || result.Graph.SchemaVersion != 3 {
		t.Fatal(result)
	}
	data, _ := json.Marshal(map[string]any{"value": Ref("result"), "secret": Secret("secret://example/key")})
	if !strings.Contains(string(data), `"variable_ref":"result"`) || !strings.Contains(string(data), `"secret_ref":"secret://example/key"`) {
		t.Fatal(string(data))
	}
}

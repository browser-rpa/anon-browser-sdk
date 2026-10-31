package workflow

import (
	"encoding/json"
	"testing"
)

func TestEnsureSessionExplicitBranches(t *testing.T) {
	b := New("local", "prepare").Add(Start("start"), EnsureSession("prepare", EnsureSessionConfig{SessionKey: "main", CreateIfMissing: true, Name: "Main", OutputVariable: "session.id"}), End("end")).Then("start", "prepare")
	for _, port := range []string{"created", "reused", "missing", "busy", "needs_review", "closed"} {
		b.Link("prepare", port, "end")
	}
	w, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	var config EnsureSessionConfig
	if err := json.Unmarshal(w.Graph.Nodes[1].Config, &config); err != nil {
		t.Fatal(err)
	}
	if config.SessionKey != "main" || !config.CreateIfMissing || config.OutputVariable != "session.id" || w.Graph.Nodes[1].Type != "browser.session_ensure" || len(w.Graph.Edges) != 7 {
		t.Fatal(w, config)
	}
}

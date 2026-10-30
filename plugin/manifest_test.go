package plugin

import (
	"encoding/json"
	"testing"
	"time"
)

func testManifest(t *testing.T) Manifest {
	t.Helper()
	node, err := Describe(
		NodeMetadata{Type: "example.title", Version: 1, DisplayName: "Title", Category: "example", SecurityLevel: "sensitive", Idempotency: "idempotent"},
		Object(nil), Object(map[string]Schema{"title": String("Title")}, "title"),
		NodePolicy{RequiredPermissions: []string{PermissionBrowserRead}, Timeouts: NodeTimeout{Default: time.Second, Minimum: time.Second, Maximum: time.Minute}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return Manifest{ID: "example", Name: "Example", Version: "1.0.0", Kind: KindAutomation, PluginAPIVersion: "1", Entrypoint: "bin/plugin", Permissions: []string{PermissionBrowserRead}, Nodes: []NodeDescriptor{node}}
}

func TestManifestRoundTrip(t *testing.T) {
	value := testManifest(t)
	data, err := value.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	if decoded.Nodes[0].Policy.Timeouts.Default != time.Second {
		t.Fatal("timeout changed on the wire")
	}
}

func TestManifestRejectsInvalidDeclarations(t *testing.T) {
	for name, mutate := range map[string]func(*Manifest){
		"missing-id":                    func(m *Manifest) { m.ID = "" },
		"missing-entrypoint":            func(m *Manifest) { m.Entrypoint = "" },
		"unsupported-protocol":          func(m *Manifest) { m.PluginAPIVersion = "2" },
		"duplicate-permission":          func(m *Manifest) { m.Permissions = append(m.Permissions, PermissionBrowserRead) },
		"undeclared-permission":         func(m *Manifest) { m.Permissions = nil },
		"safe-with-permission":          func(m *Manifest) { m.Nodes[0].Metadata.SecurityLevel = "safe" },
		"restricted-without-permission": func(m *Manifest) { m.Nodes[0].Policy.RequiredPermissions = nil },
		"duplicate-node":                func(m *Manifest) { m.Nodes = append(m.Nodes, m.Nodes[0]) },
		"invalid-schema":                func(m *Manifest) { m.Nodes[0].Schema.Input = json.RawMessage(`{`) },
	} {
		t.Run(name, func(t *testing.T) {
			value := testManifest(t)
			mutate(&value)
			if _, err := value.Marshal(); err == nil {
				t.Fatal("invalid manifest was generated")
			}
		})
	}
}

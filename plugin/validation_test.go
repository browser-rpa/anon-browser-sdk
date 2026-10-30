package plugin

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNodeValidationReportsNodeAndField(t *testing.T) {
	tests := []struct {
		name, field string
		mutate      func(*NodeDescriptor)
	}{
		{"type", "metadata.type", func(n *NodeDescriptor) { n.Metadata.Type = "Title" }},
		{"reserved", "reserved namespace", func(n *NodeDescriptor) { n.Metadata.Type = "browser.custom" }},
		{"version", "metadata.version", func(n *NodeDescriptor) { n.Metadata.Version = 0 }},
		{"title", "metadata.display_name", func(n *NodeDescriptor) { n.Metadata.DisplayName = " " }},
		{"category", "metadata.category", func(n *NodeDescriptor) { n.Metadata.Category = "" }},
		{"security", "metadata.security_level", func(n *NodeDescriptor) { n.Metadata.SecurityLevel = "unknown" }},
		{"idempotency", "metadata.idempotency", func(n *NodeDescriptor) { n.Metadata.Idempotency = "unknown" }},
		{"schema-version", "schema.version", func(n *NodeDescriptor) { n.Schema.Version = 0 }},
		{"input", "schema.input_schema", func(n *NodeDescriptor) { n.Schema.Input = json.RawMessage(`null`) }},
		{"output", "schema.output_schema", func(n *NodeDescriptor) { n.Schema.Output = json.RawMessage(`[]`) }},
		{"event", "schema.event_schema", func(n *NodeDescriptor) { n.Schema.Event = json.RawMessage(`{}`) }},
		{"duplicate-port", "schema.ports", func(n *NodeDescriptor) { n.Schema.Ports = append(n.Schema.Ports, n.Schema.Ports[0]) }},
		{"two-inputs", "schema.ports", func(n *NodeDescriptor) {
			n.Schema.Ports = append(n.Schema.Ports, NodePort{Name: "second", Kind: "input"})
		}},
		{"port-kind", "schema.ports", func(n *NodeDescriptor) { n.Schema.Ports[0].Kind = "unknown" }},
		{"port-whitespace", "schema.ports", func(n *NodeDescriptor) { n.Schema.Ports[0].Name = " in" }},
		{"permission", "policy.required_permissions", func(n *NodeDescriptor) { n.Policy.RequiredPermissions = []string{"future.unlisted"} }},
		{"duplicate-permission", "policy.required_permissions", func(n *NodeDescriptor) {
			n.Policy.RequiredPermissions = []string{PermissionBrowserRead, PermissionBrowserRead}
		}},
		{"negative-timeout", "policy.timeouts", func(n *NodeDescriptor) { n.Policy.Timeouts.Minimum = -time.Second }},
		{"default-timeout", "policy.timeouts", func(n *NodeDescriptor) { n.Policy.Timeouts.Default = 2 * time.Minute }},
		{"attempts", "policy.max_attempts", func(n *NodeDescriptor) { n.Policy.MaxAttempts = 11 }},
		{"retry-delay", "policy retry", func(n *NodeDescriptor) { n.Policy.RetryBackoff = 2 * time.Second; n.Policy.RetryMaxDelay = time.Second }},
		{"resource-delay", "policy resource", func(n *NodeDescriptor) { n.Policy.ResourceRetryBackoff = -time.Second }},
		{"resource-lease", "policy.resource_wait_timeout", func(n *NodeDescriptor) {
			n.Policy.ResourceWaitTimeout = time.Second
			n.Policy.ResourceRetryBackoff = time.Millisecond
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testManifest(t)
			tt.mutate(&m.Nodes[0])
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), "node "+m.Nodes[0].Metadata.Type+"@") || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("want node and field %q, got %v", tt.field, err)
			}
		})
	}
}

func TestDangerousPermissionsRequireHighRisk(t *testing.T) {
	for _, permission := range []string{PermissionNetworkFetch, PermissionBrowserScript, "network.modify"} {
		t.Run(permission, func(t *testing.T) {
			m := testManifest(t)
			m.Permissions = []string{permission}
			m.Nodes[0].Policy.RequiredPermissions = []string{permission}
			if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "high_risk") {
				t.Fatalf("dangerous permission accepted without high_risk: %v", err)
			}
			m.Nodes[0].Metadata.SecurityLevel = "high_risk"
			if err := m.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

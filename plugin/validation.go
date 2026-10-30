package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var nodeTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

// ValidateNodes validates the public node contract without starting a plugin.
// Errors identify the node and field so authors can fix declarations before packaging.
func ValidateNodes(nodes []NodeDescriptor, permissions []string) error {
	allowed := make(map[string]bool)
	for _, permission := range permissions {
		allowed[permission] = true
	}
	seen := make(map[string]bool)
	for _, node := range nodes {
		key := fmt.Sprintf("%s@%d", node.Metadata.Type, node.Metadata.Version)
		if seen[key] {
			return fmt.Errorf("node %s: duplicate type/version", key)
		}
		seen[key] = true
		if err := validateNode(node, allowed); err != nil {
			return fmt.Errorf("node %s: %w", key, err)
		}
	}
	return nil
}

func validateNode(n NodeDescriptor, allowed map[string]bool) error {
	m, s, p := n.Metadata, n.Schema, n.Policy
	if !nodeTypePattern.MatchString(m.Type) {
		return fmt.Errorf("metadata.type must be a namespaced lowercase identifier")
	}
	// Built-in namespaces cannot be supplied by a plugin package.
	switch strings.Split(m.Type, ".")[0] {
	case "core", "browser", "environment", "background", "data", "flow", "http", "artifact", "secret", "human", "signal", "reply", "webhook":
		return fmt.Errorf("metadata.type uses reserved namespace; use your plugin namespace")
	}
	if m.Version < 1 {
		return fmt.Errorf("metadata.version must be positive")
	}
	if strings.TrimSpace(m.DisplayName) == "" || strings.TrimSpace(m.Category) == "" {
		return fmt.Errorf("metadata.display_name and metadata.category are required")
	}
	if m.SecurityLevel != "safe" && m.SecurityLevel != "sensitive" && m.SecurityLevel != "high_risk" {
		return fmt.Errorf("metadata.security_level must be safe, sensitive or high_risk")
	}
	if m.Idempotency != "idempotent" && m.Idempotency != "conditionally_idempotent" && m.Idempotency != "non_idempotent" {
		return fmt.Errorf("metadata.idempotency is invalid")
	}
	if s.Version < 1 {
		return fmt.Errorf("schema.version must be positive")
	}
	for name, raw := range map[string]json.RawMessage{"input_schema": s.Input, "output_schema": s.Output, "event_schema": s.Event} {
		if name == "event_schema" && len(raw) == 0 {
			continue
		}
		var object map[string]any
		if json.Unmarshal(raw, &object) != nil || len(object) == 0 {
			return fmt.Errorf("schema.%s must be a non-empty JSON object", name)
		}
	}
	ports := make(map[string]bool)
	inputs := 0
	for _, port := range s.Ports {
		if strings.TrimSpace(port.Name) == "" || port.Name != strings.TrimSpace(port.Name) || ports[port.Name] {
			return fmt.Errorf("schema.ports contains an empty or duplicate name %q", port.Name)
		}
		ports[port.Name] = true
		switch port.Kind {
		case "input":
			inputs++
		case "output", "branch", "error":
		default:
			return fmt.Errorf("schema.ports[%s].kind is invalid", port.Name)
		}
	}
	if inputs > 1 {
		return fmt.Errorf("schema.ports permits at most one input port")
	}
	seen := make(map[string]bool)
	for _, permission := range p.RequiredPermissions {
		if !nodeTypePattern.MatchString(permission) || seen[permission] {
			return fmt.Errorf("policy.required_permissions has invalid or duplicate permission %q", permission)
		}
		if !allowed[permission] {
			return fmt.Errorf("policy.required_permissions contains %q missing from manifest.permissions", permission)
		}
		seen[permission] = true
		if (permission == PermissionNetworkFetch || permission == PermissionBrowserScript || permission == "network.modify") && m.SecurityLevel != "high_risk" {
			return fmt.Errorf("permission %q requires metadata.security_level high_risk", permission)
		}
	}
	if m.SecurityLevel == "safe" && len(seen) > 0 {
		return fmt.Errorf("safe nodes cannot declare restricted permissions")
	}
	if m.SecurityLevel != "safe" && len(seen) == 0 {
		return fmt.Errorf("restricted nodes must declare policy.required_permissions")
	}
	if p.Timeouts.Minimum < 0 || p.Timeouts.Default < p.Timeouts.Minimum || p.Timeouts.Maximum < p.Timeouts.Default {
		return fmt.Errorf("policy.timeouts must satisfy 0 <= minimum <= default <= maximum")
	}
	if p.MaxAttempts < 0 || p.MaxAttempts > 10 {
		return fmt.Errorf("policy.max_attempts must be between 0 and 10")
	}
	if p.RetryBackoff < 0 || p.RetryMaxDelay < 0 || (p.RetryMaxDelay > 0 && p.RetryMaxDelay < p.RetryBackoff) {
		return fmt.Errorf("policy retry delays must satisfy 0 <= backoff <= max delay")
	}
	if p.ResourceWaitTimeout < 0 || p.ResourceRetryBackoff < 0 || p.ResourceRetryMaxDelay < 0 || (p.ResourceRetryMaxDelay > 0 && p.ResourceRetryMaxDelay < p.ResourceRetryBackoff) {
		return fmt.Errorf("policy resource retry delays are invalid")
	}
	if p.ResourceWaitTimeout > 0 && (p.ResourceRetryBackoff <= 0 || !p.Resources.ProfileLease) {
		return fmt.Errorf("policy.resource_wait_timeout requires profile_lease and positive resource_retry_backoff")
	}
	return nil
}

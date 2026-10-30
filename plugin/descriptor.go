package plugin

import (
	"encoding/json"
	"time"
)

type NodeDescriptor struct {
	Metadata NodeMetadata `json:"metadata"`
	Schema   NodeSchema   `json:"schema"`
	Policy   NodePolicy   `json:"policy"`
}

type NodeMetadata struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	Category      string `json:"category"`
	SecurityLevel string `json:"security_level"`
	Idempotency   string `json:"idempotency"`
	DocsURL       string `json:"docs_url,omitempty"`
}

type NodePort struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
}

type NodeSchema struct {
	Version int             `json:"version"`
	Input   json.RawMessage `json:"input_schema"`
	Output  json.RawMessage `json:"output_schema"`
	Event   json.RawMessage `json:"event_schema,omitempty"`
	Ports   []NodePort      `json:"ports"`
	Errors  []NodeError     `json:"errors,omitempty"`
}

type NodeError struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Retryable   bool   `json:"retryable"`
}

type NodeTimeout struct {
	Default time.Duration `json:"default"`
	Minimum time.Duration `json:"minimum"`
	Maximum time.Duration `json:"maximum"`
}

type NodeResources struct {
	ProfileLease bool `json:"profile_lease,omitempty"`
	Runtime      bool `json:"runtime,omitempty"`
	ActivePage   bool `json:"active_page,omitempty"`
	ArtifactIO   bool `json:"artifact_io,omitempty"`
}

type NodePolicy struct {
	Capabilities          []string      `json:"capabilities,omitempty"`
	RequiredPermissions   []string      `json:"required_permissions,omitempty"`
	Timeouts              NodeTimeout   `json:"timeouts"`
	Resources             NodeResources `json:"resources"`
	RetryableErrors       []string      `json:"retryable_errors,omitempty"`
	MaxAttempts           int           `json:"max_attempts,omitempty"`
	RetryBackoff          time.Duration `json:"retry_backoff,omitempty"`
	RetryMaxDelay         time.Duration `json:"retry_max_delay,omitempty"`
	ResourceWaitTimeout   time.Duration `json:"resource_wait_timeout,omitempty"`
	ResourceRetryBackoff  time.Duration `json:"resource_retry_backoff,omitempty"`
	ResourceRetryMaxDelay time.Duration `json:"resource_retry_max_delay,omitempty"`
}

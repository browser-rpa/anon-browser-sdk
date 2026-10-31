package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Kind string

const KindAutomation Kind = "automation"

const (
	PermissionBrowserRead           = "browser.read"
	PermissionBrowserNavigate       = "browser.navigate"
	PermissionBrowserInteract       = "browser.interact"
	PermissionBrowserTabs           = "browser.tabs"
	PermissionBrowserScript         = "browser.script"
	PermissionBrowserFileUpload     = "browser.file_upload"
	PermissionBrowserNetworkObserve = "browser.network.observe"
	PermissionBrowserPageManage     = "browser.page.manage"
	PermissionStorageRead           = "storage.read"
	PermissionStorageWrite          = "storage.write"
	PermissionNetworkFetch          = "network.fetch"
	PermissionVariablesRead         = "variables.read"
	PermissionVariablesWrite        = "variables.write"
	PermissionArtifactsWrite        = "run.artifacts.write"
)

// Manifest is the public, JSON-serializable portion of an automation package.
// Installation and authorization are managed by the host, not by this manifest.
type Manifest struct {
	ID                         string            `json:"id"`
	Name                       string            `json:"name"`
	Version                    string            `json:"version"`
	Kind                       Kind              `json:"kind"`
	PluginAPIVersion           string            `json:"plugin_api_version,omitempty"`
	Entrypoint                 string            `json:"entrypoint,omitempty"`
	Entrypoints                map[string]string `json:"entrypoints,omitempty"`
	Permissions                []string          `json:"permissions,omitempty"`
	SupportedBrowserFamilies   []string          `json:"supported_browser_families,omitempty"`
	SupportedBrowserVersions   []string          `json:"supported_browser_versions,omitempty"`
	SupportedIdentityTemplates []string          `json:"supported_identity_templates,omitempty"`
	Nodes                      []NodeDescriptor  `json:"nodes,omitempty"`
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("manifest id, name and version are required")
	}
	if m.Kind != "" && m.Kind != KindAutomation {
		return fmt.Errorf("manifest kind must be automation")
	}
	if m.PluginAPIVersion != "" && m.PluginAPIVersion != "1" {
		return fmt.Errorf("unsupported plugin_api_version %q", m.PluginAPIVersion)
	}
	if m.Entrypoint == "" && len(m.Entrypoints) == 0 {
		return fmt.Errorf("manifest entrypoint is required")
	}
	seen := map[string]bool{}
	for _, p := range m.Permissions {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return fmt.Errorf("invalid or duplicate permission %q", p)
		}
		seen[p] = true
	}
	return ValidateNodes(m.Nodes, m.Permissions)
}

func (m Manifest) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(m, "", "  ")
}
func (m Manifest) WriteFile(path string) error {
	data, err := m.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

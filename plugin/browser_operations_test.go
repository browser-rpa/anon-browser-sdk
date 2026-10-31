package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestManagedBrowserClientWireContract(t *testing.T) {
	wait := WaitResponseRequest{Watch: ResponseWatch{URLPattern: "https://example.test/publish", Method: "POST"}, Action: ResponseAction{Action: "click", Selector: "#publish"}, Fields: []ResponseField{{Name: "id", Path: "/responses/0/body/id"}}, Checks: []ResponseCheck{{Name: "accepted", Path: "/responses/0/body/code", Equals: 0}}}
	for _, test := range []struct {
		action string
		run    func(context.Context, *Client) error
		data   string
	}{
		{"file_upload", func(ctx context.Context, c *Client) error {
			r, err := c.UploadFiles(ctx, UploadRequest{Selector: "#file", Origin: "https://example.test", Files: []MaterialRef{{Ref: "artifact://material-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}})
			if err == nil && r.State != "selected" {
				return errors.New("wrong upload result")
			}
			return err
		}, `{"state":"selected","count":1,"files":[]}`},
		{"wait_for_response", func(ctx context.Context, c *Client) error {
			r, err := c.WaitForResponse(ctx, wait)
			if err == nil && (!r.Checks["accepted"] || r.SecretRef != "run-secret://run/response") {
				return errors.New("wrong response result")
			}
			return err
		}, `{"outcome":"response","matched":1,"secret_ref":"run-secret://run/response","checks":{"accepted":true}}`},
		{"page_bind", func(ctx context.Context, c *Client) error {
			return c.BindPage(ctx, PageBindingRequest{PageKey: "compose", Origin: "https://example.test", GuardSelector: "#workspace", ExpectedText: "Workspace"})
		}, `{}`},
		{"page_fill", func(ctx context.Context, c *Client) error {
			return c.FillBoundPage(ctx, "compose", "#title", "  keep spaces  ")
		}, `{}`},
		{"page_click", func(ctx context.Context, c *Client) error { return c.ClickBoundPage(ctx, "compose", "#publish") }, `{}`},
		{"page_release", func(ctx context.Context, c *Client) error { return c.ReleasePage(ctx, "compose") }, `{}`},
		{"page_wait_response", func(ctx context.Context, c *Client) error {
			_, err := c.WaitForBoundPageResponse(ctx, "compose", wait)
			return err
		}, `{"outcome":"timeout","matched":0}`},
	} {
		t.Run(test.action, func(t *testing.T) {
			server := NewServer()
			_ = server.Register("test.echo", 1, func(ctx context.Context, call NodeCall) (NodeResult, error) {
				err := test.run(ctx, call.Client)
				return NodeResult{NextPort: "success"}, err
			})
			var out bytes.Buffer
			input := invocationLine() + fmt.Sprintf("{\"type\":\"capability_result\",\"id\":\"call-1\",\"ok\":true,\"data\":%s}\n", test.data)
			if err := server.Serve(context.Background(), strings.NewReader(input), &out); err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(&out)
			var call CapabilityCall
			if err := decoder.Decode(&call); err != nil || call.Capability != "browser" || call.Action != test.action {
				t.Fatal(call, err)
			}
			if test.action == "page_fill" && call.Arguments["text"] != "  keep spaces  " {
				t.Fatal("whitespace lost")
			}
			if test.action == "page_wait_response" && call.Arguments["page_key"] != "compose" {
				t.Fatal("binding lost")
			}
			var result Response
			if err := decoder.Decode(&result); err != nil || !result.OK {
				t.Fatal(result, err)
			}
		})
	}
}

func TestManagedOperationManifestsRejectUnsafeRetriesAndResources(t *testing.T) {
	for _, permission := range []string{PermissionBrowserFileUpload, PermissionBrowserNetworkObserve, PermissionBrowserPageManage} {
		makeManifest := func() Manifest {
			m := testManifest(t)
			m.Permissions = []string{permission}
			m.Nodes[0].Policy.RequiredPermissions = m.Permissions
			m.Nodes[0].Metadata.SecurityLevel = "high_risk"
			m.Nodes[0].Metadata.Idempotency = "non_idempotent"
			m.Nodes[0].Policy.Resources = NodeResources{ProfileLease: true, Runtime: true, ActivePage: true, ArtifactIO: true}
			return m
		}
		m := makeManifest()
		if err := m.Validate(); err != nil {
			t.Fatal(err)
		}
		for _, mutate := range []func(*NodeDescriptor){
			func(n *NodeDescriptor) { n.Policy.MaxAttempts = 2 },
			func(n *NodeDescriptor) { n.Metadata.Idempotency = "idempotent" },
			func(n *NodeDescriptor) { n.Policy.Resources.Runtime = false },
			func(n *NodeDescriptor) { n.Policy.Resources.ProfileLease = false },
		} {
			m := makeManifest()
			mutate(&m.Nodes[0])
			if m.Validate() == nil {
				t.Fatal("unsafe manifest accepted", permission)
			}
		}
		if permission == PermissionBrowserPageManage {
			m := makeManifest()
			m.Nodes[0].Metadata.SecurityLevel = "sensitive"
			if m.Validate() == nil {
				t.Fatal("page manage requires high risk")
			}
		}
		if permission == PermissionBrowserFileUpload {
			m := makeManifest()
			m.Nodes[0].Policy.Resources.ArtifactIO = false
			if m.Validate() == nil {
				t.Fatal("upload requires artifact IO")
			}
		}
	}
}

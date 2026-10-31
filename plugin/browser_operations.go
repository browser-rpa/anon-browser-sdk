package plugin

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/browser-rpa/anon-browser-sdk/workflow"
)

// The workflow builder and runtime client share the same response/filter wire
// contract. Neither API exposes browser endpoints or local filesystem paths.
type ResponseWatch = workflow.ResponseWatch
type ResponseAction = workflow.ResponseAction
type MaterialRef = workflow.MaterialRef
type SecretRef = workflow.SecretRef

type ResponseField struct {
	Name string `json:"name"`
	Path string `json:"path"` // JSON pointer, e.g. /responses/0/body/id.
}

// ResponseCheck compares a scalar inside the host, without exposing response
// bodies, headers or cookies to plugin logs. Missing values never match,
// including when Equals is nil (which checks a present JSON null).
type ResponseCheck struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Equals any    `json:"equals"`
}

type WaitResponseRequest struct {
	Watch  ResponseWatch   `json:"watch"`
	Action ResponseAction  `json:"action"`
	Fields []ResponseField `json:"fields,omitempty"`
	Checks []ResponseCheck `json:"checks,omitempty"`
}

// ResponseResult reports transport evidence, NOT business success. Fields
// contain metadata or {secret_ref:...}; raw bodies/headers/cookies stay encrypted.
// Checks can test a platform business code without disclosing its response.
type ResponseResult struct {
	Outcome       string          `json:"outcome"`
	Matched       int             `json:"matched"`
	Rejected      int             `json:"rejected"`
	Status        *int64          `json:"status,omitempty"`
	SecretRef     string          `json:"secret_ref"`
	Fields        map[string]any  `json:"fields,omitempty"`
	Checks        map[string]bool `json:"checks,omitempty"`
	MissingFields []string        `json:"missing_fields,omitempty"`
	MissingChecks []string        `json:"missing_checks,omitempty"`
}

type UploadRequest struct {
	Selector string          `json:"selector"`
	Origin   string          `json:"origin"`
	Files    []MaterialRef   `json:"files"` // Must be authorized in this Run's input/graph.
	Multiple bool            `json:"multiple,omitempty"`
	Watch    *ResponseWatch  `json:"watch,omitempty"`
	Fields   []ResponseField `json:"fields,omitempty"`
	Checks   []ResponseCheck `json:"checks,omitempty"`
}

type UploadResult struct {
	State    string          `json:"state"` // selected, response_received or unconfirmed; NOT published.
	Count    int             `json:"count"`
	Files    []MaterialRef   `json:"files"`
	Response *ResponseResult `json:"response,omitempty"`
}

type PageBindingRequest struct {
	PageKey       string `json:"page_key"`
	Origin        string `json:"origin"`
	GuardSelector string `json:"guard_selector"`
	ExpectedText  string `json:"expected_text"`
}

func browserCallAs[T any](ctx context.Context, c *Client, action string, request any) (T, error) {
	var zero T
	raw, err := json.Marshal(request)
	if err != nil {
		return zero, err
	}
	var args map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&args); err != nil {
		return zero, err
	}
	return callAs[T](ctx, c, "browser", action, args)
}

func (c *Client) UploadFiles(ctx context.Context, request UploadRequest) (UploadResult, error) {
	return browserCallAs[UploadResult](ctx, c, "file_upload", request)
}

// WaitForResponse arms observation BEFORE the associated action. It never
// repeats the action automatically, even on timeout or an unknown outcome.
func (c *Client) WaitForResponse(ctx context.Context, request WaitResponseRequest) (ResponseResult, error) {
	return browserCallAs[ResponseResult](ctx, c, "wait_for_response", request)
}

// BindPage pins the current top-level document within this Run. Navigation,
// release or session handoff invalidates it; it never silently recaptures a tab.
func (c *Client) BindPage(ctx context.Context, request PageBindingRequest) error {
	_, err := browserCallAs[map[string]any](ctx, c, "page_bind", request)
	return err
}

// ClickBoundPage and FillBoundPage dispatch DOM events, NOT trusted input.
// Success only confirms dispatch; verify business results separately.
func (c *Client) ClickBoundPage(ctx context.Context, pageKey, selector string) error {
	_, err := c.Call(ctx, "browser", "page_click", map[string]any{"page_key": pageKey, "selector": selector})
	return err
}
func (c *Client) FillBoundPage(ctx context.Context, pageKey, selector, text string) error {
	_, err := c.Call(ctx, "browser", "page_fill", map[string]any{"page_key": pageKey, "selector": selector, "text": text})
	return err
}
func (c *Client) ReleasePage(ctx context.Context, pageKey string) error {
	_, err := c.Call(ctx, "browser", "page_release", map[string]any{"page_key": pageKey})
	return err
}

// WaitForBoundPageResponse only accepts a DOM click on the existing binding.
// It does not navigate or follow the active tab.
func (c *Client) WaitForBoundPageResponse(ctx context.Context, pageKey string, request WaitResponseRequest) (ResponseResult, error) {
	return browserCallAs[ResponseResult](ctx, c, "page_wait_response", struct {
		WaitResponseRequest
		PageKey string `json:"page_key"`
	}{request, pageKey})
}

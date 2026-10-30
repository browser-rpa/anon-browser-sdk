package plugin

import (
	"context"
	"encoding/json"
)

// Typed helpers use the existing broker; they do not add permissions or direct
// browser access. Host errors are returned without retrying an action.
func callAs[T any](ctx context.Context, c *Client, capability, action string, args map[string]any) (T, error) {
	var result T
	value, err := c.Call(ctx, capability, action, args)
	if err != nil {
		return result, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}
func (c *Client) ReadTitle(ctx context.Context) (string, error) {
	return callAs[string](ctx, c, "browser", "read_title", nil)
}
func (c *Client) ReadURL(ctx context.Context) (string, error) {
	return callAs[string](ctx, c, "browser", "read_url", nil)
}
func (c *Client) ReadText(ctx context.Context, selector string) (string, error) {
	return callAs[string](ctx, c, "browser", "read_text", map[string]any{"selector": selector})
}
func (c *Client) Navigate(ctx context.Context, url string) error {
	_, err := c.Call(ctx, "browser", "navigate", map[string]any{"url": url})
	return err
}
func (c *Client) Click(ctx context.Context, selector string) error {
	_, err := c.Call(ctx, "browser", "click", map[string]any{"selector": selector})
	return err
}
func (c *Client) TypeText(ctx context.Context, selector, text string) error {
	_, err := c.Call(ctx, "browser", "type", map[string]any{"selector": selector, "text": text})
	return err
}
func (c *Client) Evaluate(ctx context.Context, expression string) (any, error) {
	return c.Call(ctx, "browser", "evaluate", map[string]any{"expression": expression})
}

type VariableResult struct {
	Value any  `json:"value"`
	Found bool `json:"found"`
}

func (c *Client) GetVariable(ctx context.Context, name string) (VariableResult, error) {
	return callAs[VariableResult](ctx, c, "variables", "get", map[string]any{"name": name})
}
func (c *Client) SetVariable(ctx context.Context, name string, value any) error {
	_, err := c.Call(ctx, "variables", "set", map[string]any{"name": name, "value": value})
	return err
}

type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}
type HTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
}

func (c *Client) HTTPRequest(ctx context.Context, req HTTPRequest) (HTTPResponse, error) {
	args := map[string]any{"method": req.Method, "url": req.URL, "headers": req.Headers}
	if req.Body != nil {
		args["body"] = req.Body
	}
	return callAs[HTTPResponse](ctx, c, "http", "request", args)
}
func (c *Client) GetStorage(ctx context.Context, origin, storageType string, keys []string) (map[string]string, error) {
	return callAs[map[string]string](ctx, c, "storage", "get", map[string]any{"origin": origin, "storage_type": storageType, "keys": keys})
}
func (c *Client) SetStorage(ctx context.Context, origin, storageType string, items map[string]string) error {
	_, err := c.Call(ctx, "storage", "set", map[string]any{"origin": origin, "storage_type": storageType, "items": items})
	return err
}
func (c *Client) WriteTextArtifact(ctx context.Context, name, contentType, text string) (ArtifactRef, error) {
	return callAs[ArtifactRef](ctx, c, "artifacts", "write", map[string]any{"name": name, "content_type": contentType, "text": text})
}

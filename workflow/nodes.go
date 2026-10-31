package workflow

// Ref and Secret are explicit wire values, not string interpolation. They can
// be used only in fields whose live Catalog schema permits such references.
type VariableRef struct {
	Name string `json:"variable_ref"`
}
type SecretRef struct {
	Ref string `json:"secret_ref"`
}

func Ref(name string) VariableRef { return VariableRef{Name: name} }
func Secret(ref string) SecretRef { return SecretRef{Ref: ref} }
func Start(id string) Spec        { return N(id, "core.start", struct{}{}) }
func End(id string) Spec          { return N(id, "core.end", struct{}{}) }
func Open(id string, headless bool) Spec {
	return N(id, "browser.open", struct {
		Headless bool `json:"headless"`
	}{headless})
}
func Navigate(id string, url any) Spec {
	return N(id, "browser.navigate", struct {
		URL any `json:"url"`
	}{url})
}
func Click(id, selector string) Spec {
	return N(id, "browser.click", struct {
		Selector string `json:"selector"`
	}{selector})
}
func TypeText(id, selector string, text any) Spec {
	return N(id, "browser.type", struct {
		Selector string `json:"selector"`
		Text     any    `json:"text"`
	}{selector, text})
}
func Evaluate(id, expression, output string) Spec {
	return N(id, "browser.evaluate", struct {
		Expression string `json:"expression"`
		Output     string `json:"output_variable,omitempty"`
	}{expression, output})
}
func SetVariable(id, name string, value any) Spec {
	return N(id, "data.set", struct {
		Name  string `json:"name"`
		Value any    `json:"value"`
	}{name, value})
}
func ReadText(id, selector, output string) Spec {
	return N(id, "browser.read_text", struct {
		Selector string `json:"selector"`
		Output   string `json:"output_variable"`
	}{selector, output})
}
func Wait(id string, timeoutMS int) Spec {
	return N(id, "flow.wait", struct {
		Timeout int `json:"timeout_ms"`
	}{timeoutMS})
}
func KeepSession(id, name, output string) Spec {
	return N(id, "browser.session_keep", struct {
		Name   string `json:"name"`
		Output string `json:"output_variable,omitempty"`
	}{name, output})
}
func BindSession(id string, session any) Spec {
	return N(id, "browser.session_bind", struct {
		Session any `json:"session_id"`
	}{session})
}

// EnsureSessionConfig selects a project-local stable session key. It does not
// grant plugins session-management authority; the workflow owns this decision.
type EnsureSessionConfig struct {
	SessionKey      any    `json:"session_key"`
	CreateIfMissing bool   `json:"create_if_missing"`
	Name            string `json:"name,omitempty"`
	OutputVariable  string `json:"output_variable,omitempty"`
}

// EnsureSession has created/reused/missing/busy/needs_review/closed ports, not
// success. Use Link for all branches. Unavailable outcomes clear OutputVariable.
func EnsureSession(id string, config EnsureSessionConfig) Spec {
	return N(id, "browser.session_ensure", config)
}

type WaitCondition struct {
	Kind     string `json:"kind"`
	Selector string `json:"selector,omitempty"`
	Expected string `json:"expected,omitempty"`
	URL      string `json:"url,omitempty"`
	Name     string `json:"name,omitempty"`
}
type HumanWaitConfig struct {
	Mode           string          `json:"mode,omitempty"`
	Title          string          `json:"title,omitempty"`
	TimeoutMS      int64           `json:"timeout_ms,omitempty"`
	PollIntervalMS int64           `json:"poll_interval_ms,omitempty"`
	StableMS       int64           `json:"stable_ms,omitempty"`
	Match          string          `json:"match,omitempty"`
	Conditions     []WaitCondition `json:"conditions,omitempty"`
	OutputVariable string          `json:"output_variable,omitempty"`
}

func HumanWait(id string, config HumanWaitConfig) Spec { return N(id, "human.wait", config) }

type ScriptConfig struct {
	Key               string `json:"key"`
	Origin            string `json:"origin"`
	Source            string `json:"source"`
	RunAt             string `json:"run_at"`
	Scope             string `json:"scope"`
	ConnectorID       string `json:"connector_id,omitempty"`
	ConnectorRevision int64  `json:"connector_revision,omitempty"`
}

func RegisterScript(id string, config ScriptConfig) Spec {
	return N(id, "browser.script_register", config)
}
func UnregisterScript(id, key string) Spec {
	return N(id, "browser.script_unregister", struct {
		Key string `json:"key"`
	}{key})
}

type SignalField struct {
	Name     string            `json:"name"`
	Type     WorkflowInputType `json:"type"`
	Required bool              `json:"required"`
	Variable string            `json:"variable"`
}
type SignalWaitConfig struct {
	SignalName     string        `json:"signal_name"`
	OutputVariable string        `json:"output_variable"`
	TimeoutMS      int64         `json:"timeout_ms,omitempty"`
	Fields         []SignalField `json:"fields,omitempty"`
}

func WaitSignal(id string, config SignalWaitConfig) Spec { return N(id, "signal.wait", config) }
func CompleteSignal(id string, receipt any, outcome string) Spec {
	return N(id, "signal.complete", struct {
		Receipt any    `json:"receipt"`
		Outcome string `json:"outcome"`
	}{receipt, outcome})
}

type ReplyRequestConfig struct {
	SignalName     string         `json:"signal_name"`
	MessageKey     any            `json:"message_key"`
	Context        map[string]any `json:"context"`
	TimeoutMS      int64          `json:"timeout_ms,omitempty"`
	OutputVariable string         `json:"output_variable"`
}

func RequestReply(id string, config ReplyRequestConfig) Spec { return N(id, "reply.request", config) }
func Webhook(id, url, signingRef string, body any) Spec {
	return N(id, "webhook.emit", struct {
		URL     string    `json:"url"`
		Signing SecretRef `json:"signing_secret"`
		Body    any       `json:"body"`
	}{url, Secret(signingRef), body})
}

type CallConfig struct {
	SubflowID string            `json:"subflow_id"`
	Inputs    map[string]any    `json:"inputs,omitempty"`
	Outputs   map[string]string `json:"outputs,omitempty"`
}

func Call(id string, config CallConfig) Spec { return N(id, "flow.call", config) }

type ResponseWatch struct {
	URLPattern    string `json:"url_pattern"`
	URLMatch      string `json:"url_match,omitempty"`
	Method        string `json:"method,omitempty"`
	ResourceType  string `json:"resource_type,omitempty"`
	FrameID       string `json:"frame_id,omitempty"`
	Selection     string `json:"selection,omitempty"`
	Nth           int    `json:"nth,omitempty"`
	ExpectedCount int    `json:"expected_count,omitempty"`
	WindowMS      int    `json:"window_ms,omitempty"`
	QuietMS       int    `json:"quiet_ms,omitempty"`
	TimeoutMS     int    `json:"timeout_ms,omitempty"`
	MaxRequests   int    `json:"max_requests,omitempty"`
	MaxBodyBytes  int    `json:"max_body_bytes,omitempty"`
	MaxTotalBytes int    `json:"max_total_bytes,omitempty"`
	BodyMode      string `json:"body_mode,omitempty"`
	StatusEquals  int    `json:"status_equals,omitempty"`
}
type ResponseAction struct {
	Action   string `json:"action"`
	Selector string `json:"selector,omitempty"`
	Key      string `json:"key,omitempty"`
	URL      string `json:"url,omitempty"`
}
type ExtractField struct {
	Path     string `json:"path"`
	Variable string `json:"variable"`
}
type WaitResponseConfig struct {
	Watch          ResponseWatch  `json:"watch"`
	Action         ResponseAction `json:"action"`
	OutputVariable string         `json:"output_variable,omitempty"`
	Extract        []ExtractField `json:"extract,omitempty"`
}

func WaitResponse(id string, config WaitResponseConfig) Spec {
	return N(id, "browser.wait_for_response", config)
}

type MaterialRef struct {
	Ref string `json:"artifact_ref"`
}

func Material(ref string) MaterialRef { return MaterialRef{Ref: ref} }

type UploadConfig struct {
	Selector               string         `json:"selector"`
	Origin                 string         `json:"origin"`
	Files                  any            `json:"files"` // []MaterialRef or VariableRef, never a local path/CDN URL.
	Multiple               bool           `json:"multiple,omitempty"`
	Watch                  *ResponseWatch `json:"watch,omitempty"`
	OutputVariable         string         `json:"output_variable,omitempty"`
	ResponseOutputVariable string         `json:"response_output_variable,omitempty"`
	Extract                []ExtractField `json:"extract,omitempty"`
}

func Upload(id string, config UploadConfig) Spec { return N(id, "browser.file_upload", config) }

type DownloadConfig struct {
	Selector        string `json:"selector"`
	Origin          string `json:"origin"`
	URLPattern      string `json:"url_pattern"`
	FilenamePattern string `json:"filename_pattern"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	MaxBytes        int64  `json:"max_bytes,omitempty"`
	OutputVariable  string `json:"output_variable,omitempty"`
}

func Download(id string, config DownloadConfig) Spec { return N(id, "browser.download", config) }

package native

import "encoding/json"

type TurnKind string

const (
	TurnUser        TurnKind = "user"
	TurnToolResults TurnKind = "tool_results"
)

type Turn struct {
	Kind TurnKind
	Text string
}

type Request struct {
	System        string
	ToolsJSON     []byte
	ToolNames     []string
	Turns         []Turn
	MaxNewTokens  int
	ToolIndexPath string
}

type Envelope struct {
	Type          string         `json:"type"`
	Success       bool           `json:"success"`
	Error         any            `json:"error"`
	ErrorCode     any            `json:"error_code"`
	FunctionCalls []FunctionCall `json:"function_calls"`
	Reasoning     string         `json:"reasoning"`
	Confidence    *float64       `json:"confidence"`
	Validation    Validation     `json:"validation"`
	PrefillTPS    float64        `json:"prefill_tps"`
	DecodeTPS     float64        `json:"decode_tps"`
	PeakRAMMB     float64        `json:"peak_ram_mb"`
}

type FunctionCall struct {
	Name      string                     `json:"name"`
	Arguments map[string]json.RawMessage `json:"arguments"`
}

type Validation struct {
	Ungrounded []string `json:"ungrounded"`
	Negation   bool     `json:"negation"`
}

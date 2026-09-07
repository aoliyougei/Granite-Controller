package needle

import "encoding/json"

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Tools     []tool        `json:"tools"`
	MaxTokens int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type tool struct {
	Type     string       `json:"type"`
	Function functionTool `json:"function"`
}

type functionTool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  parameters `json:"parameters"`
}

type parameters struct {
	Type                 string              `json:"type"`
	Properties           map[string]property `json:"properties"`
	Required             []string            `json:"required"`
	AdditionalProperties bool                `json:"additionalProperties"`
}

type property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Safety SafetyMetadata `json:"x_needle"`
}

type Completion struct {
	ToolCalls []ToolCall
	Safety    SafetyMetadata
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"-"`
}

func (f *FunctionCall) UnmarshalJSON(data []byte) error {
	var wire struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	f.Name = wire.Name
	f.Arguments = json.RawMessage(wire.Arguments)
	return nil
}

type SafetyMetadata struct {
	Confidence *float64   `json:"confidence"`
	Validation Validation `json:"validation"`
}

type Validation struct {
	Ungrounded json.RawMessage `json:"ungrounded"`
	Negation   *bool           `json:"negation"`
}

package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

func requestWithTools(tools ...FunctionTool) ChatCompletionRequest {
	return ChatCompletionRequest{Tools: tools}
}

func functionTool(name, schema string) FunctionTool {
	return FunctionTool{Type: "function", Function: FunctionDefinition{Name: name, Description: "A tool", Parameters: json.RawMessage(schema)}}
}

func TestSchemaAcceptsSupportedKeywordsAndTypes(t *testing.T) {
	schema := `{
		"type":"object","description":"root","additionalProperties":false,
		"properties":{
			"text":{"type":"string","description":"text","enum":["a","b"],"const":"a","minLength":1,"maxLength":8,"pattern":"^[a-z]+$","format":"name"},
			"count":{"type":"integer","minimum":1,"maximum":10,"exclusiveMinimum":0,"exclusiveMaximum":11,"multipleOf":1},
			"ratio":{"type":"number","minimum":0},
			"enabled":{"type":"boolean"},
			"items":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":3,"uniqueItems":true}
		},"required":["text","count"]
	}`
	got, warnings, err := ValidateAndSelectTools(requestWithTools(functionTool("valid_tool", schema)), DefaultSchemaLimits())
	if err != nil || len(warnings) != 0 || len(got) != 1 || got[0].Name != "valid_tool" {
		t.Fatalf("tools=%+v warnings=%v error=%v", got, warnings, err)
	}
	if string(got[0].Parameters) != schema {
		t.Fatal("validated schema was rewritten")
	}
}

func TestSchemaRejectsUnsupportedOrMalformedTools(t *testing.T) {
	deep := `{"type":"array","items":{"type":"array","items":{"type":"array","items":{"type":"string"}}}}`
	manyProps := `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"}}}`
	tests := []struct {
		name   string
		req    ChatCompletionRequest
		limits SchemaLimits
	}{
		{"no tools", ChatCompletionRequest{}, DefaultSchemaLimits()},
		{"non function", requestWithTools(FunctionTool{Type: "custom", Function: FunctionDefinition{Name: "x", Parameters: json.RawMessage(`{"type":"object"}`)}}), DefaultSchemaLimits()},
		{"invalid name", requestWithTools(functionTool("bad name", `{"type":"object"}`)), DefaultSchemaLimits()},
		{"duplicate name", requestWithTools(functionTool("same", `{"type":"object"}`), functionTool("same", `{"type":"object"}`)), DefaultSchemaLimits()},
		{"description too long", requestWithTools(FunctionTool{Type: "function", Function: FunctionDefinition{Name: "x", Description: strings.Repeat("界", 3), Parameters: json.RawMessage(`{"type":"object"}`)}}), SchemaLimits{MaxTools: 64, MaxCatalogBytes: 1 << 20, MaxDescriptionRunes: 2, MaxDepth: 8, MaxProperties: 64}},
		{"unknown keyword", requestWithTools(functionTool("x", `{"type":"object","title":"no"}`)), DefaultSchemaLimits()},
		{"ref", requestWithTools(functionTool("x", `{"$ref":"#/x"}`)), DefaultSchemaLimits()},
		{"combinator", requestWithTools(functionTool("x", `{"oneOf":[]}`)), DefaultSchemaLimits()},
		{"additional true", requestWithTools(functionTool("x", `{"type":"object","additionalProperties":true}`)), DefaultSchemaLimits()},
		{"object additional", requestWithTools(functionTool("x", `{"type":"object","additionalProperties":{"type":"string"}}`)), DefaultSchemaLimits()},
		{"array missing items", requestWithTools(functionTool("x", `{"type":"array"}`)), DefaultSchemaLimits()},
		{"bad required", requestWithTools(functionTool("x", `{"type":"object","properties":{"a":{"type":"string"}},"required":["missing"]}`)), DefaultSchemaLimits()},
		{"required without properties", requestWithTools(functionTool("x", `{"type":"object","required":["a"]}`)), DefaultSchemaLimits()},
		{"too deep", requestWithTools(functionTool("x", deep)), SchemaLimits{MaxTools: 64, MaxCatalogBytes: 1 << 20, MaxDescriptionRunes: 100, MaxDepth: 2, MaxProperties: 64}},
		{"too many properties", requestWithTools(functionTool("x", manyProps)), SchemaLimits{MaxTools: 64, MaxCatalogBytes: 1 << 20, MaxDescriptionRunes: 100, MaxDepth: 8, MaxProperties: 1}},
		{"duplicate key after null", requestWithTools(functionTool("x", `{"type":null,"type":"object"}`)), DefaultSchemaLimits()},
		{"malformed", requestWithTools(functionTool("x", `{`)), DefaultSchemaLimits()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ValidateAndSelectTools(tc.req, tc.limits)
			if err == nil || err.HTTPStatus != 400 || err.Param == nil || *err.Param != "tools" {
				t.Fatalf("error=%+v", err)
			}
		})
	}
}

func TestToolChoiceSafeSubset(t *testing.T) {
	tools := []FunctionTool{functionTool("one", `{"type":"object"}`), functionTool("two", `{"type":"object"}`)}
	tests := []struct {
		name, choice          string
		count                 int
		picked, warning, code string
	}{
		{"absent", "", 2, "one", "", ""},
		{"auto", `"auto"`, 2, "one", "", ""},
		{"required", `"required"`, 2, "one", "cannot be enforced", ""},
		{"named", `{"type":"function","function":{"name":"two"}}`, 1, "two", "", ""},
		{"none", `"none"`, 0, "", "", "tools_required"},
		{"unknown", `{"type":"function","function":{"name":"missing"}}`, 0, "", "", "invalid_tool_choice"},
		{"named trailing", `{"type":"function","function":{"name":"two"}} {}`, 0, "", "", "invalid_tool_choice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := requestWithTools(tools...)
			req.ToolChoice = json.RawMessage(tc.choice)
			got, warnings, err := ValidateAndSelectTools(req, DefaultSchemaLimits())
			if tc.code != "" {
				if err == nil || err.Code != tc.code {
					t.Fatalf("error=%+v", err)
				}
				return
			}
			if err != nil || len(got) != tc.count || got[0].Name != tc.picked {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			joined := strings.Join(warnings, " ")
			if tc.warning != "" && !strings.Contains(joined, tc.warning) {
				t.Fatalf("warnings=%v", warnings)
			}
		})
	}
}

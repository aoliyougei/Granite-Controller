package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"needle-controller/internal/apierror"
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)

var allowedSchemaKeys = map[string]bool{
	"type": true, "properties": true, "required": true, "description": true,
	"enum": true, "const": true, "items": true, "minimum": true, "maximum": true,
	"exclusiveMinimum": true, "exclusiveMaximum": true, "multipleOf": true,
	"minLength": true, "maxLength": true, "pattern": true, "format": true,
	"minItems": true, "maxItems": true, "uniqueItems": true, "additionalProperties": true,
}

func ValidateAndSelectTools(req ChatCompletionRequest, limits SchemaLimits) ([]NativeTool, []string, *apierror.Error) {
	if len(req.Tools) == 0 {
		return nil, nil, toolError("tools_required", "needle-2 requires at least one function tool")
	}
	if len(req.Tools) > limits.MaxTools {
		return nil, nil, toolError("invalid_tools", "too many function tools")
	}
	seen := make(map[string]bool, len(req.Tools))
	tools := make([]NativeTool, 0, len(req.Tools))
	for _, entry := range req.Tools {
		if entry.Type != "function" || !toolNamePattern.MatchString(entry.Function.Name) || seen[entry.Function.Name] {
			return nil, nil, toolError("invalid_tools", "function tool type or name is invalid")
		}
		seen[entry.Function.Name] = true
		if utf8.RuneCountInString(entry.Function.Description) > limits.MaxDescriptionRunes {
			return nil, nil, toolError("invalid_tools", "function description is too long")
		}
		if err := validateSchema(entry.Function.Parameters, limits); err != nil {
			return nil, nil, toolError("invalid_tools", err.Error())
		}
		tools = append(tools, NativeTool{Name: entry.Function.Name, Description: entry.Function.Description, Parameters: entry.Function.Parameters})
	}
	catalog, err := json.Marshal(tools)
	if err != nil || len(catalog) > limits.MaxCatalogBytes {
		return nil, nil, toolError("invalid_tools", "tool catalog is too large")
	}
	return applyToolChoice(tools, req.ToolChoice)
}

func applyToolChoice(tools []NativeTool, raw json.RawMessage) ([]NativeTool, []string, *apierror.Error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return tools, nil, nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		switch text {
		case "auto":
			return tools, nil, nil
		case "required":
			return tools, []string{"tool_choice 'required' cannot be enforced by Needle"}, nil
		case "none":
			return nil, nil, toolError("tools_required", "needle-2 requires at least one function tool")
		default:
			return nil, nil, toolError("invalid_tool_choice", "unsupported tool_choice")
		}
	}
	var named struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&named) != nil || named.Type != "function" || named.Function.Name == "" {
		return nil, nil, toolError("invalid_tool_choice", "invalid named tool_choice")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, nil, toolError("invalid_tool_choice", "invalid named tool_choice")
	}
	for _, candidate := range tools {
		if candidate.Name == named.Function.Name {
			return []NativeTool{candidate}, nil, nil
		}
	}
	return nil, nil, toolError("invalid_tool_choice", "tool_choice names an unknown function")
}

func validateSchema(raw json.RawMessage, limits SchemaLimits) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := parseJSONValue(decoder, 1, limits)
	if err != nil {
		return err
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("function parameters must be an object schema")
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return fmt.Errorf("schema has trailing data")
	}
	return validateSchemaObject(value.(map[string]any), 1, limits)
}

func parseJSONValue(decoder *json.Decoder, depth int, limits SchemaLimits) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("invalid schema JSON")
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}
	switch delim {
	case '{':
		if depth > limits.MaxDepth {
			return nil, fmt.Errorf("schema nesting is too deep")
		}
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("invalid schema object")
			}
			key, ok := keyToken.(string)
			if !ok || object[key] != nil {
				return nil, fmt.Errorf("schema contains duplicate keys")
			}
			value, err := parseJSONValue(decoder, depth+1, limits)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if close, err := decoder.Token(); err != nil || close != json.Delim('}') {
			return nil, fmt.Errorf("invalid schema object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := parseJSONValue(decoder, depth+1, limits)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if close, err := decoder.Token(); err != nil || close != json.Delim(']') {
			return nil, fmt.Errorf("invalid schema array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("invalid schema delimiter")
	}
}

func validateSchemaObject(schema map[string]any, depth int, limits SchemaLimits) error {
	for key := range schema {
		if !allowedSchemaKeys[key] {
			return fmt.Errorf("unsupported schema keyword %q", key)
		}
	}
	typeName, ok := schema["type"].(string)
	if !ok || !map[string]bool{"object": true, "array": true, "string": true, "integer": true, "number": true, "boolean": true}[typeName] {
		return fmt.Errorf("schema type is invalid")
	}
	if description, exists := schema["description"]; exists {
		if _, ok := description.(string); !ok {
			return fmt.Errorf("description must be a string")
		}
	}
	if additional, exists := schema["additionalProperties"]; exists && additional != false {
		return fmt.Errorf("additionalProperties must be false")
	}
	properties, hasProperties := schema["properties"]
	if typeName == "object" {
		if _, hasRequired := schema["required"]; hasRequired && !hasProperties {
			return fmt.Errorf("required needs object properties")
		}
		if hasProperties {
			object, ok := properties.(map[string]any)
			if !ok || len(object) > limits.MaxProperties {
				return fmt.Errorf("object properties are invalid")
			}
			for _, child := range object {
				childSchema, ok := child.(map[string]any)
				if !ok {
					return fmt.Errorf("property schema must be an object")
				}
				if err := validateSchemaObject(childSchema, depth+1, limits); err != nil {
					return err
				}
			}
			if required, exists := schema["required"]; exists {
				array, ok := required.([]any)
				if !ok {
					return fmt.Errorf("required must be an array")
				}
				seen := map[string]bool{}
				for _, item := range array {
					name, ok := item.(string)
					if !ok || seen[name] || object[name] == nil {
						return fmt.Errorf("required contains an invalid property")
					}
					seen[name] = true
				}
			}
		}
	} else if hasProperties || schema["required"] != nil {
		return fmt.Errorf("properties and required require object type")
	}
	if typeName == "array" {
		child, ok := schema["items"].(map[string]any)
		if !ok {
			return fmt.Errorf("array items schema is required")
		}
		if err := validateSchemaObject(child, depth+1, limits); err != nil {
			return err
		}
	} else if schema["items"] != nil {
		return fmt.Errorf("items requires array type")
	}
	if depth > limits.MaxDepth {
		return fmt.Errorf("schema nesting is too deep")
	}
	return validateKeywordTypes(schema, typeName)
}

func validateKeywordTypes(schema map[string]any, typeName string) error {
	stringKeys := []string{"pattern", "format"}
	intKeys := []string{"minLength", "maxLength", "minItems", "maxItems"}
	numberKeys := []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf"}
	for _, key := range stringKeys {
		if value, ok := schema[key]; ok {
			if _, valid := value.(string); !valid || typeName != "string" {
				return fmt.Errorf("%s is invalid for schema type", key)
			}
		}
	}
	for _, key := range intKeys {
		if value, ok := schema[key]; ok {
			number, valid := value.(json.Number)
			if !valid || strings.ContainsAny(number.String(), ".eE") || (typeName != "string" && typeName != "array") {
				return fmt.Errorf("%s is invalid for schema type", key)
			}
		}
	}
	for _, key := range numberKeys {
		if value, ok := schema[key]; ok {
			if _, valid := value.(json.Number); !valid || (typeName != "integer" && typeName != "number") {
				return fmt.Errorf("%s is invalid for schema type", key)
			}
		}
	}
	if value, ok := schema["uniqueItems"]; ok {
		if _, valid := value.(bool); !valid || typeName != "array" {
			return fmt.Errorf("uniqueItems is invalid for schema type")
		}
	}
	if value, ok := schema["enum"]; ok {
		if array, valid := value.([]any); !valid || len(array) == 0 {
			return fmt.Errorf("enum must be a non-empty array")
		}
	}
	return nil
}

func toolError(code, message string) *apierror.Error {
	return apierror.OpenAI(code, message, "tools", http.StatusBadRequest, nil)
}

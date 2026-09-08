package native

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeABI struct {
	calls     []string
	initCode  int
	responses [][]byte
	codes     []int
	initArgs  [][]byte
}

func (f *fakeABI) Init(system, tools, index []byte) int {
	f.calls = append(f.calls, "init")
	f.initArgs = [][]byte{append([]byte(nil), system...), append([]byte(nil), tools...), append([]byte(nil), index...)}
	return f.initCode
}
func (f *fakeABI) Complete(text []byte, tokens int, output []byte) int {
	f.calls = append(f.calls, "complete:"+string(text))
	if len(f.codes) > 0 {
		code := f.codes[0]
		f.codes = f.codes[1:]
		if code < 0 {
			return code
		}
	}
	if len(f.responses) == 0 {
		return -9
	}
	data := f.responses[0]
	f.responses = f.responses[1:]
	copy(output, data)
	return 0
}
func (f *fakeABI) Reset() { f.calls = append(f.calls, "reset") }

func validEnvelope(vmid int) []byte {
	return append([]byte(`{"type":"call","success":true,"error":null,"error_code":null,"function_calls":[{"name":"pve_vm_start","arguments":{"vmid":`+json.Number(string(rune('0'+vmid))).String()+`}}],"reasoning":"ok","confidence":0.9,"validation":{"ungrounded":[],"negation":false},"prefill_tps":10,"decode_tps":20,"peak_ram_mb":30}`), 0)
}

func TestEngineExecutesResetInitAndAllTurns(t *testing.T) {
	first := []byte(`{"type":"call","success":true,"function_calls":[],"confidence":0.8,"validation":{"ungrounded":[],"negation":false}}\x00`)
	first[len(first)-4] = 0
	first = first[:len(first)-3]
	second := []byte(`{"type":"call","success":true,"function_calls":[{"name":"pve_vm_start","arguments":{"vmid":3052}}],"confidence":0.92,"validation":{"ungrounded":[],"negation":false}}`)
	second = append(second, 0)
	abi := &fakeABI{responses: [][]byte{first, second}}
	engine := NewEngine(abi, 65536)
	got, err := engine.Execute(Request{System: "date: now", ToolsJSON: []byte(`[{"name":"pve_vm_start"}]`), ToolNames: []string{"pve_vm_start"}, ToolIndexPath: "/tmp/index", MaxNewTokens: 256, Turns: []Turn{{Kind: TurnUser, Text: "start"}, {Kind: TurnToolResults, Text: `[{"ok":true}]`}}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"reset", "init", "complete:start", "complete:[{\"ok\":true}]"}
	if strings.Join(abi.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls=%v", abi.calls)
	}
	if string(abi.initArgs[0]) != "date: now" || string(abi.initArgs[2]) != "/tmp/index" {
		t.Fatalf("init args=%q", abi.initArgs)
	}
	if len(got.FunctionCalls) != 1 || got.FunctionCalls[0].Name != "pve_vm_start" || got.Confidence == nil || *got.Confidence != 0.92 {
		t.Fatalf("envelope=%+v", got)
	}
}

func TestEngineRejectsNativeFailuresAndUnsafeOutput(t *testing.T) {
	tests := []struct {
		name   string
		abi    *fakeABI
		code   string
		buffer int
	}{
		{"init", &fakeABI{initCode: -1}, "native_init_failed", 64},
		{"complete", &fakeABI{codes: []int{-2}}, "native_complete_failed", 64},
		{"malformed", &fakeABI{responses: [][]byte{[]byte("{\x00")}}, "native_output_invalid", 64},
		{"missing nul", &fakeABI{responses: [][]byte{[]byte(strings.Repeat("x", 64))}}, "native_output_truncated", 64},
		{"unknown tool", &fakeABI{responses: [][]byte{append([]byte(`{"type":"call","success":true,"function_calls":[{"name":"other","arguments":{}}]}`), 0)}}, "native_output_invalid", 256},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewEngine(tc.abi, tc.buffer)
			_, err := engine.Execute(Request{ToolsJSON: []byte(`[]`), ToolNames: []string{"allowed"}, MaxNewTokens: 1, Turns: []Turn{{Kind: TurnUser, Text: "x"}}})
			var nativeErr *NativeError
			if !errors.As(err, &nativeErr) || nativeErr.Code != tc.code {
				t.Fatalf("err=%+v calls=%v", err, tc.abi.calls)
			}
			if tc.name != "init" && tc.abi.calls[len(tc.abi.calls)-1] != "reset" {
				t.Fatalf("failure did not reset: %v", tc.abi.calls)
			}
		})
	}
}

func TestValidationTracksRequiredFieldPresence(t *testing.T) {
	var present, missing Validation
	if err := json.Unmarshal([]byte(`{"ungrounded":[],"negation":false}`), &present); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{}`), &missing); err != nil {
		t.Fatal(err)
	}
	if !present.UngroundedPresent || !present.NegationPresent || missing.UngroundedPresent || missing.NegationPresent {
		t.Fatalf("present=%+v missing=%+v", present, missing)
	}
}

func TestEngineRejectsNativeErrorEnvelope(t *testing.T) {
	response := append([]byte(`{"type":"respond","success":false,"error":"failed","error_code":"decode"}`), 0)
	engine := NewEngine(&fakeABI{responses: [][]byte{response}}, 256)
	_, err := engine.Execute(Request{ToolsJSON: []byte(`[]`), MaxNewTokens: 1, Turns: []Turn{{Kind: TurnUser, Text: "x"}}})
	var nativeErr *NativeError
	if !errors.As(err, &nativeErr) || nativeErr.Code != "native_engine_error" {
		t.Fatalf("error=%+v", err)
	}
}

func TestEngineClearsBufferBetweenCalls(t *testing.T) {
	long := append([]byte(`{"type":"respond","success":true,"function_calls":[]}`), 0)
	short := append([]byte(`{"type":"respond","success":true,"function_calls":[]}`), 0)
	abi := &fakeABI{responses: [][]byte{long, short}}
	engine := NewEngine(abi, 128)
	request := Request{ToolsJSON: []byte(`[]`), MaxNewTokens: 1, Turns: []Turn{{Kind: TurnUser, Text: "x"}}}
	if _, err := engine.Execute(request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Execute(request); err != nil {
		t.Fatal(err)
	}
}

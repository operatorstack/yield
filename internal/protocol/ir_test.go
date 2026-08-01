package protocol

// These tests bind the Go reference implementation to the language-neutral
// IR under ir/yield.v1: every value the runtime marshals must validate
// against the canonical schemas, so the IR cannot drift from the runtime.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func irCompiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	dir := filepath.Join("..", "..", "ir", "yield.v1")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("IR directory missing: %v", err)
	}
	c := jsonschema.NewCompiler()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".schema.json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s is not valid JSON: %v", e.Name(), err)
		}
		if err := c.AddResource(e.Name(), doc); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func validateIR(t *testing.T, c *jsonschema.Compiler, schemaFile string, v any) {
	t.Helper()
	compiled, err := c.Compile(schemaFile)
	if err != nil {
		t.Fatalf("%s does not compile: %v", schemaFile, err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(doc); err != nil {
		t.Fatalf("Go value does not validate against %s:\n%s\n%v", schemaFile, raw, err)
	}
}

const irDigest = "sha256:9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c9f2c"

func sampleEnvelope() RequestEnvelope {
	return RequestEnvelope{
		Protocol: Version,
		RunID:    "run_1",
		Skill:    SkillRef{Name: "safe-change", Digest: irDigest},
		Sequence: 1,
		Request: Request{
			ID:           "confirm-scope",
			Kind:         OpAskUser,
			Payload:      json.RawMessage(`{"question":"May this change modify the public API?","options":[{"value":"preserve"}]}`),
			OutputSchema: json.RawMessage(`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`),
		},
	}
}

func TestGoTypesValidateAgainstIR(t *testing.T) {
	c := irCompiler(t)

	validateIR(t, c, "request-envelope.schema.json", sampleEnvelope())

	validateIR(t, c, "response-envelope.schema.json", ResponseEnvelope{
		RunID: "run_1", Sequence: 1, RequestID: "confirm-scope",
		Status: "completed", Result: json.RawMessage(`{"value":"preserve"}`),
	})

	validateIR(t, c, "journal.schema.json", Journal{
		RunID: "run_1",
		Skill: SkillRef{Name: "safe-change", Digest: irDigest},
		Entries: []JournalEntry{{
			Request: sampleEnvelope().Request,
			Response: ResponseEnvelope{
				RunID: "run_1", Sequence: 1, RequestID: "confirm-scope",
				Status: "completed", Result: json.RawMessage(`{"value":"preserve"}`),
			},
		}},
	})

	env := sampleEnvelope()
	validateIR(t, c, "program-output.schema.json", ProgramOutput{
		Type: OutputRequest, Envelope: &env,
		Requirements: []Requirement{{Claim: "tests pass", Passed: true}},
	})
	validateIR(t, c, "program-output.schema.json", ProgramOutput{
		Type:     OutputTerminal,
		Terminal: &TerminalOutcome{Status: StatusCompleted, Result: json.RawMessage(`{"ok":true}`)},
	})
	validateIR(t, c, "program-output.schema.json", ProgramOutput{
		Type: OutputDiverged,
		Divergence: &Divergence{
			Sequence: 2, Expected: irDigest, Got: irDigest,
			Detail: "replay produced a different operation",
		},
	})
}

func TestIRRejectsUnknownKind(t *testing.T) {
	c := irCompiler(t)
	compiled, err := c.Compile("request-envelope.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	env := sampleEnvelope()
	env.Request.Kind = "shell_access"
	raw, _ := json.Marshal(env)
	doc, _ := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err := compiled.Validate(doc); err == nil {
		t.Fatal("the IR must reject operation kinds outside the closed set")
	}
}

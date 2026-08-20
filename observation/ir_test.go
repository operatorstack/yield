package observation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func observationSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join("..", "ir", "yield.observation.v1", "run-receipt.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("run-receipt.schema.json", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("run-receipt.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestGoReceiptValidatesAgainstObservationIR(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	digest := protocol.DigestBytes([]byte("source"))
	skill := protocol.SkillRef{Name: "schema-test", Version: "1.0.0", Digest: digest}
	events := []runlog.Event{
		event(t, 1, runlog.RunOpened, t0, map[string]any{"run_id": "run_schema", "skill_name": "schema-test", "input_digest": digest, "supervisor_version": "1.0.0"}),
		event(t, 2, runlog.RunStarted, t0, map[string]any{"run_id": "run_schema", "skill": skill, "input_digest": digest, "supervisor_version": "1.0.0", "required_yield_version": "1.0.0", "source_digest_profile": protocol.SkillSourceProfileV1, "source_digest": digest}),
		event(t, 3, runlog.RunCompleted, t0.Add(time.Second), map[string]any{"result": json.RawMessage(`{"ok":true}`)}),
	}
	r, err := Project(journalBytes(t, events))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := observationSchema(t).Validate(document); err != nil {
		t.Fatalf("Go receipt does not validate against observation IR:\n%s\n%v", raw, err)
	}
}

func TestObservationIRRejectsUnknownFields(t *testing.T) {
	raw := []byte(`{"schema":"yield.observation.v1","kind":"run_receipt","receipt_digest":"sha256:0000000000000000000000000000000000000000000000000000000000000000","journal":{"run_id":"r","head_sequence":1,"head_digest":"sha256:0000000000000000000000000000000000000000000000000000000000000000"},"run":{"id":"r"},"skill":{"name":"s","binding_digest":"sha256:0000000000000000000000000000000000000000000000000000000000000000"},"timing":{"started_at":"2026-08-20T10:00:00Z","last_observed_at":"2026-08-20T10:00:00Z"},"operations":[],"operation_summaries":[],"outcome":{"phase":"advancing"},"requirements":[],"response_rejections":[],"divergences":[],"prompt":"secret"}`)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := observationSchema(t).Validate(document); err == nil {
		t.Fatal("observation IR accepted an unknown raw-content field")
	}
}

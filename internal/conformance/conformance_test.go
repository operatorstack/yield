// Package conformance is the end-to-end certification harness: the SAME
// program, written in Go, TypeScript, Python, and Rust, driven through the
// real supervisor, must exhibit identical observable protocol behavior.
//
// The scenario matrix verifies the shared contract. Languages whose toolchain
// is absent are skipped with a notice. CI provides all four.
package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/operatorstack/yield/internal/engine"
	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

type language struct {
	name string
	dir  string
	tool string // required binary on PATH; empty = always available
}

var languages = []language{
	{name: "go", dir: "skill-go", tool: "go"},
	{name: "typescript", dir: "skill-ts", tool: "node"},
	{name: "python", dir: "skill-py", tool: "python3"},
	{name: "rust", dir: "skill-rs", tool: "cargo"},
}

func newEngine(t *testing.T, lang language) *engine.Engine {
	t.Helper()
	if lang.tool != "" {
		if _, err := exec.LookPath(lang.tool); err != nil {
			t.Skipf("toolchain %q not on PATH; skipping %s conformance", lang.tool, lang.name)
		}
	}
	abs, err := filepath.Abs(filepath.Join("testdata", lang.dir))
	if err != nil {
		t.Fatal(err)
	}
	return &engine.Engine{SkillDir: abs, RunsDir: t.TempDir(), Stderr: os.Stderr, SupervisorVersion: "dev"}
}

func respond(t *testing.T, e *engine.Engine, p *engine.Progress, result string) (*engine.Progress, error) {
	t.Helper()
	b, err := json.Marshal(protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: p.Envelope.Sequence,
		RequestID: p.Envelope.Request.ID, Status: "completed",
		Result: json.RawMessage(result),
	})
	if err != nil {
		t.Fatal(err)
	}
	return e.Resume(p.RunID, b, false)
}

// irValidator compiles the canonical request-envelope schema once.
func irValidator(t *testing.T) *jsonschema.Schema {
	t.Helper()
	dir := filepath.Join("..", "..", "ir", "yield.v1")
	c := jsonschema.NewCompiler()
	for _, name := range []string{"request-envelope.schema.json", "response-envelope.schema.json", "journal.schema.json", "program-output.schema.json"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource(name, doc); err != nil {
			t.Fatal(err)
		}
	}
	compiled, err := c.Compile("request-envelope.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func validateEnvelope(t *testing.T, schema *jsonschema.Schema, env *protocol.RequestEnvelope) {
	t.Helper()
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("envelope does not validate against the IR: %v\n%s", err, raw)
	}
}

// step is one observable protocol step: what the supervisor (and therefore
// the agent) sees. Payload bytes are deliberately excluded — key order is
// a language serialization detail; kind/id/sequence/status are the
// protocol.
type step struct {
	Seq  int
	Kind protocol.OpKind
	ID   string
}

type trace struct {
	Steps    []step
	Terminal string
	ReqsPass int
}

// runComplete drives the happy path to completion and returns the
// observable trace.
func runComplete(t *testing.T, lang language) trace {
	e := newEngine(t, lang)
	schema := irValidator(t)
	var tr trace

	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatalf("[%s] start: %v", lang.name, err)
	}
	validateEnvelope(t, schema, p.Envelope)
	tr.Steps = append(tr.Steps, step{p.Envelope.Sequence, p.Envelope.Request.Kind, p.Envelope.Request.ID})

	p, err = respond(t, e, p, `{"value":"yes"}`)
	if err != nil {
		t.Fatalf("[%s] resume q1: %v", lang.name, err)
	}
	validateEnvelope(t, schema, p.Envelope)
	tr.Steps = append(tr.Steps, step{p.Envelope.Sequence, p.Envelope.Request.Kind, p.Envelope.Request.ID})

	p, err = respond(t, e, p, `{"n":1}`)
	if err != nil {
		t.Fatalf("[%s] resume t2: %v", lang.name, err)
	}
	if p.Terminal == nil || p.Terminal.Status != protocol.StatusCompleted {
		t.Fatalf("[%s] must complete, got %+v", lang.name, p)
	}
	tr.Terminal = string(p.Terminal.Status)

	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var cmdSeen bool
	for _, ev := range l.Events() {
		switch ev.Type {
		case runlog.RequirementPassed:
			tr.ReqsPass++
		case runlog.RequirementFailed:
			t.Fatalf("[%s] no requirement may fail on the happy path", lang.name)
		case runlog.OperationRequested:
			var env protocol.RequestEnvelope
			if err := ev.Decode(&env); err != nil {
				t.Fatal(err)
			}
			if env.Request.Kind == protocol.OpRunCommand {
				cmdSeen = true
				tr.Steps = append(tr.Steps, step{env.Sequence, env.Request.Kind, env.Request.ID})
			}
		case runlog.OperationCompleted:
			var d struct {
				RequestID string          `json:"request_id"`
				Result    json.RawMessage `json:"result"`
			}
			if err := ev.Decode(&d); err != nil {
				t.Fatal(err)
			}
			if d.RequestID == "c3-echo" {
				var res protocol.CommandResult
				if err := json.Unmarshal(d.Result, &res); err != nil {
					t.Fatal(err)
				}
				if res.ExitCode != 0 || !strings.Contains(res.Stdout, "conform-ok") {
					t.Fatalf("[%s] run_command result must be the observed output, got %+v", lang.name, res)
				}
			}
		}
	}
	if !cmdSeen {
		t.Fatalf("[%s] the run_command operation must appear in the log", lang.name)
	}

	// Replay determinism on the completed run.
	rp, err := e.Replay(p.RunID)
	if err != nil {
		t.Fatalf("[%s] replay must be deterministic: %v", lang.name, err)
	}
	if rp.Terminal == nil || rp.Terminal.Status != protocol.StatusCompleted {
		t.Fatalf("[%s] replay must reproduce the terminal, got %+v", lang.name, rp)
	}
	return tr
}

// TestCrossLanguageTraceEquality is the core claim: one program, any
// language, the same observable protocol behavior.
func TestCrossLanguageTraceEquality(t *testing.T) {
	traces := map[string]trace{}
	for _, lang := range languages {
		lang := lang
		t.Run(lang.name, func(t *testing.T) {
			traces[lang.name] = runComplete(t, lang)
		})
	}
	if len(traces) < 2 {
		t.Skip("fewer than two toolchains available; equality is vacuous")
	}
	var refName string
	var ref trace
	for name, tr := range traces {
		refName, ref = name, tr
		break
	}
	for name, tr := range traces {
		if fmt.Sprintf("%+v", tr) != fmt.Sprintf("%+v", ref) {
			t.Fatalf("observable traces differ:\n%s: %+v\n%s: %+v", refName, ref, name, tr)
		}
	}
}

func TestRefusedTerminal(t *testing.T) {
	for _, lang := range languages {
		lang := lang
		t.Run(lang.name, func(t *testing.T) {
			e := newEngine(t, lang)
			p, err := e.StartRun(nil)
			if err != nil {
				t.Fatal(err)
			}
			p, err = respond(t, e, p, `{"value":"no"}`)
			if err != nil {
				t.Fatal(err)
			}
			if p.Terminal == nil || p.Terminal.Status != protocol.StatusRefused {
				t.Fatalf("declining must refuse, got %+v", p)
			}
			assertClosed(t, e, p.RunID, runlog.RunRefused)
		})
	}
}

func TestBlockedTerminal(t *testing.T) {
	for _, lang := range languages {
		lang := lang
		t.Run(lang.name, func(t *testing.T) {
			e := newEngine(t, lang)
			p, err := e.StartRun(nil)
			if err != nil {
				t.Fatal(err)
			}
			if p, err = respond(t, e, p, `{"value":"yes"}`); err != nil {
				t.Fatal(err)
			}
			if p, err = respond(t, e, p, `{"n":0}`); err != nil {
				t.Fatal(err)
			}
			if p.Terminal == nil || p.Terminal.Status != protocol.StatusBlocked {
				t.Fatalf("n=0 must block, got %+v", p)
			}
			assertClosed(t, e, p.RunID, runlog.RunBlocked)
		})
	}
}

func TestFailedRequirementNeverCompletes(t *testing.T) {
	for _, lang := range languages {
		lang := lang
		t.Run(lang.name, func(t *testing.T) {
			e := newEngine(t, lang)
			p, err := e.StartRun(nil)
			if err != nil {
				t.Fatal(err)
			}
			if p, err = respond(t, e, p, `{"value":"yes"}`); err != nil {
				t.Fatal(err)
			}
			if p, err = respond(t, e, p, `{"n":-1}`); err != nil {
				t.Fatal(err)
			}
			if p.Terminal == nil || p.Terminal.Status == protocol.StatusCompleted {
				t.Fatalf("a failed requirement must prevent completion, got %+v", p)
			}
			l, err := e.Log(p.RunID)
			if err != nil {
				t.Fatal(err)
			}
			var failed, blocked bool
			for _, ev := range l.Events() {
				if ev.Type == runlog.RequirementFailed {
					failed = true
				}
				if ev.Type == runlog.RunBlocked {
					blocked = true
				}
				if ev.Type == runlog.RunCompleted {
					t.Fatal("run.completed must never follow a failed requirement")
				}
			}
			if !failed || !blocked {
				t.Fatalf("log must record requirement.failed and run.blocked (failed=%v blocked=%v)", failed, blocked)
			}
		})
	}
}

func TestGuardRefusals(t *testing.T) {
	for _, lang := range languages {
		lang := lang
		t.Run(lang.name, func(t *testing.T) {
			e := newEngine(t, lang)
			p, err := e.StartRun(nil)
			if err != nil {
				t.Fatal(err)
			}

			// Schema-invalid ask_user result.
			if _, err := respond(t, e, p, `{"wrong":"shape"}`); err == nil || !strings.Contains(err.Error(), "schema-invalid") {
				t.Fatalf("schema-invalid result must be refused, got %v", err)
			}
			if _, err := respond(t, e, p, `{"value":"not-a-declared-option"}`); err == nil || !strings.Contains(err.Error(), "schema-invalid") {
				t.Fatalf("unknown ask_user option must be refused, got %v", err)
			}
			p2, err := respond(t, e, p, `{"value":"yes"}`)
			if err != nil {
				t.Fatal(err)
			}

			// Stale: re-answer sequence 1.
			stale, _ := json.Marshal(protocol.ResponseEnvelope{
				RunID: p.RunID, Sequence: 1, RequestID: "q1-proceed",
				Status: "completed", Result: json.RawMessage(`{"value":"REWRITE"}`),
			})
			if _, err := e.Resume(p.RunID, stale, false); err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("rewriting an answered sequence must be refused, got %v", err)
			}

			// Wrong run id.
			wrong, _ := json.Marshal(protocol.ResponseEnvelope{
				RunID: "run_someone_else", Sequence: p2.Envelope.Sequence, RequestID: p2.Envelope.Request.ID,
				Status: "completed", Result: json.RawMessage(`{"n":1}`),
			})
			if _, err := e.Resume(p.RunID, wrong, false); err == nil || !strings.Contains(err.Error(), "wrong-run") {
				t.Fatalf("a response for another run must be refused, got %v", err)
			}
		})
	}
}

// TestDivergenceFailsLoudlyEverywhere tampers the recorded first operation
// in the log; every SDK must detect the drift at replay and refuse to
// consume the recorded response.
func TestDivergenceFailsLoudlyEverywhere(t *testing.T) {
	for _, lang := range languages {
		lang := lang
		t.Run(lang.name, func(t *testing.T) {
			e := newEngine(t, lang)
			p, err := e.StartRun(nil)
			if err != nil {
				t.Fatal(err)
			}
			if p, err = respond(t, e, p, `{"value":"yes"}`); err != nil {
				t.Fatal(err)
			}

			logPath := filepath.Join(e.RunsDir, p.RunID+".jsonl")
			b, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			tampered := bytes.ReplaceAll(b,
				[]byte("Proceed with the conformance run?"),
				[]byte("Tampered question, same shape ok?"))
			if bytes.Equal(tampered, b) {
				t.Fatal("tampering must change the log")
			}
			if err := os.WriteFile(logPath, tampered, 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := e.Replay(p.RunID); err == nil || !strings.Contains(err.Error(), "diverged") {
				t.Fatalf("every SDK must fail loudly on a drifted recorded operation, got %v", err)
			}
		})
	}
}

func assertClosed(t *testing.T, e *engine.Engine, runID string, want runlog.EventType) {
	t.Helper()
	l, err := e.Log(runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range l.Events() {
		if ev.Type == want {
			return
		}
	}
	t.Fatalf("log must contain %s", want)
}

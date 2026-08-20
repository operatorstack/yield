package observation

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

func TestProjectCompletedReceiptIsDeterministicAndPrivate(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	secret := "SECRET_PROMPT_COMMAND_STDOUT_TOKEN"
	skill := protocol.SkillRef{Name: "review-release", Digest: protocol.DigestBytes([]byte("source"))}
	envelope := protocol.RequestEnvelope{
		Protocol: protocol.Version, RunID: "run_test", Skill: skill, Sequence: 1,
		Request: protocol.Request{ID: "review", Kind: protocol.OpAgentTask, Payload: json.RawMessage(`{"instruction":"` + secret + `"}`)},
	}
	result := json.RawMessage(`{"answer":"` + secret + `"}`)
	events := []runlog.Event{
		event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_test", "skill": skill, "input_digest": protocol.DigestBytes([]byte(secret))}),
		event(t, 2, runlog.OperationRequested, t0.Add(time.Second), envelope),
		event(t, 3, runlog.OperationCompleted, t0.Add(4*time.Second), map[string]any{"sequence": 1, "request_id": "review", "result": result, "result_digest": protocol.DigestBytes(result)}),
		event(t, 4, runlog.RequirementPassed, t0.Add(5*time.Second), protocol.Requirement{Claim: secret, Passed: true, EvidenceDigest: protocol.DigestBytes(result)}),
		event(t, 5, runlog.RunCompleted, t0.Add(6*time.Second), map[string]any{"result": result, "requirements": 1}),
	}
	snapshot := journalBytes(t, events)
	first, err := Project(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Project(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := CanonicalBytes(first)
	secondBytes, _ := CanonicalBytes(second)
	if !bytes.Equal(firstBytes, secondBytes) || first.ReceiptDigest != second.ReceiptDigest {
		t.Fatal("same journal prefix produced different receipt bytes")
	}
	if strings.Contains(string(firstBytes), secret) {
		t.Fatalf("receipt leaked forbidden raw content: %s", firstBytes)
	}
	if first.Outcome.Phase != "terminal" || first.Outcome.TerminalDisposition != "completed" {
		t.Fatalf("unexpected outcome: %+v", first.Outcome)
	}
	if first.Operations[0].ElapsedMS == nil || *first.Operations[0].ElapsedMS != 3000 {
		t.Fatalf("unexpected operation timing: %+v", first.Operations[0])
	}
}

func TestProjectLegacyJournalDoesNotInventSourceOrRuntime(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "legacy", Digest: protocol.DigestBytes([]byte("legacy"))}
	events := []runlog.Event{
		event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_legacy", "skill": skill, "input_digest": protocol.DigestBytes(nil)}),
	}
	receipt, err := Project(journalBytes(t, events))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Runtime != nil || receipt.Skill.SourceDigest != nil {
		t.Fatalf("legacy receipt invented facts: %+v", receipt)
	}
	if receipt.Outcome.Phase != "advancing" {
		t.Fatalf("legacy phase = %q", receipt.Outcome.Phase)
	}
}

func TestProjectClockAnomalyOmitsDuration(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "clock", Digest: protocol.DigestBytes([]byte("clock"))}
	envelope := protocol.RequestEnvelope{Protocol: protocol.Version, RunID: "run_clock", Skill: skill, Sequence: 1, Request: protocol.Request{ID: "ask", Kind: protocol.OpAskUser, Payload: json.RawMessage(`{"question":"ok"}`)}}
	events := []runlog.Event{
		event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_clock", "skill": skill}),
		event(t, 2, runlog.OperationRequested, t0.Add(time.Second), envelope),
		event(t, 3, runlog.OperationCompleted, t0, map[string]any{"sequence": 1, "request_id": "ask", "result": json.RawMessage(`{"value":"yes"}`)}),
	}
	receipt, err := Project(journalBytes(t, events))
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Timing.ClockAnomaly || !receipt.Operations[0].ClockAnomaly || receipt.Operations[0].ElapsedMS != nil {
		t.Fatalf("clock anomaly was not preserved: %+v", receipt.Operations[0])
	}
}

func TestCanonicalBytesUsesJCSStringAndKeyRules(t *testing.T) {
	raw, err := canonicalBytes(map[string]any{"😀": "\u2028", "€": "<", "\r": "\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"\\r\":\"\\n\",\"€\":\"<\",\"😀\":\"\u2028\"}"
	if string(raw) != want {
		t.Fatalf("canonical JSON = %q, want %q", raw, want)
	}
}

func TestProjectLifecycleClassifications(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	digest := protocol.DigestBytes([]byte("source"))
	skill := protocol.SkillRef{Name: "classify", Digest: digest}
	started := func() runlog.Event {
		return event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_classify", "skill": skill})
	}
	tests := []struct {
		name   string
		events []runlog.Event
		phase  string
		cause  string
	}{
		{
			name: "initialization failed",
			events: []runlog.Event{
				event(t, 1, runlog.RunOpened, t0, map[string]any{"run_id": "run_init", "skill_name": "classify", "input_digest": digest}),
				event(t, 2, runlog.RunInitializationFailed, t0.Add(time.Second), map[string]any{"phase": "initialize", "code": "manifest_invalid"}),
			},
			phase: "initialization_failed",
		},
		{
			name:   "recoverable execution failure",
			events: []runlog.Event{started(), event(t, 2, runlog.ExecutionFailed, t0.Add(time.Second), map[string]any{"code": "subprocess_failed"})},
			phase:  "recoverable_error",
		},
		{
			name: "diverged with an unanswered prior frontier",
			events: []runlog.Event{
				started(),
				event(t, 2, runlog.OperationRequested, t0.Add(time.Second), protocol.RequestEnvelope{
					Protocol: protocol.Version, RunID: "run_classify", Skill: skill, Sequence: 1,
					Request: protocol.Request{ID: "ask", Kind: protocol.OpAskUser},
				}),
				event(t, 3, runlog.ReplayDiverged, t0.Add(2*time.Second), protocol.Divergence{Sequence: 1, Expected: digest, Got: protocol.DigestBytes([]byte("got")), Detail: "PRIVATE DETAIL"}),
			},
			phase: "diverged",
		},
		{
			name:   "blocked requirement",
			events: []runlog.Event{started(), event(t, 2, runlog.RequirementFailed, t0.Add(time.Second), protocol.Requirement{Claim: "PRIVATE CLAIM"}), event(t, 3, runlog.RunBlocked, t0.Add(2*time.Second), map[string]any{"cause": "requirement_failed", "reason": "PRIVATE REASON"})},
			phase:  "terminal", cause: "requirement_failed",
		},
		{
			name:   "refused",
			events: []runlog.Event{started(), event(t, 2, runlog.RunRefused, t0.Add(time.Second), map[string]any{"reason": "PRIVATE REASON"})},
			phase:  "terminal", cause: "refused",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := Project(journalBytes(t, test.events))
			if err != nil {
				t.Fatal(err)
			}
			if r.Outcome.Phase != test.phase || r.Outcome.TerminalCause != test.cause {
				t.Fatalf("outcome = %+v", r.Outcome)
			}
			raw, _ := CanonicalBytes(r)
			if strings.Contains(string(raw), "PRIVATE") {
				t.Fatalf("receipt leaked private detail: %s", raw)
			}
		})
	}
}

func TestProjectRejectsBrokenOperationPairing(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	digest := protocol.DigestBytes([]byte("source"))
	skill := protocol.SkillRef{Name: "broken", Digest: digest}
	events := []runlog.Event{
		event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_broken", "skill": skill}),
		event(t, 2, runlog.OperationCompleted, t0.Add(time.Second), map[string]any{"sequence": 1, "request_id": "missing", "result_digest": digest}),
	}
	if _, err := Project(journalBytes(t, events)); err == nil {
		t.Fatal("broken operation pairing was accepted")
	}
}

func TestProjectFailsClosedOnUnknownAndPostTerminalEvents(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	digest := protocol.DigestBytes([]byte("source"))
	skill := protocol.SkillRef{Name: "closed", Digest: digest}
	started := event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_closed", "skill": skill})
	unknown := []runlog.Event{started, event(t, 2, runlog.EventType("future.event"), t0.Add(time.Second), map[string]any{})}
	if _, err := Project(journalBytes(t, unknown)); err == nil {
		t.Fatal("unknown event was silently omitted")
	}
	postTerminal := []runlog.Event{
		started,
		event(t, 2, runlog.RunRefused, t0.Add(time.Second), map[string]any{}),
		event(t, 3, runlog.RunCompleted, t0.Add(2*time.Second), map[string]any{}),
	}
	if _, err := Project(journalBytes(t, postTerminal)); err == nil {
		t.Fatal("event after terminal was accepted")
	}
}

func TestProjectRejectsPartialBlankAndUnknownEnvelopeFields(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "strict", Digest: protocol.DigestBytes([]byte("strict"))}
	events := []runlog.Event{event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_strict", "skill": skill})}
	complete := journalBytes(t, events)

	for name, journal := range map[string][]byte{
		"partial line":           complete[:len(complete)-1],
		"blank line":             append(append([]byte{}, complete...), '\n'),
		"unknown envelope field": []byte(strings.Replace(string(complete), `"data":`, `"unknown":true,"data":`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Project(journal); err == nil {
				t.Fatal("malformed journal prefix was accepted")
			}
		})
	}
}

func TestParseRequiresExactCanonicalVerifiedBytes(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "parse", Digest: protocol.DigestBytes([]byte("parse"))}
	events := []runlog.Event{event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_parse", "skill": skill})}
	receipt, err := Project(journalBytes(t, events))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalBytes(receipt)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(canonical)
	if err != nil || parsed.ReceiptDigest != receipt.ReceiptDigest {
		t.Fatalf("canonical receipt did not round trip: %+v, %v", parsed, err)
	}

	mutatedDigest := bytes.Replace(canonical, []byte(receipt.ReceiptDigest), []byte(protocol.DigestBytes([]byte("wrong"))), 1)
	unknownField := append(append([]byte{}, canonical[:len(canonical)-1]...), []byte(`,"prompt":"secret"}`)...)
	for name, raw := range map[string][]byte{
		"noncanonical whitespace": append([]byte(" "), canonical...),
		"digest mismatch":         mutatedDigest,
		"unknown field":           unknownField,
		"trailing content":        append(append([]byte{}, canonical...), []byte("\n{}")...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(raw); err == nil {
				t.Fatal("invalid receipt bytes were accepted")
			}
		})
	}
}

func event(t *testing.T, sequence int, kind runlog.EventType, at time.Time, data any) runlog.Event {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return runlog.Event{Seq: sequence, Type: kind, At: at, Data: raw}
}

func journalBytes(t *testing.T, events []runlog.Event) []byte {
	t.Helper()
	var output bytes.Buffer
	for _, item := range events {
		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		output.Write(raw)
		output.WriteByte('\n')
	}
	return output.Bytes()
}

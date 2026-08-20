package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/operatorstack/yield/internal/guard"
	"github.com/operatorstack/yield/internal/outbox"
	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
	receipt "github.com/operatorstack/yield/observation"
)

// testEngine points at a testdata skill but keeps run logs in a temp dir,
// so tests never write into the source tree.
func testEngine(t *testing.T, skill string) *Engine {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", skill))
	if err != nil {
		t.Fatal(err)
	}
	return &Engine{SkillDir: abs, RunsDir: t.TempDir(), Stderr: os.Stderr}
}

func TestStartRunMaterializesReceiptBeforeReturn(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := receipt.NewStore(filepath.Dir(e.RunsDir)).LoadRun(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome.Phase != "awaiting_response" || r.Skill.SourceDigest == nil || r.Skill.SourceDigest.Profile != protocol.SkillSourceProfileV1 {
		t.Fatalf("unexpected materialized receipt: %+v", r)
	}
}

func TestExperimentMetadataIsObservedButNotInReplayJournal(t *testing.T) {
	e := testEngine(t, "skill-basic")
	experiment := &receipt.ExperimentContext{ExperimentID: "exp-1", VariantID: "candidate-a", Role: "candidate", BaselineVariantID: "baseline-a"}
	p, err := e.StartRunWithOptions(nil, StartOptions{Experiment: experiment})
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := e.Receipt(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Experiment == nil || *r.Experiment != *experiment {
		t.Fatalf("experiment context was not projected: %+v", r.Experiment)
	}
	if _, err := e.Replay(p.RunID); err != nil {
		t.Fatalf("experiment metadata changed replay: %v", err)
	}
}

func TestInitializationFailureHasRunIDJournalAndReceipt(t *testing.T) {
	skillDir := t.TempDir()
	runsDir := filepath.Join(t.TempDir(), "runs")
	if err := os.MkdirAll(runsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	e := &Engine{SkillDir: skillDir, RunsDir: runsDir, SupervisorVersion: "1.0.0", Stderr: os.Stderr}
	_, err := e.StartRun(nil)
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.RunID == "" {
		t.Fatalf("initialization error did not preserve run id: %v", err)
	}
	l, openErr := e.Log(runErr.RunID)
	if openErr != nil {
		t.Fatal(openErr)
	}
	if got := l.Events(); len(got) != 2 || got[0].Type != runlog.RunOpened || got[1].Type != runlog.RunInitializationFailed {
		t.Fatalf("unexpected initialization journal: %+v", got)
	}
	r, _, loadErr := receipt.NewStore(filepath.Dir(e.RunsDir)).LoadRun(runErr.RunID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if r.Outcome.Phase != "initialization_failed" || r.Outcome.FailureCode != "runner_missing" {
		t.Fatalf("unexpected initialization receipt: %+v", r.Outcome)
	}
}

func TestRustWorkspaceSkillWithoutLocalLockfileReachesExecution(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "Cargo.lock"), []byte("version = 4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(workspace, "audit-security")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.rs": "fn main() {}\n",
		"runner.go": `package main
import "fmt"
func main() { fmt.Println("{\"type\":\"terminal\",\"terminal\":{\"status\":\"completed\",\"result\":null}}") }
`,
		"skill.json": `{"version":1,"yield_version":"1.0.0","language":"rust","run":["go","run","runner.go"]}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(skillDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(skillDir, "Cargo.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("test skill unexpectedly has a local lockfile: %v", err)
	}
	runsDir := filepath.Join(t.TempDir(), "runs")
	if err := os.MkdirAll(runsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	e := &Engine{SkillDir: skillDir, RunsDir: runsDir, SupervisorVersion: "1.0.0", Stderr: os.Stderr}
	progress, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Terminal == nil || progress.Terminal.Status != protocol.StatusCompleted {
		t.Fatalf("workspace Rust skill did not reach completion: %+v", progress)
	}
	r, _, loadErr := receipt.NewStore(filepath.Dir(runsDir)).LoadRun(progress.RunID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if r.Outcome.Phase != "terminal" || r.Skill.SourceDigest == nil || r.Skill.SourceDigest.Profile != protocol.SkillSourceProfileV1 {
		t.Fatalf("unexpected receipt: %+v", r)
	}
}

func TestMaterializationFailureDoesNotRewriteJournal(t *testing.T) {
	e := testEngine(t, "skill-basic")
	yieldDir := filepath.Dir(e.RunsDir)
	if err := os.WriteFile(filepath.Join(yieldDir, "receipts"), []byte("block directory creation"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := e.StartRun(nil)
	var runErr *RunError
	if !errors.As(err, &runErr) {
		t.Fatalf("expected run-bound materialization error, got %v", err)
	}
	l, openErr := e.Log(runErr.RunID)
	if openErr != nil {
		t.Fatal(openErr)
	}
	if len(l.Events()) < 3 || l.Events()[0].Type != runlog.RunOpened || l.Events()[1].Type != runlog.RunStarted {
		t.Fatalf("formal journal was not preserved: %+v", l.Events())
	}
}

func TestOutboxStateDoesNotModifyJournalOrReplay(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("true fixture is Unix-only")
	}
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(e.RunsDir, p.RunID+".jsonl")
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	r, raw, err := e.Receipt(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	manager := outbox.New(filepath.Dir(e.RunsDir))
	if err := manager.Enqueue("test-sink", r, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Deliver(context.Background(), "test-sink", []string{"true"}, time.Second); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("outbox operation modified the authoritative journal")
	}
	if _, err := e.Replay(p.RunID); err != nil {
		t.Fatalf("outbox operation changed replay: %v", err)
	}
}

func TestConcurrentIdenticalResumeCommitsOnce(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: p.Envelope.Sequence, RequestID: p.Envelope.Request.ID,
		Status: "completed", Result: json.RawMessage(`{"value":"preserve"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, callErr := e.Resume(p.RunID, response, false); errs <- callErr }()
	}
	wg.Wait()
	close(errs)
	for callErr := range errs {
		if callErr != nil {
			t.Fatalf("identical concurrent retry failed: %v", callErr)
		}
	}
	log, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	for _, event := range log.Events() {
		if event.Type == runlog.OperationCompleted {
			var data struct {
				RequestID string `json:"request_id"`
			}
			_ = event.Decode(&data)
			if data.RequestID == "confirm-scope" {
				completed++
			}
		}
	}
	if completed != 1 {
		t.Fatalf("confirm-scope completion events = %d, want 1", completed)
	}
}

func TestConcurrentResumeAndReceiptInspectionSeeWholePrefixes(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: p.Envelope.Sequence, RequestID: p.Envelope.Request.ID,
		Status: "completed", Result: json.RawMessage(`{"value":"preserve"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	go func() { _, callErr := e.Resume(p.RunID, response, false); errs <- callErr }()
	go func() { _, _, callErr := e.Receipt(p.RunID); errs <- callErr }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent transition observed a partial prefix: %v", err)
		}
	}
}

func TestConcurrentResumeProcessesCommitOnce(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: p.Envelope.Sequence, RequestID: p.Envelope.Request.ID,
		Status: "completed", Result: json.RawMessage(`{"value":"preserve"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	responsePath := filepath.Join(t.TempDir(), "response.json")
	if err := os.WriteFile(responsePath, response, 0o600); err != nil {
		t.Fatal(err)
	}
	commands := make([]*exec.Cmd, 2)
	outputs := make([]bytes.Buffer, 2)
	for index := range commands {
		commands[index] = exec.Command(os.Args[0], "-test.run=^TestResumeProcessHelper$", "--", e.SkillDir, e.RunsDir, p.RunID, responsePath)
		commands[index].Env = append(os.Environ(), "YIELD_RESUME_HELPER=1")
		commands[index].Stdout = &outputs[index]
		commands[index].Stderr = &outputs[index]
		if err := commands[index].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for index, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("resume process failed: %v: %s", err, outputs[index].String())
		}
	}
	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	for _, event := range l.Events() {
		if event.Type == runlog.OperationCompleted {
			var data struct {
				RequestID string `json:"request_id"`
			}
			_ = event.Decode(&data)
			if data.RequestID == "confirm-scope" {
				completed++
			}
		}
	}
	if completed != 1 {
		t.Fatalf("cross-process completion events = %d, want 1", completed)
	}
}

func TestResumeProcessHelper(t *testing.T) {
	if os.Getenv("YIELD_RESUME_HELPER") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+4 >= len(os.Args) {
		os.Exit(2)
	}
	response, err := os.ReadFile(os.Args[separator+4])
	if err != nil {
		os.Exit(3)
	}
	e := &Engine{SkillDir: os.Args[separator+1], RunsDir: os.Args[separator+2], Stderr: os.Stderr}
	if _, err := e.Resume(os.Args[separator+3], response, false); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestRespondBuildsEnvelopeFromPendingFrontier(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err = e.Respond(p.RunID, json.RawMessage(`{"value":"preserve"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Envelope == nil || p.Envelope.Request.ID != "summarize" {
		t.Fatalf("direct response did not reach the next frontier: %+v", p)
	}
}

func TestContinueAdvancesAfterCommittedResponse(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	result := json.RawMessage(`{"value":"preserve"}`)
	if err := e.acceptResponse(l, p.Envelope, protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: p.Envelope.Sequence, RequestID: p.Envelope.Request.ID,
		Status: "completed", Result: result,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = e.Continue(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Envelope == nil || p.Envelope.Request.ID != "summarize" {
		t.Fatalf("continue did not reconstruct the next frontier: %+v", p)
	}
}

func TestRespondRecoveryRejectsDifferentCommittedContent(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	committed := json.RawMessage(`{"value":"preserve"}`)
	if err := e.acceptResponse(l, p.Envelope, protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: p.Envelope.Sequence, RequestID: p.Envelope.Request.ID,
		Status: "completed", Result: committed,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Respond(p.RunID, json.RawMessage(`{"value":"replace"}`)); err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("different recovery response = %v", err)
	}
	p, err = e.Respond(p.RunID, committed)
	if err != nil {
		t.Fatal(err)
	}
	if p.Envelope == nil || p.Envelope.Request.ID != "summarize" {
		t.Fatalf("exact recovery did not reach the next frontier: %+v", p)
	}
}

func respond(t *testing.T, e *Engine, p *Progress, result string, migrate bool) (*Progress, error) {
	t.Helper()
	b, err := json.Marshal(protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: p.Envelope.Sequence,
		RequestID: p.Envelope.Request.ID, Status: "completed",
		Result: json.RawMessage(result),
	})
	if err != nil {
		t.Fatal(err)
	}
	return e.Resume(p.RunID, b, migrate)
}

func TestEndToEndRunResumeComplete(t *testing.T) {
	e := testEngine(t, "skill-basic")

	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Envelope == nil || p.Envelope.Request.ID != "confirm-scope" || p.Envelope.Request.Kind != protocol.OpAskUser {
		t.Fatalf("first operation must be ask_user confirm-scope, got %+v", p.Envelope)
	}
	if p.Envelope.Protocol != protocol.Version {
		t.Fatalf("envelope must carry %s", protocol.Version)
	}

	// A stale response (sequence 0) is refused and recorded.
	stale, _ := json.Marshal(protocol.ResponseEnvelope{
		RunID: p.RunID, Sequence: 0, RequestID: "confirm-scope",
		Status: "completed", Result: json.RawMessage(`{"value":"x"}`),
	})
	if _, err := e.Resume(p.RunID, stale, false); err == nil {
		t.Fatal("stale response must be refused")
	}

	p, err = respond(t, e, p, `{"value":"preserve"}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Envelope == nil || p.Envelope.Request.ID != "summarize" {
		t.Fatalf("second operation must be agent_task summarize, got %+v", p.Envelope)
	}

	// A schema-invalid agent_task result is refused.
	if _, err := respond(t, e, p, `{"not_summary":1}`, false); err == nil {
		t.Fatal("schema-invalid result must be refused")
	}

	// A valid result lets the engine advance through run_command (executed
	// by the engine itself) and the requirement to completion.
	p, err = respond(t, e, p, `{"summary":"a tiny repo"}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Terminal == nil || p.Terminal.Status != protocol.StatusCompleted {
		t.Fatalf("run must complete, got %+v", p)
	}

	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	var cmdResult protocol.CommandResult
	for _, ev := range l.Events() {
		types = append(types, string(ev.Type))
		if ev.Type == runlog.OperationCompleted {
			var d struct {
				RequestID string          `json:"request_id"`
				Result    json.RawMessage `json:"result"`
			}
			_ = ev.Decode(&d)
			if d.RequestID == "run-tests" {
				_ = json.Unmarshal(d.Result, &cmdResult)
			}
		}
	}
	joined := strings.Join(types, ",")
	for _, want := range []string{"run.started", "operation.requested", "operation.completed", "requirement.passed", "run.completed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("log must contain %s; got %s", want, joined)
		}
	}
	if strings.Contains(joined, string(runlog.RequirementFailed)) {
		t.Fatalf("no requirement failed in this run; got %s", joined)
	}
	// run_command was executed by the engine: observed fact, not transcription.
	if cmdResult.ExitCode != 0 || !strings.Contains(cmdResult.Stdout, "test-ok") {
		t.Fatalf("run-tests result must be the observed command output, got %+v", cmdResult)
	}

	// The closed run refuses further responses.
	if _, err := respond(t, e, &Progress{RunID: p.RunID, Envelope: &protocol.RequestEnvelope{Sequence: 2, Request: protocol.Request{ID: "summarize"}}}, `{"summary":"again"}`, false); err == nil {
		t.Fatal("responses on a closed run must be refused")
	}
	receipt, _, err := e.Receipt(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	rejections := map[string]int{}
	for _, rejection := range receipt.ResponseRejections {
		rejections[rejection.Reason] = rejection.Count
	}
	for _, reason := range []string{"stale-response", "schema-invalid", "run-closed"} {
		if rejections[reason] != 1 {
			t.Fatalf("receipt rejection %s count = %d", reason, rejections[reason])
		}
	}
}

func TestReplayIsDeterministic(t *testing.T) {
	e := testEngine(t, "skill-basic")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err = respond(t, e, p, `{"value":"migration"}`, false)
	if err != nil {
		t.Fatal(err)
	}
	rp, err := e.Replay(p.RunID)
	if err != nil {
		t.Fatalf("replay must be deterministic: %v", err)
	}
	if rp.Envelope == nil || rp.Envelope.Request.ID != p.Envelope.Request.ID {
		t.Fatalf("replay must reach the recorded frontier %q, got %+v", p.Envelope.Request.ID, rp)
	}
}

func TestReplayDivergenceFailsLoudly(t *testing.T) {
	e := testEngine(t, "skill-envbranch")
	t.Setenv("YIELD_TEST_BRANCH", "a")

	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err = respond(t, e, p, `{"value":"one"}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Envelope.Request.ID != "second-question-a" {
		t.Fatalf("branch a must yield second-question-a, got %s", p.Envelope.Request.ID)
	}

	// The program's behavior changes under its feet: replaying the journal
	// now produces a different second operation. The run must fail loudly,
	// never silently fork.
	t.Setenv("YIELD_TEST_BRANCH", "b")
	_, err = respond(t, e, p, `{"value":"two"}`, false)
	if err == nil || !strings.Contains(err.Error(), "diverged") {
		t.Fatalf("divergence must fail loudly, got %v", err)
	}

	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range l.Events() {
		if ev.Type == runlog.ReplayDiverged {
			found = true
		}
	}
	if !found {
		t.Fatal("replay.diverged must be recorded in the run log")
	}
}

func TestExecuteRejectsAmbiguousProgramOutput(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"version":1,"yield_version":"1.2.3","run":["sh","-c","printf '%s\\n' '{\"type\":\"request\",\"envelope\":{},\"terminal\":{\"status\":\"completed\"}}'"]}`
	if err := os.WriteFile(filepath.Join(dir, "skill.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{SkillDir: dir, RunsDir: t.TempDir(), Stderr: os.Stderr, SupervisorVersion: "1.2.3"}
	l, err := runlog.Create(e.RunsDir, "run_1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Append(runlog.RunStarted, map[string]any{
		"run_id": "run_1",
		"skill":  protocol.SkillRef{Name: "invalid", Digest: protocol.DigestBytes([]byte("invalid"))},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = e.execute(l, "run_1")
	if err == nil || !strings.Contains(err.Error(), "exactly one variant") {
		t.Fatalf("engine must reject ambiguous output before dispatch, got %v", err)
	}
}

func TestExecuteBindsWorkflowToSupervisorVersion(t *testing.T) {
	makeLog := func(t *testing.T, e *Engine) *runlog.Log {
		t.Helper()
		l, err := runlog.Create(e.RunsDir, "run_1")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := l.Append(runlog.RunStarted, map[string]any{
			"run_id": "run_1",
			"skill":  protocol.SkillRef{Name: "identity", Digest: protocol.DigestBytes([]byte("identity"))},
		}); err != nil {
			t.Fatal(err)
		}
		return l
	}
	makeEngine := func(t *testing.T, supervisor string) *Engine {
		t.Helper()
		dir := t.TempDir()
		manifest := `{"version":1,"yield_version":"1.2.3","language":"typescript","run":["sh","-c","test \"$YIELD_SUPERVISOR_VERSION\" = 1.2.3 && printf '%s\\n' '{\"type\":\"terminal\",\"terminal\":{\"status\":\"completed\"}}'"]}`
		if err := os.WriteFile(filepath.Join(dir, "skill.json"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		return &Engine{SkillDir: dir, RunsDir: t.TempDir(), Stderr: os.Stderr, SupervisorVersion: supervisor}
	}

	t.Run("matching version reaches the SDK", func(t *testing.T) {
		e := makeEngine(t, "1.2.3")
		out, err := e.execute(makeLog(t, e), "run_1")
		if err != nil || out.Terminal == nil || out.Terminal.Status != protocol.StatusCompleted {
			t.Fatalf("matching supervisor = out %#v, err %v", out, err)
		}
	})
	for _, supervisor := range []string{"", "1.2.2", "1.2.4"} {
		t.Run("rejects "+supervisor, func(t *testing.T) {
			e := makeEngine(t, supervisor)
			_, err := e.execute(makeLog(t, e), "run_1")
			if err == nil || !strings.Contains(err.Error(), "1.2.3") {
				t.Fatalf("supervisor %q error = %v", supervisor, err)
			}
			if supervisor != "" && !strings.Contains(err.Error(), supervisor) {
				t.Fatalf("error must name both versions: %v", err)
			}
		})
	}
}

func TestFailedRequirementBlocksRun(t *testing.T) {
	e := testEngine(t, "skill-reqfail")
	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Terminal == nil || p.Terminal.Status == protocol.StatusCompleted {
		t.Fatalf("a failed requirement must prevent completion, got %+v", p)
	}
	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	s, err := guard.Reconstruct(l)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ReqFailed || !s.Closed {
		t.Fatalf("log must show requirement.failed and a closed run; state %+v", s)
	}
	var blocked bool
	for _, ev := range l.Events() {
		if ev.Type == runlog.RunBlocked {
			blocked = true
		}
		if ev.Type == runlog.RunCompleted {
			t.Fatal("run.completed must never follow a failed requirement")
		}
	}
	if !blocked {
		t.Fatal("run must close blocked")
	}
}

func TestDigestMismatchRefusedThenMigrates(t *testing.T) {
	// Copy the basic skill into a mutable dir INSIDE the module tree (so
	// `go run .` still resolves the yield module) and edit it mid-run;
	// run logs stay in a separate temp dir.
	src, err := filepath.Abs(filepath.Join("testdata", "skill-basic"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join("testdata"), "tmp-skill-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if dir, err = filepath.Abs(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(src, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Engine{SkillDir: dir, RunsDir: t.TempDir(), Stderr: os.Stderr}

	p, err := e.StartRun(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Change the skill source mid-run: resume must refuse without migration.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), append(b, []byte("\n// edited mid-run\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = respond(t, e, p, `{"value":"preserve"}`, false)
	if err == nil || !strings.Contains(err.Error(), "digest-mismatch") {
		t.Fatalf("digest mismatch must be refused, got %v", err)
	}
	// Explicit migration rebinds and proceeds (the edit is a comment, so
	// replay does not diverge).
	p2, err := respond(t, e, p, `{"value":"preserve"}`, true)
	if err != nil {
		t.Fatalf("explicit migration must proceed: %v", err)
	}
	if p2.Envelope == nil || p2.Envelope.Request.ID != "summarize" {
		t.Fatalf("migrated run must advance, got %+v", p2)
	}
	l, err := e.Log(p.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var migrated bool
	for _, ev := range l.Events() {
		if ev.Type == runlog.DigestMigrated {
			migrated = true
		}
	}
	if !migrated {
		t.Fatal("digest.migrated must be recorded")
	}
}

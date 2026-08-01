// Package engine is the supervisor: it owns run creation, subprocess
// execution, the auto-execution of run_command operations, response
// acceptance, and terminal handling. Every state change goes through the
// run log; every refusal goes through the guard.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/operatorstack/yield/internal/guard"
	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

// Engine binds a skill directory to a runs directory.
type Engine struct {
	SkillDir string
	RunsDir  string
	// Stderr receives subprocess diagnostics (compile errors etc.).
	Stderr *os.File
}

// New creates an engine rooted at the skill directory; run logs live in
// <skillDir>/.yield/runs.
func New(skillDir string) (*Engine, error) {
	abs, err := filepath.Abs(skillDir)
	if err != nil {
		return nil, err
	}
	runs, err := runlog.RunsDir(abs)
	if err != nil {
		return nil, err
	}
	return &Engine{SkillDir: abs, RunsDir: runs, Stderr: os.Stderr}, nil
}

// Advance's result: either the next operation for the agent, or a
// terminal status.
type Progress struct {
	RunID    string
	Envelope *protocol.RequestEnvelope
	Terminal *protocol.TerminalOutcome
}

// StartRun creates a run bound to the current skill digest and advances
// to the first agent-facing operation or terminal.
func (e *Engine) StartRun(input json.RawMessage) (*Progress, error) {
	digest, err := protocol.DigestSkillDir(e.SkillDir)
	if err != nil {
		return nil, err
	}
	skill := protocol.SkillRef{Name: filepath.Base(e.SkillDir), Digest: digest}
	runID := newRunID()
	l, err := runlog.Create(e.RunsDir, runID)
	if err != nil {
		return nil, err
	}
	if _, err := l.Append(runlog.RunStarted, map[string]any{
		"run_id": runID, "skill": skill,
		"input_digest": protocol.DigestBytes(input),
	}); err != nil {
		return nil, err
	}
	return e.advance(l, runID)
}

// Resume validates and accepts a response for the pending operation, then
// advances. migrate=true explicitly rebinds the run to the current skill
// digest (the migrate_digest mechanism; divergence detection remains the
// safety net).
func (e *Engine) Resume(runID string, respBytes []byte, migrate bool) (*Progress, error) {
	l, err := runlog.Open(e.RunsDir, runID)
	if err != nil {
		return nil, err
	}
	s, err := guard.Reconstruct(l)
	if err != nil {
		return nil, err
	}
	current, err := protocol.DigestSkillDir(e.SkillDir)
	if err != nil {
		return nil, err
	}
	if err := guard.CheckDigest(s, current, migrate); err != nil {
		return nil, e.rejected(l, err)
	}
	if migrate && s.BoundDigest != current {
		if _, err := l.Append(runlog.DigestMigrated, map[string]string{"from": s.BoundDigest, "to": current}); err != nil {
			return nil, err
		}
	}
	var resp protocol.ResponseEnvelope
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("response does not decode: %w", err)
	}
	if err := guard.CheckResponse(s, resp); err != nil {
		return nil, e.rejected(l, err)
	}
	if err := e.acceptResponse(l, s.Pending, resp); err != nil {
		return nil, err
	}
	return e.advance(l, runID)
}

// Replay re-executes the program against the full journal and verifies it
// reproduces the run's recorded frontier — the determinism check.
func (e *Engine) Replay(runID string) (*Progress, error) {
	l, err := runlog.Open(e.RunsDir, runID)
	if err != nil {
		return nil, err
	}
	s, err := guard.Reconstruct(l)
	if err != nil {
		return nil, err
	}
	out, err := e.execute(l, runID)
	if err != nil {
		return nil, err
	}
	switch out.Type {
	case protocol.OutputDiverged:
		return nil, fmt.Errorf("replay diverged at sequence %d: %s", out.Divergence.Sequence, out.Divergence.Detail)
	case protocol.OutputRequest:
		if s.Pending == nil {
			return nil, fmt.Errorf("replay reached a new operation (seq %d) but the log has no pending operation", out.Envelope.Sequence)
		}
		if protocol.RequestDigest(out.Envelope.Request) != protocol.RequestDigest(s.Pending.Request) {
			return nil, fmt.Errorf("replay reached a different pending operation than recorded")
		}
		return &Progress{RunID: runID, Envelope: out.Envelope}, nil
	case protocol.OutputTerminal:
		return &Progress{RunID: runID, Terminal: out.Terminal}, nil
	}
	return nil, fmt.Errorf("program emitted unknown output type %q", out.Type)
}

// Log opens a run's log for inspection.
func (e *Engine) Log(runID string) (*runlog.Log, error) {
	return runlog.Open(e.RunsDir, runID)
}

// ListRuns returns known run IDs, newest last.
func (e *Engine) ListRuns() ([]string, error) {
	entries, err := os.ReadDir(e.RunsDir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, en := range entries {
		if strings.HasSuffix(en.Name(), ".jsonl") {
			ids = append(ids, strings.TrimSuffix(en.Name(), ".jsonl"))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// advance executes the program and drives it as far as it can go without
// the agent: run_command operations are executed by the engine itself
// (observed fact), everything else is handed back as the next envelope.
func (e *Engine) advance(l *runlog.Log, runID string) (*Progress, error) {
	for {
		out, err := e.execute(l, runID)
		if err != nil {
			return nil, err
		}
		switch out.Type {
		case protocol.OutputDiverged:
			if _, err := l.Append(runlog.ReplayDiverged, out.Divergence); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("replay diverged at sequence %d: %s — the skill program changed behavior mid-run; resume with --accept-new-digest only rebinds sources, divergence always fails the run", out.Divergence.Sequence, out.Divergence.Detail)

		case protocol.OutputRequest:
			env := out.Envelope
			s, err := guard.Reconstruct(l)
			if err != nil {
				return nil, err
			}
			// Idempotent re-emission of the already-pending operation.
			if s.Pending == nil || protocol.RequestDigest(s.Pending.Request) != protocol.RequestDigest(env.Request) || s.Pending.Sequence != env.Sequence {
				if _, err := l.Append(runlog.OperationRequested, env); err != nil {
					return nil, err
				}
			}
			if env.Request.Kind == protocol.OpRunCommand {
				resp, err := e.runCommand(env)
				if err != nil {
					return nil, err
				}
				if err := e.acceptResponse(l, env, resp); err != nil {
					return nil, err
				}
				continue // the program can now advance past this operation
			}
			return &Progress{RunID: runID, Envelope: env}, nil

		case protocol.OutputTerminal:
			return e.terminate(l, runID, out)

		default:
			return nil, fmt.Errorf("program emitted unknown output type %q", out.Type)
		}
	}
}

// execute runs the skill subprocess once against the journal rebuilt from
// the log and returns its single ProgramOutput.
func (e *Engine) execute(l *runlog.Log, runID string) (*protocol.ProgramOutput, error) {
	s, err := guard.Reconstruct(l)
	if err != nil {
		return nil, err
	}
	journal := protocol.Journal{RunID: runID, Skill: s.Skill}
	// Rebuild answered entries in sequence order from the log.
	pendingBySeq := map[int]protocol.Request{}
	results := map[int]json.RawMessage{}
	for _, ev := range l.Events() {
		switch ev.Type {
		case runlog.OperationRequested:
			var env protocol.RequestEnvelope
			if err := ev.Decode(&env); err != nil {
				return nil, err
			}
			pendingBySeq[env.Sequence] = env.Request
		case runlog.OperationCompleted:
			var d struct {
				Sequence int             `json:"sequence"`
				Result   json.RawMessage `json:"result"`
			}
			if err := ev.Decode(&d); err != nil {
				return nil, err
			}
			results[d.Sequence] = d.Result
		}
	}
	for seq := 1; ; seq++ {
		req, ok := pendingBySeq[seq]
		res, done := results[seq]
		if !ok || !done {
			break
		}
		journal.Entries = append(journal.Entries, protocol.JournalEntry{
			Request: req,
			Response: protocol.ResponseEnvelope{
				RunID: runID, Sequence: seq, RequestID: req.ID, Status: "completed", Result: res,
			},
		})
	}
	jf, err := os.CreateTemp("", "yield-journal-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(jf.Name())
	if err := json.NewEncoder(jf).Encode(journal); err != nil {
		return nil, err
	}
	jf.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", ".")
	cmd.Dir = e.SkillDir
	cmd.Env = append(os.Environ(), "YIELD_JOURNAL="+jf.Name())
	cmd.Stderr = e.Stderr
	outBytes, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("skill program failed: %w", err)
	}
	var out protocol.ProgramOutput
	if err := json.Unmarshal(outBytes, &out); err != nil {
		return nil, fmt.Errorf("skill program emitted invalid output: %w", err)
	}
	return &out, nil
}

// runCommand executes a run_command operation itself with a timeout; the
// result enters the log as observed fact.
func (e *Engine) runCommand(env *protocol.RequestEnvelope) (protocol.ResponseEnvelope, error) {
	var p protocol.RunCommandPayload
	if err := json.Unmarshal(env.Request.Payload, &p); err != nil {
		return protocol.ResponseEnvelope{}, err
	}
	timeout := time.Duration(p.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", p.Command)
	cmd.Dir = e.SkillDir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	res := protocol.CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
	} else if exitErr, ok := runErr.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
	} else if runErr != nil {
		return protocol.ResponseEnvelope{}, runErr
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return protocol.ResponseEnvelope{}, err
	}
	return protocol.ResponseEnvelope{
		RunID: env.RunID, Sequence: env.Sequence, RequestID: env.Request.ID,
		Status: "completed", Result: raw,
	}, nil
}

// acceptResponse appends an accepted response as operation.completed.
func (e *Engine) acceptResponse(l *runlog.Log, env *protocol.RequestEnvelope, resp protocol.ResponseEnvelope) error {
	_, err := l.Append(runlog.OperationCompleted, map[string]any{
		"sequence":      resp.Sequence,
		"request_id":    resp.RequestID,
		"result":        resp.Result,
		"result_digest": protocol.DigestBytes(resp.Result),
	})
	return err
}

// terminate closes the run, enforcing evidence-bound completion.
func (e *Engine) terminate(l *runlog.Log, runID string, out *protocol.ProgramOutput) (*Progress, error) {
	s, err := guard.Reconstruct(l)
	if err != nil {
		return nil, err
	}
	term := out.Terminal
	// Record the requirement trail first.
	for _, r := range out.Requirements {
		t := runlog.RequirementPassed
		if !r.Passed {
			t = runlog.RequirementFailed
		}
		if _, err := l.Append(t, r); err != nil {
			return nil, err
		}
	}
	switch term.Status {
	case protocol.StatusCompleted:
		if err := guard.CheckCompletion(s, out.Requirements); err != nil {
			// complete_unproven is forbidden: the run closes blocked, loudly.
			if _, aerr := l.Append(runlog.RunBlocked, map[string]string{"reason": err.Error()}); aerr != nil {
				return nil, aerr
			}
			return nil, err
		}
		if _, err := l.Append(runlog.RunCompleted, map[string]any{
			"result": term.Result, "requirements": len(out.Requirements),
		}); err != nil {
			return nil, err
		}
	case protocol.StatusRequirementFailed, protocol.StatusBlocked:
		if _, err := l.Append(runlog.RunBlocked, map[string]string{"reason": term.Reason}); err != nil {
			return nil, err
		}
	case protocol.StatusRefused:
		if _, err := l.Append(runlog.RunRefused, map[string]string{"reason": term.Reason}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("program emitted unknown terminal status %q", term.Status)
	}
	return &Progress{RunID: runID, Terminal: term}, nil
}

// rejected records a guard refusal in the log and returns it.
func (e *Engine) rejected(l *runlog.Log, err error) error {
	if rej, ok := err.(*guard.Rejection); ok {
		_, _ = l.Append(runlog.ResponseRejected, map[string]string{
			"reason": string(rej.Reason), "detail": rej.Detail,
		})
	}
	return err
}

func newRunID() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return fmt.Sprintf("run_%d_%s", time.Now().UTC().Unix(), hex.EncodeToString(b[:]))
}

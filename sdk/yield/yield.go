// Package yield is the skill-program SDK. A skill is an ordinary Go main
// package that calls Main with a deterministic program. Every side effect
// crosses a yielded primitive; code between yields must be deterministic —
// that is what makes replay-based resume sound.
//
// Execution model (deterministic re-execution): on every run/resume the
// program re-executes from the top. Recorded responses are fed back in
// order; at the first unanswered operation the SDK emits a yield.v1
// request envelope on stdout and exits. If a replayed step produces a
// different operation than the journal recorded, the SDK reports
// divergence and the run fails loudly — it never silently forks.
package yield

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/operatorstack/yield/internal/protocol"
)

// EnvJournal names the environment variable pointing at the journal file
// the supervisor (yskill) hands to the subprocess.
const EnvJournal = "YIELD_JOURNAL"

// Context carries the replay cursor and the primitives.
type Context struct {
	journal      protocol.Journal
	idx          int
	requirements []protocol.Requirement
}

// Outcome is what a program returns on success.
type Outcome struct {
	Result any
}

// Complete finishes the run with a result; evidence is the requirement
// trail accumulated via Require.
func (c *Context) Complete(result any) (Outcome, error) {
	return Outcome{Result: result}, nil
}

// Blocked ends the run at a true frontier, explicitly.
type BlockedError struct{ Reason string }

func (e *BlockedError) Error() string { return "blocked: " + e.Reason }

// Refused ends the run because the skill declines to proceed.
type RefusedError struct{ Reason string }

func (e *RefusedError) Error() string { return "refused: " + e.Reason }

// Blocked returns the terminal blocked error.
func (c *Context) Blocked(reason string) error { return &BlockedError{Reason: reason} }

// Refused returns the terminal refused error.
func (c *Context) Refused(reason string) error { return &RefusedError{Reason: reason} }

// AskUser yields a question to be asked through the host's normal
// interface and returns the selected value on resume.
func (c *Context) AskUser(id, question string, options ...protocol.Option) string {
	payload := mustJSON(protocol.AskUserPayload{Question: question, Options: options})
	valueSchema := map[string]any{"type": "string"}
	if len(options) > 0 {
		values := make([]string, 0, len(options))
		for _, option := range options {
			values = append(values, option.Value)
		}
		valueSchema["enum"] = values
	}
	schema := mustJSON(map[string]any{
		"type": "object", "required": []string{"value"},
		"additionalProperties": false,
		"properties":           map[string]any{"value": valueSchema},
	})
	resp := c.step(protocol.Request{
		ID: id, Kind: protocol.OpAskUser, Payload: payload,
		OutputSchema: schema,
	})
	var r protocol.AskUserResult
	mustDecode(resp.Result, &r)
	return r.Value
}

// AgentTask delegates reasoning to the model. schema (JSON Schema bytes,
// may be nil) is enforced by the supervisor on resume; the returned raw
// message is schema-valid by construction.
func (c *Context) AgentTask(id, instruction string, contextData any, schema json.RawMessage) json.RawMessage {
	var ctxRaw json.RawMessage
	if contextData != nil {
		ctxRaw = mustJSON(contextData)
	}
	payload := mustJSON(protocol.AgentTaskPayload{Instruction: instruction, Context: ctxRaw})
	resp := c.step(protocol.Request{ID: id, Kind: protocol.OpAgentTask, Payload: payload, OutputSchema: schema})
	return resp.Result
}

// RunCommand yields a command that yskill executes itself — the result is
// observed fact, not the agent's account of it.
func (c *Context) RunCommand(id, command string, timeoutSeconds int) protocol.CommandResult {
	payload := mustJSON(protocol.RunCommandPayload{Command: command, TimeoutSeconds: timeoutSeconds})
	resp := c.step(protocol.Request{ID: id, Kind: protocol.OpRunCommand, Payload: payload})
	var r protocol.CommandResult
	mustDecode(resp.Result, &r)
	return r
}

// Require binds a claim to evidence. A failed requirement terminates the
// program immediately with a requirement_failed outcome; completion is
// structurally unreachable past a failed requirement.
func (c *Context) Require(ok bool, claim string, evidence any) {
	req := protocol.Requirement{Claim: claim, Passed: ok}
	if evidence != nil {
		req.EvidenceDigest = protocol.DigestBytes(mustJSON(evidence))
	}
	c.requirements = append(c.requirements, req)
	if !ok {
		emit(protocol.ProgramOutput{
			Type:         protocol.OutputTerminal,
			Terminal:     &protocol.TerminalOutcome{Status: protocol.StatusRequirementFailed, Reason: claim},
			Requirements: c.requirements,
		})
	}
}

// step is the yield mechanism: replay if recorded, emit-and-exit if not,
// diverge loudly if the program no longer matches its own history.
func (c *Context) step(req protocol.Request) protocol.ResponseEnvelope {
	seq := c.idx + 1
	if c.idx < len(c.journal.Entries) {
		entry := c.journal.Entries[c.idx]
		want := protocol.RequestDigest(entry.Request)
		got := protocol.RequestDigest(req)
		if want != got {
			emit(protocol.ProgramOutput{
				Type: protocol.OutputDiverged,
				Divergence: &protocol.Divergence{
					Sequence: seq, Expected: want, Got: got,
					Detail: fmt.Sprintf("replay produced operation %q (%s) where the journal recorded %q (%s)", req.ID, req.Kind, entry.Request.ID, entry.Request.Kind),
				},
			})
		}
		c.idx++
		return entry.Response
	}
	c.idx++
	emit(protocol.ProgramOutput{
		Type: protocol.OutputRequest,
		Envelope: &protocol.RequestEnvelope{
			Protocol: protocol.Version,
			RunID:    c.journal.RunID,
			Skill:    c.journal.Skill,
			Sequence: seq,
			Request:  req,
		},
		Requirements: c.requirements,
	})
	panic("unreachable")
}

// Main runs a skill program under the supervisor protocol. It reads the
// journal named by YIELD_JOURNAL, executes the program, and emits exactly
// one ProgramOutput on stdout.
func Main(program func(*Context) (Outcome, error)) {
	path := os.Getenv(EnvJournal)
	if path == "" {
		fmt.Fprintln(os.Stderr, "yield: YIELD_JOURNAL is not set; this program is run by yskill, not directly")
		os.Exit(2)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yield: cannot read journal: %v\n", err)
		os.Exit(2)
	}
	var j protocol.Journal
	if err := json.Unmarshal(b, &j); err != nil {
		fmt.Fprintf(os.Stderr, "yield: corrupt journal: %v\n", err)
		os.Exit(2)
	}
	ctx := &Context{journal: j}
	out, err := program(ctx)
	if err != nil {
		switch e := err.(type) {
		case *BlockedError:
			emit(protocol.ProgramOutput{Type: protocol.OutputTerminal,
				Terminal:     &protocol.TerminalOutcome{Status: protocol.StatusBlocked, Reason: e.Reason},
				Requirements: ctx.requirements})
		case *RefusedError:
			emit(protocol.ProgramOutput{Type: protocol.OutputTerminal,
				Terminal:     &protocol.TerminalOutcome{Status: protocol.StatusRefused, Reason: e.Reason},
				Requirements: ctx.requirements})
		default:
			fmt.Fprintf(os.Stderr, "yield: program error: %v\n", err)
			os.Exit(1)
		}
	}
	emit(protocol.ProgramOutput{Type: protocol.OutputTerminal,
		Terminal:     &protocol.TerminalOutcome{Status: protocol.StatusCompleted, Result: mustJSON(out.Result)},
		Requirements: ctx.requirements})
}

// emit writes the single ProgramOutput and exits the process. A skill
// execution produces exactly one output.
func emit(out protocol.ProgramOutput) {
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "yield: cannot emit output: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func mustJSON(v any) json.RawMessage {
	if raw, ok := v.(json.RawMessage); ok {
		return raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("yield: unmarshalable value: %v", err))
	}
	return b
}

func mustDecode(raw json.RawMessage, v any) {
	if err := json.Unmarshal(raw, v); err != nil {
		panic(fmt.Sprintf("yield: recorded response does not decode: %v", err))
	}
}

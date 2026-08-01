// Package protocol defines the yield.v1 wire protocol: the typed operation
// envelopes a skill program yields to the coding agent, and the response
// envelopes the agent (or yskill itself) feeds back. Everything that crosses
// a process boundary is defined here and nowhere else.
package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Version is the protocol identifier carried by every request envelope.
const Version = "yield.v1"

// OpKind is the closed set of operations a skill program may yield.
type OpKind string

const (
	OpAskUser    OpKind = "ask_user"
	OpAgentTask  OpKind = "agent_task"
	OpRunCommand OpKind = "run_command"
)

// SkillRef identifies the skill a run is bound to. Digest is the
// content digest of the skill source; responses against a different
// digest are rejected unless explicitly migrated.
type SkillRef struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Digest  string `json:"digest"`
}

// Request is one yielded operation.
type Request struct {
	ID           string          `json:"id"`
	Kind         OpKind          `json:"kind"`
	Payload      json.RawMessage `json:"payload"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
}

// RequestEnvelope is what the agent sees: one operation, bound to a run,
// a sequence number, and the skill digest.
type RequestEnvelope struct {
	Protocol string   `json:"protocol"`
	RunID    string   `json:"run_id"`
	Skill    SkillRef `json:"skill"`
	Sequence int      `json:"sequence"`
	Request  Request  `json:"request"`
}

// ResponseEnvelope is what comes back for exactly one pending request.
type ResponseEnvelope struct {
	RunID     string          `json:"run_id"`
	Sequence  int             `json:"sequence"`
	RequestID string          `json:"request_id"`
	Status    string          `json:"status"` // "completed" | "failed"
	Result    json.RawMessage `json:"result"`
}

// AskUserPayload asks a question through the host's normal interface.
type AskUserPayload struct {
	Question string   `json:"question"`
	Options  []Option `json:"options,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// AskUserResult is the expected result shape for ask_user.
type AskUserResult struct {
	Value string `json:"value"`
}

// AgentTaskPayload delegates a reasoning task to the model.
type AgentTaskPayload struct {
	Instruction string          `json:"instruction"`
	Context     json.RawMessage `json:"context,omitempty"`
}

// RunCommandPayload names a command yskill executes itself, so the result
// enters the log as observed fact rather than the agent's transcription.
type RunCommandPayload struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// CommandResult is the observed outcome of a run_command operation.
type CommandResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	TimedOut bool   `json:"timed_out,omitempty"`
}

// Requirement is a claim bound to evidence; a failed requirement prevents
// run completion.
type Requirement struct {
	Claim          string `json:"claim"`
	Passed         bool   `json:"passed"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}

// Program output types: a skill subprocess emits exactly one ProgramOutput
// per execution — the next yielded request, a terminal outcome, or a
// replay divergence report.
type OutputKind string

const (
	OutputRequest  OutputKind = "request"
	OutputTerminal OutputKind = "terminal"
	OutputDiverged OutputKind = "diverged"
)

// Terminal statuses.
type TerminalStatus string

const (
	StatusCompleted         TerminalStatus = "completed"
	StatusBlocked           TerminalStatus = "blocked"
	StatusRefused           TerminalStatus = "refused"
	StatusRequirementFailed TerminalStatus = "requirement_failed"
)

type ProgramOutput struct {
	Type         OutputKind       `json:"type"`
	Envelope     *RequestEnvelope `json:"envelope,omitempty"`
	Terminal     *TerminalOutcome `json:"terminal,omitempty"`
	Divergence   *Divergence      `json:"divergence,omitempty"`
	Requirements []Requirement    `json:"requirements,omitempty"`
}

type TerminalOutcome struct {
	Status TerminalStatus  `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Reason string          `json:"reason,omitempty"`
}

// Divergence reports that replay produced a different operation at a
// recorded sequence — nondeterminism leaked between yields. It always
// fails the run loudly.
type Divergence struct {
	Sequence int    `json:"sequence"`
	Expected string `json:"expected_digest"`
	Got      string `json:"got_digest"`
	Detail   string `json:"detail,omitempty"`
}

// InvalidProgramOutputError means a skill emitted bytes that are not one
// complete yield.v1 output variant. The engine must not dispatch such output.
type InvalidProgramOutputError struct {
	Reason string
}

func (e *InvalidProgramOutputError) Error() string {
	return "invalid program output: " + e.Reason
}

// DecodeProgramOutput is the single admission boundary between an SDK
// subprocess and engine authority. It accepts exactly one complete variant
// and rejects unknown fields so future protocol changes fail closed until the
// IR, decoder, and dispatch logic are upgraded together.
func DecodeProgramOutput(raw []byte) (*ProgramOutput, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var out ProgramOutput
	if err := dec.Decode(&out); err != nil {
		return nil, &InvalidProgramOutputError{Reason: err.Error()}
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return nil, &InvalidProgramOutputError{Reason: err.Error()}
	}
	if err := out.validate(); err != nil {
		return nil, &InvalidProgramOutputError{Reason: err.Error()}
	}
	return &out, nil
}

func (out ProgramOutput) validate() error {
	active := 0
	if out.Envelope != nil {
		active++
	}
	if out.Terminal != nil {
		active++
	}
	if out.Divergence != nil {
		active++
	}
	if active != 1 {
		return fmt.Errorf("expected exactly one variant payload, got %d", active)
	}
	switch out.Type {
	case OutputRequest:
		if out.Envelope == nil {
			return fmt.Errorf("request output requires envelope")
		}
		if err := out.Envelope.validate(); err != nil {
			return err
		}
	case OutputTerminal:
		if out.Terminal == nil {
			return fmt.Errorf("terminal output requires terminal")
		}
		switch out.Terminal.Status {
		case StatusCompleted, StatusBlocked, StatusRefused, StatusRequirementFailed:
		default:
			return fmt.Errorf("unknown terminal status %q", out.Terminal.Status)
		}
	case OutputDiverged:
		if out.Divergence == nil {
			return fmt.Errorf("diverged output requires divergence")
		}
		if out.Divergence.Sequence < 1 {
			return fmt.Errorf("divergence sequence must be positive")
		}
		if !validDigest(out.Divergence.Expected) || !validDigest(out.Divergence.Got) {
			return fmt.Errorf("divergence digests must be sha256 digests")
		}
	default:
		return fmt.Errorf("unknown output type %q", out.Type)
	}
	for _, requirement := range out.Requirements {
		if requirement.Claim == "" {
			return fmt.Errorf("requirement claim must not be empty")
		}
		if requirement.EvidenceDigest != "" && !validDigest(requirement.EvidenceDigest) {
			return fmt.Errorf("requirement evidence must be a sha256 digest")
		}
	}
	return nil
}

func (env RequestEnvelope) validate() error {
	if env.Protocol != Version {
		return fmt.Errorf("request protocol must be %q", Version)
	}
	if env.RunID == "" || env.Skill.Name == "" || !validDigest(env.Skill.Digest) {
		return fmt.Errorf("request run and skill identity are incomplete")
	}
	if env.Sequence < 1 || env.Request.ID == "" {
		return fmt.Errorf("request sequence and id are required")
	}
	switch env.Request.Kind {
	case OpAskUser, OpAgentTask, OpRunCommand:
	default:
		return fmt.Errorf("unknown operation kind %q", env.Request.Kind)
	}
	if !json.Valid(env.Request.Payload) {
		return fmt.Errorf("request payload must be valid JSON")
	}
	if len(env.Request.OutputSchema) > 0 && !json.Valid(env.Request.OutputSchema) {
		return fmt.Errorf("request output schema must be valid JSON")
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

// JournalEntry is one recorded request/response pair handed to the skill
// subprocess for replay.
type JournalEntry struct {
	Request  Request          `json:"request"`
	Response ResponseEnvelope `json:"response"`
}

// Journal is the replay input: the run identity plus all answered
// operations in sequence order.
type Journal struct {
	RunID   string         `json:"run_id"`
	Skill   SkillRef       `json:"skill"`
	Entries []JournalEntry `json:"entries"`
}

// DigestBytes returns the canonical content digest string for a byte slice.
func DigestBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// RequestDigest is the canonical digest of a request, used for replay
// divergence detection: kind, id, payload, and output schema all bind.
// JSON fields are compacted first so that a request digests identically
// before and after a round-trip through the log (encoding/json compacts
// embedded RawMessage on marshal).
func RequestDigest(r Request) string {
	var buf bytes.Buffer
	buf.WriteString(string(r.Kind))
	buf.WriteByte(0)
	buf.WriteString(r.ID)
	buf.WriteByte(0)
	buf.Write(compactJSON(r.Payload))
	buf.WriteByte(0)
	buf.Write(compactJSON(r.OutputSchema))
	return DigestBytes(buf.Bytes())
}

func compactJSON(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw
	}
	return buf.Bytes()
}

// DigestSkillDir computes the skill source digest: sha256 over the sorted
// relative paths and contents of *.go, SKILL.md, and go.mod files.
func DigestSkillDir(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".yield" || name == "fixtures" || strings.HasPrefix(name, ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		base := d.Name()
		switch {
		case strings.HasSuffix(base, ".go"), strings.HasSuffix(base, ".ts"),
			strings.HasSuffix(base, ".js"), strings.HasSuffix(base, ".mjs"),
			strings.HasSuffix(base, ".py"):
			files = append(files, path)
		case base == "SKILL.md", base == "go.mod", base == "skill.json",
			base == "package.json", base == "pyproject.toml", base == "requirements.txt":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(b))
		h.Write(b)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ValidateResult checks a completed result against the request's embedded
// JSON schema. A nil schema accepts any JSON value.
func ValidateResult(schema, result json.RawMessage) error {
	if len(schema) == 0 {
		return nil
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return fmt.Errorf("output schema is not valid JSON: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", doc); err != nil {
		return err
	}
	compiled, err := c.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("output schema does not compile: %w", err)
	}
	val, err := jsonschema.UnmarshalJSON(bytes.NewReader(result))
	if err != nil {
		return fmt.Errorf("result is not valid JSON: %w", err)
	}
	if err := compiled.Validate(val); err != nil {
		return fmt.Errorf("schema-invalid result: %w", err)
	}
	return nil
}

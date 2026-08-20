// Package observation projects privacy-safe, portable observations from Yield's
// authoritative append-only run journal.
package observation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

const (
	Schema = "yield.observation.v1"
	Kind   = "run_receipt"
)

// RunReceipt is the canonical privacy-safe projection of one journal prefix.
type RunReceipt struct {
	Schema             string                     `json:"schema"`
	Kind               string                     `json:"kind"`
	ReceiptDigest      string                     `json:"receipt_digest,omitempty"`
	Journal            JournalBinding             `json:"journal"`
	Run                RunIdentity                `json:"run"`
	Skill              SkillIdentity              `json:"skill"`
	Runtime            *RuntimeIdentity           `json:"runtime,omitempty"`
	Timing             TimingSummary              `json:"timing"`
	Operations         []OperationObservation     `json:"operations"`
	OperationSummaries []OperationSummary         `json:"operation_summaries"`
	Outcome            OutcomeSummary             `json:"outcome"`
	Requirements       []RequirementOutcome       `json:"requirements"`
	ResponseRejections []ResponseRejectionSummary `json:"response_rejections"`
	Divergences        []DivergenceOutcome        `json:"divergences"`
	Experiment         *ExperimentContext         `json:"experiment,omitempty"`
}

// OperationKind is one supervisor-observed Yield operation kind.
type OperationKind string

const (
	OperationAskUser    OperationKind = "ask_user"
	OperationAgentTask  OperationKind = "agent_task"
	OperationRunCommand OperationKind = "run_command"
)

// JournalBinding identifies the exact authoritative journal prefix projected.
type JournalBinding struct {
	RunID        string `json:"run_id"`
	HeadSequence int    `json:"head_sequence"`
	HeadDigest   string `json:"head_digest"`
}

// RunIdentity identifies the observed run and its privacy-safe input digest.
type RunIdentity struct {
	ID          string `json:"id"`
	InputDigest string `json:"input_digest,omitempty"`
}

// SkillIdentity records the skill facts available in the journal.
type SkillIdentity struct {
	Name          string         `json:"name"`
	Version       string         `json:"version,omitempty"`
	BindingDigest string         `json:"binding_digest,omitempty"`
	SourceDigest  *ProfileDigest `json:"source_digest,omitempty"`
}

// ProfileDigest names a versioned digest profile and its value.
type ProfileDigest struct {
	Profile string `json:"profile"`
	Value   string `json:"value"`
}

// RuntimeIdentity records authoritative supervisor compatibility facts.
type RuntimeIdentity struct {
	SupervisorVersion string `json:"supervisor_version,omitempty"`
	RequiredVersion   string `json:"required_version,omitempty"`
	Compatible        *bool  `json:"compatible,omitempty"`
}

// TimingSummary contains journal timestamps and safe derived timing.
type TimingSummary struct {
	StartedAt      string `json:"started_at"`
	LastObservedAt string `json:"last_observed_at"`
	EndedAt        string `json:"ended_at,omitempty"`
	ElapsedMS      *int64 `json:"elapsed_ms,omitempty"`
	ClockAnomaly   bool   `json:"clock_anomaly,omitempty"`
}

// OperationObservation describes one supervisor-observed operation.
type OperationObservation struct {
	Sequence           int           `json:"sequence"`
	Kind               OperationKind `json:"kind"`
	OperationKeyDigest string        `json:"operation_key_digest"`
	RequestedAt        string        `json:"requested_at"`
	CompletedAt        string        `json:"completed_at,omitempty"`
	ElapsedMS          *int64        `json:"elapsed_ms,omitempty"`
	ResultDigest       string        `json:"result_digest,omitempty"`
	ClockAnomaly       bool          `json:"clock_anomaly,omitempty"`
}

// OperationSummary aggregates observations of one operation kind.
type OperationSummary struct {
	Kind           OperationKind `json:"kind"`
	Requested      int           `json:"requested"`
	Completed      int           `json:"completed"`
	TotalElapsedMS int64         `json:"total_elapsed_ms"`
}

// OutcomeSummary classifies the latest lifecycle and terminal outcome.
type OutcomeSummary struct {
	Phase               string `json:"phase"`
	TerminalDisposition string `json:"terminal_disposition,omitempty"`
	TerminalCause       string `json:"terminal_cause,omitempty"`
	ResultDigest        string `json:"result_digest,omitempty"`
	FailureCode         string `json:"failure_code,omitempty"`
}

// RequirementOutcome records a requirement result without its raw claim.
type RequirementOutcome struct {
	Outcome        string `json:"outcome"`
	ClaimDigest    string `json:"claim_digest"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}

// ResponseRejectionSummary counts one closed response-rejection reason.
type ResponseRejectionSummary struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// DivergenceOutcome records expected and actual digests at a replay sequence.
type DivergenceOutcome struct {
	Sequence int    `json:"sequence"`
	Expected string `json:"expected_digest"`
	Got      string `json:"got_digest"`
}

// ExperimentContext groups a run for external evaluation without selecting a winner.
type ExperimentContext struct {
	ExperimentID       string `json:"experiment_id"`
	CohortID           string `json:"cohort_id,omitempty"`
	VariantID          string `json:"variant_id"`
	Role               string `json:"role"`
	BaselineVariantID  string `json:"baseline_variant_id,omitempty"`
	ParentSkillVersion string `json:"parent_skill_version,omitempty"`
}

type snapshot struct {
	Bytes  []byte
	Events []runlog.Event
}

type runOpenedData struct {
	RunID             string             `json:"run_id"`
	SkillName         string             `json:"skill_name"`
	InputDigest       string             `json:"input_digest"`
	SupervisorVersion string             `json:"supervisor_version"`
	Experiment        *ExperimentContext `json:"experiment,omitempty"`
}

type runStartedData struct {
	RunID               string             `json:"run_id"`
	Skill               protocol.SkillRef  `json:"skill"`
	InputDigest         string             `json:"input_digest"`
	SupervisorVersion   string             `json:"supervisor_version"`
	RequiredVersion     string             `json:"required_yield_version"`
	SourceDigestProfile string             `json:"source_digest_profile"`
	SourceDigest        string             `json:"source_digest"`
	Experiment          *ExperimentContext `json:"experiment,omitempty"`
}

type operationCompletedData struct {
	Sequence     int             `json:"sequence"`
	RequestID    string          `json:"request_id"`
	Result       json.RawMessage `json:"result"`
	ResultDigest string          `json:"result_digest"`
}

// Project deterministically derives and seals one receipt from an exact
// append-only run-journal prefix. The prefix must end at a complete JSONL
// record and is never rewritten.
func Project(journalPrefix []byte) (RunReceipt, error) {
	events, err := runlog.ParseSnapshot(journalPrefix)
	if err != nil {
		return RunReceipt{}, err
	}
	projected, err := project(snapshot{Bytes: journalPrefix, Events: events})
	if err != nil {
		return RunReceipt{}, err
	}
	return *projected, nil
}

func project(snapshot snapshot) (*RunReceipt, error) {
	if len(snapshot.Events) == 0 {
		return nil, fmt.Errorf("receipt: journal is empty")
	}
	if len(snapshot.Bytes) == 0 {
		return nil, fmt.Errorf("receipt: journal bytes are empty")
	}

	r := &RunReceipt{
		Schema:             Schema,
		Kind:               Kind,
		Operations:         []OperationObservation{},
		OperationSummaries: []OperationSummary{},
		Requirements:       []RequirementOutcome{},
		ResponseRejections: []ResponseRejectionSummary{},
		Divergences:        []DivergenceOutcome{},
	}
	r.Journal.HeadDigest = digest("", snapshot.Bytes)
	r.Journal.HeadSequence = len(snapshot.Events)

	operations := map[int]*OperationObservation{}
	requestIDs := map[int]string{}
	rejections := map[string]int{}
	var started, last, ended time.Time
	var previous time.Time
	var terminal bool
	var openedSeen, startedSeen bool
	var initializationFailed, recoverableFailed bool
	var initializationCode, executionCode string
	var requirementFailed bool
	var lastDivergenceSeq int

	for index, event := range snapshot.Events {
		if event.Seq != index+1 {
			return nil, fmt.Errorf("receipt: event sequence %d, want %d", event.Seq, index+1)
		}
		if event.At.IsZero() {
			return nil, fmt.Errorf("receipt: event %d has no timestamp", event.Seq)
		}
		if !previous.IsZero() && event.At.UTC().Before(previous) {
			r.Timing.ClockAnomaly = true
		}
		previous = event.At.UTC()
		last = event.At.UTC()
		if terminal && event.Type != runlog.ResponseRejected {
			return nil, fmt.Errorf("receipt: event %d follows a terminal event", event.Seq)
		}

		switch event.Type {
		case runlog.RunOpened:
			if openedSeen || startedSeen || index != 0 {
				return nil, fmt.Errorf("receipt: run.opened must be the first event")
			}
			openedSeen = true
			var data runOpenedData
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			r.Run.ID = data.RunID
			r.Run.InputDigest = data.InputDigest
			r.Skill.Name = data.SkillName
			r.Runtime = runtimeIdentity(data.SupervisorVersion, "")
			r.Experiment = data.Experiment
			started = event.At.UTC()

		case runlog.RunStarted:
			if startedSeen {
				return nil, fmt.Errorf("receipt: duplicate run.started event")
			}
			startedSeen = true
			var data runStartedData
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			if r.Run.ID != "" && r.Run.ID != data.RunID {
				return nil, fmt.Errorf("receipt: run identity changed from %s to %s", r.Run.ID, data.RunID)
			}
			if r.Skill.Name != "" && r.Skill.Name != data.Skill.Name {
				return nil, fmt.Errorf("receipt: skill identity changed from %s to %s", r.Skill.Name, data.Skill.Name)
			}
			if r.Run.InputDigest != "" && data.InputDigest != "" && r.Run.InputDigest != data.InputDigest {
				return nil, fmt.Errorf("receipt: input digest changed at run start")
			}
			r.Run.ID = data.RunID
			r.Run.InputDigest = data.InputDigest
			r.Skill.Name = data.Skill.Name
			r.Skill.Version = data.Skill.Version
			r.Skill.BindingDigest = data.Skill.Digest
			if data.SourceDigestProfile != "" && data.SourceDigest != "" {
				r.Skill.SourceDigest = &ProfileDigest{Profile: data.SourceDigestProfile, Value: data.SourceDigest}
			}
			r.Runtime = runtimeIdentity(data.SupervisorVersion, data.RequiredVersion)
			if data.Experiment != nil {
				r.Experiment = data.Experiment
			}
			if started.IsZero() {
				started = event.At.UTC()
			}

		case runlog.DigestMigrated:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: digest migration precedes run start")
			}
			var data struct {
				To string `json:"to"`
			}
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			r.Skill.BindingDigest = data.To
			if r.Skill.SourceDigest != nil {
				r.Skill.SourceDigest.Value = data.To
			}

		case runlog.OperationRequested:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: operation precedes run start")
			}
			var envelope protocol.RequestEnvelope
			if err := event.Decode(&envelope); err != nil {
				return nil, err
			}
			if envelope.RunID != r.Run.ID {
				return nil, fmt.Errorf("receipt: operation sequence %d has a different run id", envelope.Sequence)
			}
			if envelope.Skill.Digest != r.Skill.BindingDigest {
				return nil, fmt.Errorf("receipt: operation sequence %d has a different skill binding", envelope.Sequence)
			}
			if operations[envelope.Sequence] != nil {
				return nil, fmt.Errorf("receipt: duplicate operation sequence %d", envelope.Sequence)
			}
			operation := &OperationObservation{
				Sequence:           envelope.Sequence,
				Kind:               OperationKind(envelope.Request.Kind),
				OperationKeyDigest: digest("yield.operation.v1", []byte(string(envelope.Request.Kind)+"\x00"+envelope.Request.ID)),
				RequestedAt:        formatTime(event.At),
			}
			operations[envelope.Sequence] = operation
			requestIDs[envelope.Sequence] = envelope.Request.ID
			recoverableFailed = false

		case runlog.OperationCompleted:
			var data operationCompletedData
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			operation := operations[data.Sequence]
			if operation == nil {
				return nil, fmt.Errorf("receipt: completion at sequence %d has no request", data.Sequence)
			}
			if operation.CompletedAt != "" {
				return nil, fmt.Errorf("receipt: duplicate completion at sequence %d", data.Sequence)
			}
			if requestIDs[data.Sequence] != data.RequestID {
				return nil, fmt.Errorf("receipt: completion at sequence %d has a different request id", data.Sequence)
			}
			operation.CompletedAt = formatTime(event.At)
			operation.ResultDigest = data.ResultDigest
			if operation.ResultDigest == "" && len(data.Result) > 0 {
				operation.ResultDigest = protocol.DigestBytes(data.Result)
			}
			requestedAt, _ := time.Parse(time.RFC3339Nano, operation.RequestedAt)
			setDuration(&operation.ElapsedMS, &operation.ClockAnomaly, requestedAt, event.At.UTC())
			recoverableFailed = false

		case runlog.ResponseRejected:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: response rejection precedes run start")
			}
			var data struct {
				Reason string `json:"reason"`
			}
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			if data.Reason != "" {
				rejections[data.Reason]++
			}

		case runlog.RequirementPassed, runlog.RequirementFailed:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: requirement precedes run start")
			}
			var data protocol.Requirement
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			outcome := "passed"
			if event.Type == runlog.RequirementFailed {
				outcome = "failed"
				requirementFailed = true
			}
			r.Requirements = append(r.Requirements, RequirementOutcome{
				Outcome: outcome, ClaimDigest: digest("yield.requirement.claim.v1", []byte(data.Claim)), EvidenceDigest: data.EvidenceDigest,
			})

		case runlog.ReplayDiverged:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: divergence precedes run start")
			}
			var data protocol.Divergence
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			r.Divergences = append(r.Divergences, DivergenceOutcome{Sequence: data.Sequence, Expected: data.Expected, Got: data.Got})
			lastDivergenceSeq = event.Seq

		case runlog.ExecutionFailed:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: execution failure precedes run start")
			}
			var data struct {
				Code string `json:"code"`
			}
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			recoverableFailed = true
			executionCode = data.Code

		case runlog.RunInitializationFailed:
			if startedSeen || !openedSeen {
				return nil, fmt.Errorf("receipt: initialization failure has invalid ordering")
			}
			var data struct {
				Code string `json:"code"`
			}
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			initializationFailed = true
			initializationCode = data.Code
			terminal = true
			ended = event.At.UTC()

		case runlog.RunCompleted:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: completion precedes run start")
			}
			var data struct {
				Result json.RawMessage `json:"result"`
			}
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			terminal = true
			ended = event.At.UTC()
			r.Outcome.TerminalDisposition = "completed"
			r.Outcome.TerminalCause = "completed"
			if len(data.Result) > 0 {
				r.Outcome.ResultDigest = protocol.DigestBytes(data.Result)
			}

		case runlog.RunBlocked:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: blocked terminal precedes run start")
			}
			var data struct {
				Cause  string `json:"cause"`
				Reason string `json:"reason"`
			}
			if err := event.Decode(&data); err != nil {
				return nil, err
			}
			terminal = true
			ended = event.At.UTC()
			r.Outcome.TerminalDisposition = "blocked"
			r.Outcome.TerminalCause = data.Cause
			if r.Outcome.TerminalCause == "" {
				switch {
				case requirementFailed:
					r.Outcome.TerminalCause = "requirement_failed"
				case strings.Contains(data.Reason, "completion-unproven"):
					r.Outcome.TerminalCause = "completion_unproven"
				default:
					r.Outcome.TerminalCause = "blocked"
				}
			}

		case runlog.RunRefused:
			if !startedSeen {
				return nil, fmt.Errorf("receipt: refusal precedes run start")
			}
			terminal = true
			ended = event.At.UTC()
			r.Outcome.TerminalDisposition = "refused"
			r.Outcome.TerminalCause = "refused"

		default:
			return nil, fmt.Errorf("receipt: unsupported journal event type %q", event.Type)
		}
	}

	if r.Run.ID == "" {
		return nil, fmt.Errorf("receipt: journal has no run identity")
	}
	if r.Skill.Name == "" {
		return nil, fmt.Errorf("receipt: journal has no skill identity")
	}
	if r.Skill.BindingDigest == "" && !initializationFailed {
		return nil, fmt.Errorf("receipt: started run has no binding digest")
	}
	r.Journal.RunID = r.Run.ID
	r.Timing.StartedAt = formatTime(started)
	r.Timing.LastObservedAt = formatTime(last)
	if terminal {
		r.Timing.EndedAt = formatTime(ended)
		setDuration(&r.Timing.ElapsedMS, &r.Timing.ClockAnomaly, started, ended)
	}

	sequences := make([]int, 0, len(operations))
	for sequence := range operations {
		sequences = append(sequences, sequence)
	}
	sort.Ints(sequences)
	summaries := map[OperationKind]*OperationSummary{}
	for _, sequence := range sequences {
		operation := operations[sequence]
		r.Operations = append(r.Operations, *operation)
		summary := summaries[operation.Kind]
		if summary == nil {
			summary = &OperationSummary{Kind: operation.Kind}
			summaries[operation.Kind] = summary
		}
		summary.Requested++
		if operation.CompletedAt != "" {
			summary.Completed++
		}
		if operation.ElapsedMS != nil {
			summary.TotalElapsedMS += *operation.ElapsedMS
		}
	}
	for _, kind := range []OperationKind{OperationAskUser, OperationAgentTask, OperationRunCommand} {
		if summary := summaries[kind]; summary != nil {
			r.OperationSummaries = append(r.OperationSummaries, *summary)
		}
	}
	reasons := make([]string, 0, len(rejections))
	for reason := range rejections {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		r.ResponseRejections = append(r.ResponseRejections, ResponseRejectionSummary{Reason: reason, Count: rejections[reason]})
	}

	switch {
	case initializationFailed:
		r.Outcome.Phase = "initialization_failed"
		r.Outcome.FailureCode = initializationCode
	case terminal:
		r.Outcome.Phase = "terminal"
	case recoverableFailed:
		r.Outcome.Phase = "recoverable_error"
		r.Outcome.FailureCode = executionCode
	case lastDivergenceSeq > 0:
		r.Outcome.Phase = "diverged"
	case pendingOperation(r.Operations):
		r.Outcome.Phase = "awaiting_response"
	case r.Skill.BindingDigest == "":
		r.Outcome.Phase = "initializing"
	default:
		r.Outcome.Phase = "advancing"
	}
	if r.Experiment != nil {
		if err := r.Experiment.Validate(); err != nil {
			return nil, fmt.Errorf("receipt: invalid experiment context: %w", err)
		}
	}

	if err := seal(r); err != nil {
		return nil, err
	}
	return r, nil
}

func runtimeIdentity(supervisor, required string) *RuntimeIdentity {
	if supervisor == "dev" {
		supervisor = ""
	}
	if supervisor == "" && required == "" {
		return nil
	}
	runtime := &RuntimeIdentity{SupervisorVersion: supervisor, RequiredVersion: required}
	if supervisor != "" && required != "" {
		compatible := supervisor == required
		runtime.Compatible = &compatible
	}
	return runtime
}

func pendingOperation(operations []OperationObservation) bool {
	return len(operations) > 0 && operations[len(operations)-1].CompletedAt == ""
}

func setDuration(target **int64, anomaly *bool, start, end time.Time) {
	if start.IsZero() || end.IsZero() {
		return
	}
	if end.Before(start) {
		*anomaly = true
		return
	}
	value := end.Sub(start).Milliseconds()
	*target = &value
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func digest(domain string, value []byte) string {
	h := sha256.New()
	if domain != "" {
		h.Write([]byte(domain))
		h.Write([]byte{0})
	}
	h.Write(value)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func seal(receipt *RunReceipt) error {
	receipt.ReceiptDigest = ""
	if err := receipt.Validate(); err != nil {
		return err
	}
	body, err := CanonicalBytes(*receipt)
	if err != nil {
		return err
	}
	receipt.ReceiptDigest = digest("", body)
	return nil
}

// VerifyCanonical proves that bytes are the canonical encoding named by the
// receipt digest.
func VerifyCanonical(receipt RunReceipt, raw []byte) error {
	if receipt.ReceiptDigest == "" {
		return fmt.Errorf("receipt: missing receipt digest")
	}
	want := receipt.ReceiptDigest
	copy := receipt
	if err := seal(&copy); err != nil {
		return err
	}
	if copy.ReceiptDigest != want {
		return fmt.Errorf("receipt: digest verification failed")
	}
	canonical, err := CanonicalBytes(copy)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, raw) {
		return fmt.Errorf("receipt: bytes are not canonical JSON")
	}
	return nil
}

// Parse strictly decodes and verifies one canonical RunReceipt.
func Parse(raw []byte) (RunReceipt, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var receipt RunReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return RunReceipt{}, fmt.Errorf("receipt: decode: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return RunReceipt{}, fmt.Errorf("receipt: trailing content: %w", err)
	}
	if err := VerifyCanonical(receipt, raw); err != nil {
		return RunReceipt{}, err
	}
	return receipt, nil
}

// Validate checks the closed Go representation against the public contract.
func (receipt *RunReceipt) Validate() error {
	if receipt == nil || receipt.Schema != Schema || receipt.Kind != Kind {
		return fmt.Errorf("receipt: invalid schema or kind")
	}
	if receipt.Journal.RunID == "" || receipt.Journal.HeadSequence < 1 || !validDigest(receipt.Journal.HeadDigest) || receipt.Run.ID != receipt.Journal.RunID || receipt.Skill.Name == "" {
		return fmt.Errorf("receipt: incomplete journal, run, or skill identity")
	}
	for _, value := range []string{receipt.ReceiptDigest, receipt.Run.InputDigest, receipt.Skill.BindingDigest, receipt.Outcome.ResultDigest} {
		if value != "" && !validDigest(value) {
			return fmt.Errorf("receipt: invalid digest")
		}
	}
	if receipt.Skill.SourceDigest != nil {
		if !sourceDigestProfile.MatchString(receipt.Skill.SourceDigest.Profile) || !validDigest(receipt.Skill.SourceDigest.Value) {
			return fmt.Errorf("receipt: invalid source digest")
		}
	}
	if receipt.Skill.Version != "" && !semanticVersion.MatchString(receipt.Skill.Version) {
		return fmt.Errorf("receipt: invalid skill version")
	}
	if receipt.Runtime != nil {
		if receipt.Runtime.SupervisorVersion == "" && receipt.Runtime.RequiredVersion == "" && receipt.Runtime.Compatible == nil {
			return fmt.Errorf("receipt: empty runtime identity")
		}
		if receipt.Runtime.SupervisorVersion != "" && !semanticVersion.MatchString(receipt.Runtime.SupervisorVersion) || receipt.Runtime.RequiredVersion != "" && !semanticVersion.MatchString(receipt.Runtime.RequiredVersion) {
			return fmt.Errorf("receipt: invalid runtime version")
		}
		if receipt.Runtime.Compatible != nil && (receipt.Runtime.SupervisorVersion == "" || receipt.Runtime.RequiredVersion == "") {
			return fmt.Errorf("receipt: runtime compatibility lacks authoritative versions")
		}
	}
	if receipt.Timing.StartedAt == "" || receipt.Timing.LastObservedAt == "" {
		return fmt.Errorf("receipt: timing identity is incomplete")
	}
	for _, value := range []string{receipt.Timing.StartedAt, receipt.Timing.LastObservedAt, receipt.Timing.EndedAt} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
				return fmt.Errorf("receipt: invalid timestamp: %w", err)
			}
		}
	}
	if receipt.Timing.ElapsedMS != nil && *receipt.Timing.ElapsedMS < 0 {
		return fmt.Errorf("receipt: negative elapsed time")
	}
	validPhase := map[string]bool{"initializing": true, "initialization_failed": true, "awaiting_response": true, "advancing": true, "recoverable_error": true, "diverged": true, "terminal": true}
	if !validPhase[receipt.Outcome.Phase] {
		return fmt.Errorf("receipt: invalid lifecycle phase %q", receipt.Outcome.Phase)
	}
	if receipt.Outcome.Phase == "terminal" {
		validDisposition := map[string]bool{"completed": true, "blocked": true, "refused": true}
		validCause := map[string]bool{"completed": true, "blocked": true, "refused": true, "requirement_failed": true, "completion_unproven": true}
		if !validDisposition[receipt.Outcome.TerminalDisposition] || !validCause[receipt.Outcome.TerminalCause] {
			return fmt.Errorf("receipt: invalid terminal outcome")
		}
		if receipt.Timing.EndedAt == "" {
			return fmt.Errorf("receipt: terminal timing has no end")
		}
	} else if receipt.Outcome.TerminalDisposition != "" || receipt.Outcome.TerminalCause != "" {
		return fmt.Errorf("receipt: nonterminal receipt has a terminal outcome")
	}
	validFailureCode := map[string]bool{
		"manifest_invalid": true, "manifest_read_failed": true, "runtime_version_missing": true,
		"runtime_incompatible": true, "runner_missing": true,
		"source_digest_failed": true, "initialization_failed": true, "invalid_program_output": true,
		"execution_timeout": true, "subprocess_failed": true, "execution_failed": true,
		"command_execution_failed": true,
	}
	if receipt.Outcome.FailureCode != "" && !validFailureCode[receipt.Outcome.FailureCode] {
		return fmt.Errorf("receipt: invalid failure code")
	}
	if (receipt.Outcome.Phase == "initialization_failed" || receipt.Outcome.Phase == "recoverable_error") && receipt.Outcome.FailureCode == "" {
		return fmt.Errorf("receipt: failure phase has no code")
	}
	for _, operation := range receipt.Operations {
		if operation.Sequence < 1 || !validOperationKind(operation.Kind) || !validDigest(operation.OperationKeyDigest) || operation.RequestedAt == "" || operation.ElapsedMS != nil && *operation.ElapsedMS < 0 || operation.ResultDigest != "" && !validDigest(operation.ResultDigest) {
			return fmt.Errorf("receipt: invalid operation observation")
		}
		for _, value := range []string{operation.RequestedAt, operation.CompletedAt} {
			if value != "" {
				if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
					return fmt.Errorf("receipt: invalid operation timestamp")
				}
			}
		}
	}
	for _, summary := range receipt.OperationSummaries {
		if !validOperationKind(summary.Kind) || summary.Requested < 0 || summary.Completed < 0 || summary.Completed > summary.Requested || summary.TotalElapsedMS < 0 {
			return fmt.Errorf("receipt: invalid operation summary")
		}
	}
	for _, requirement := range receipt.Requirements {
		if requirement.Outcome != "passed" && requirement.Outcome != "failed" || !validDigest(requirement.ClaimDigest) || requirement.EvidenceDigest != "" && !validDigest(requirement.EvidenceDigest) {
			return fmt.Errorf("receipt: invalid requirement outcome")
		}
	}
	validRejection := map[string]bool{"wrong-run": true, "stale-response": true, "duplicate-response": true, "wrong-request": true, "schema-invalid": true, "digest-mismatch": true, "completion-unproven": true, "run-closed": true, "no-pending-operation": true}
	for _, rejection := range receipt.ResponseRejections {
		if !validRejection[rejection.Reason] || rejection.Count < 1 {
			return fmt.Errorf("receipt: invalid response rejection")
		}
	}
	for _, divergence := range receipt.Divergences {
		if divergence.Sequence < 1 || !validDigest(divergence.Expected) || !validDigest(divergence.Got) {
			return fmt.Errorf("receipt: invalid divergence outcome")
		}
	}
	if receipt.Experiment != nil {
		if err := receipt.Experiment.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validOperationKind(kind OperationKind) bool {
	return kind == OperationAskUser || kind == OperationAgentTask || kind == OperationRunCommand
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

// CanonicalBytes returns RFC 8785-compatible JSON for the receipt's closed,
// integer-only data model.
func CanonicalBytes(receipt RunReceipt) ([]byte, error) {
	return canonicalBytes(receipt)
}

func canonicalBytes(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := writeCanonical(&output, decoded); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writeCanonical(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		output.WriteString(strconv.FormatBool(typed))
	case string:
		encoded, err := marshalString(typed)
		if err != nil {
			return err
		}
		output.Write(encoded)
	case json.Number:
		text := typed.String()
		if strings.ContainsAny(text, ".eE") {
			return fmt.Errorf("receipt: non-integer JSON number %q is not supported", text)
		}
		if _, err := strconv.ParseInt(text, 10, 64); err != nil {
			return fmt.Errorf("receipt: invalid integer %q", text)
		}
		output.WriteString(text)
	case []any:
		output.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := writeCanonical(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			encoded, err := marshalString(key)
			if err != nil {
				return err
			}
			output.Write(encoded)
			output.WriteByte(':')
			if err := writeCanonical(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("receipt: unsupported canonical JSON type %T", value)
	}
	return nil
}

func marshalString(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("receipt: invalid UTF-8 string")
	}
	var output bytes.Buffer
	output.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(r)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&output, `\u%04x`, r)
			} else {
				output.WriteRune(r)
			}
		}
	}
	output.WriteByte('"')
	return output.Bytes(), nil
}

func utf16Less(left, right string) bool {
	l := utf16.Encode([]rune(left))
	r := utf16.Encode([]rune(right))
	for index := 0; index < len(l) && index < len(r); index++ {
		if l[index] != r[index] {
			return l[index] < r[index]
		}
	}
	return len(l) < len(r)
}

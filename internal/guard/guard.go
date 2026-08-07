// Package guard owns every protocol refusal. Each rejection reason has a
// named check and a test that proves the request is refused.
package guard

import (
	"fmt"

	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

type RejectReason string

const (
	RejectWrongRun       RejectReason = "wrong-run"
	RejectStale          RejectReason = "stale-response"
	RejectDuplicate      RejectReason = "duplicate-response"
	RejectWrongRequest   RejectReason = "wrong-request"
	RejectSchemaInvalid  RejectReason = "schema-invalid"
	RejectDigestMismatch RejectReason = "digest-mismatch"
	RejectUnproven       RejectReason = "completion-unproven"
	RejectRunClosed      RejectReason = "run-closed"
	RejectNoPendingOp    RejectReason = "no-pending-operation"
)

// Rejection is a typed refusal; it renders calmly and names its reason.
type Rejection struct {
	Reason RejectReason
	Detail string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("rejected (%s): %s", r.Reason, r.Detail)
}

func reject(reason RejectReason, format string, args ...any) *Rejection {
	return &Rejection{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}

// RunState is the guard-relevant projection of a run log.
type RunState struct {
	RunID            string
	BoundDigest      string
	Skill            protocol.SkillRef
	Pending          *protocol.RequestEnvelope // unanswered operation, if any
	Completed        map[int]string            // sequence -> result digest
	CompletedRequest map[int]string            // sequence -> request id
	Closed           bool                      // a terminal run.* event exists
	ReqFailed        bool                      // a requirement.failed event exists
	Diverged         bool
}

// Reconstruct folds a run log into its guard state. The log is the only
// source of truth; nothing else is consulted.
func Reconstruct(l *runlog.Log) (*RunState, error) {
	s := &RunState{Completed: map[int]string{}, CompletedRequest: map[int]string{}}
	for _, e := range l.Events() {
		switch e.Type {
		case runlog.RunStarted:
			var d struct {
				RunID string            `json:"run_id"`
				Skill protocol.SkillRef `json:"skill"`
			}
			if err := e.Decode(&d); err != nil {
				return nil, err
			}
			s.RunID = d.RunID
			s.Skill = d.Skill
			s.BoundDigest = d.Skill.Digest
		case runlog.OperationRequested:
			var env protocol.RequestEnvelope
			if err := e.Decode(&env); err != nil {
				return nil, err
			}
			s.Pending = &env
		case runlog.OperationCompleted:
			var d struct {
				Sequence     int    `json:"sequence"`
				RequestID    string `json:"request_id"`
				ResultDigest string `json:"result_digest"`
			}
			if err := e.Decode(&d); err != nil {
				return nil, err
			}
			s.Completed[d.Sequence] = d.ResultDigest
			s.CompletedRequest[d.Sequence] = d.RequestID
			if s.Pending != nil && s.Pending.Sequence == d.Sequence {
				s.Pending = nil
			}
		case runlog.DigestMigrated:
			var d struct {
				To string `json:"to"`
			}
			if err := e.Decode(&d); err != nil {
				return nil, err
			}
			s.BoundDigest = d.To
		case runlog.RequirementFailed:
			s.ReqFailed = true
		case runlog.ReplayDiverged:
			s.Diverged = true
		case runlog.RunCompleted, runlog.RunBlocked, runlog.RunRefused:
			s.Closed = true
		}
	}
	return s, nil
}

// CheckDigest refuses to act on a run whose skill source changed since the
// run was bound, unless the caller explicitly migrates. This is the
// migrate_digest mechanism from the lifecycle model — without it, a
// diverged-source run would block.
func CheckDigest(s *RunState, currentDigest string, migrate bool) error {
	if s.BoundDigest == currentDigest {
		return nil
	}
	if migrate {
		return nil
	}
	return reject(RejectDigestMismatch,
		"skill source changed since the run was bound (bound %s, current %s); re-run with --accept-new-digest to migrate explicitly",
		short(s.BoundDigest), short(currentDigest))
}

// CheckResponse decides whether a response envelope may be accepted for
// the run's pending operation. Refusals: wrong run, stale, duplicate,
// wrong request id, schema-invalid result.
func CheckResponse(s *RunState, resp protocol.ResponseEnvelope) error {
	if s.Closed {
		return reject(RejectRunClosed, "run %s already reached a terminal state", s.RunID)
	}
	if resp.RunID != s.RunID {
		return reject(RejectWrongRun, "response is for run %s, this run is %s", resp.RunID, s.RunID)
	}
	if s.Pending == nil {
		return reject(RejectNoPendingOp, "run %s has no pending operation", s.RunID)
	}
	if resp.Sequence != s.Pending.Sequence {
		if d, ok := s.Completed[resp.Sequence]; ok {
			if d == protocol.DigestBytes(resp.Result) {
				return reject(RejectDuplicate, "sequence %d already completed with identical content", resp.Sequence)
			}
			return reject(RejectDuplicate, "sequence %d already completed with DIFFERENT content; refusing to rewrite history", resp.Sequence)
		}
		return reject(RejectStale, "response targets sequence %d but the pending operation is sequence %d", resp.Sequence, s.Pending.Sequence)
	}
	if resp.RequestID != s.Pending.Request.ID {
		return reject(RejectWrongRequest, "response is for request %q but the pending request is %q", resp.RequestID, s.Pending.Request.ID)
	}
	if resp.Status == "completed" {
		if err := protocol.ValidateResult(s.Pending.Request.OutputSchema, resp.Result); err != nil {
			return reject(RejectSchemaInvalid, "%v", err)
		}
	}
	return nil
}

// CheckCompletion refuses run.completed when any requirement has failed:
// evidence-bound completion is a supervisory invariant, not advice.
func CheckCompletion(s *RunState, reqs []protocol.Requirement) error {
	if s.ReqFailed {
		return reject(RejectUnproven, "a requirement already failed in this run; completion is refused")
	}
	for _, r := range reqs {
		if !r.Passed {
			return reject(RejectUnproven, "requirement %q failed; completion is refused", r.Claim)
		}
	}
	return nil
}

func short(digest string) string {
	if len(digest) > 19 {
		return digest[:19] + "…"
	}
	return digest
}

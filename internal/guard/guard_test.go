package guard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/operatorstack/yield/internal/protocol"
)

// These tests verify that every declared rejection has a working refusal path.

func pendingState() *RunState {
	return &RunState{
		RunID:       "run_1",
		BoundDigest: "sha256:aaa",
		Pending: &protocol.RequestEnvelope{
			Protocol: protocol.Version,
			RunID:    "run_1",
			Sequence: 3,
			Request: protocol.Request{
				ID:   "confirm-scope",
				Kind: protocol.OpAskUser,
				OutputSchema: json.RawMessage(
					`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`),
			},
		},
		Completed: map[int]string{
			1: protocol.DigestBytes([]byte(`{"value":"a"}`)),
			2: protocol.DigestBytes([]byte(`{"value":"b"}`)),
		},
	}
}

func resp(seq int, id, result string) protocol.ResponseEnvelope {
	return protocol.ResponseEnvelope{
		RunID: "run_1", Sequence: seq, RequestID: id,
		Status: "completed", Result: json.RawMessage(result),
	}
}

func wantReject(t *testing.T, err error, reason RejectReason) {
	t.Helper()
	rej, ok := err.(*Rejection)
	if !ok {
		t.Fatalf("want *Rejection(%s), got %v", reason, err)
	}
	if rej.Reason != reason {
		t.Fatalf("want reason %s, got %s (%s)", reason, rej.Reason, rej.Detail)
	}
}

func TestRefusesStaleResponse(t *testing.T) {
	// disable-mechanism:accept_stale — a stale response never enters replay.
	err := CheckResponse(pendingState(), resp(0, "confirm-scope", `{"value":"x"}`))
	wantReject(t, err, RejectStale)
}

func TestRefusesDuplicateWithDifferentContent(t *testing.T) {
	err := CheckResponse(pendingState(), resp(2, "confirm-scope", `{"value":"REWRITTEN"}`))
	wantReject(t, err, RejectDuplicate)
	if !strings.Contains(err.Error(), "DIFFERENT content") {
		t.Fatalf("rejection must name the history rewrite: %v", err)
	}
}

func TestRefusesDuplicateIdentical(t *testing.T) {
	err := CheckResponse(pendingState(), resp(2, "confirm-scope", `{"value":"b"}`))
	wantReject(t, err, RejectDuplicate)
}

func TestRefusesWrongRun(t *testing.T) {
	r := resp(3, "confirm-scope", `{"value":"x"}`)
	r.RunID = "run_OTHER"
	wantReject(t, CheckResponse(pendingState(), r), RejectWrongRun)
}

func TestRefusesWrongRequestID(t *testing.T) {
	wantReject(t, CheckResponse(pendingState(), resp(3, "some-other-request", `{"value":"x"}`)), RejectWrongRequest)
}

func TestRefusesSchemaInvalidResult(t *testing.T) {
	// disable-mechanism:accept_response — schema validity gates acceptance.
	wantReject(t, CheckResponse(pendingState(), resp(3, "confirm-scope", `{"wrong":"shape"}`)), RejectSchemaInvalid)
}

func TestAcceptsValidResponse(t *testing.T) {
	if err := CheckResponse(pendingState(), resp(3, "confirm-scope", `{"value":"preserve"}`)); err != nil {
		t.Fatalf("valid response must be accepted: %v", err)
	}
}

func TestRefusesResponseOnClosedRun(t *testing.T) {
	s := pendingState()
	s.Closed = true
	wantReject(t, CheckResponse(s, resp(3, "confirm-scope", `{"value":"x"}`)), RejectRunClosed)
}

func TestRefusesCompletionAfterFailedRequirement(t *testing.T) {
	// disable-mechanism:complete_unproven — the forbidden transition
	// REQ_FAILED --complete--> COMPLETED is structurally refused.
	s := pendingState()
	s.ReqFailed = true
	wantReject(t, CheckCompletion(s, nil), RejectUnproven)

	err := CheckCompletion(pendingState(), []protocol.Requirement{
		{Claim: "tests pass", Passed: false},
	})
	wantReject(t, err, RejectUnproven)
}

func TestAllowsCompletionWithPassedRequirements(t *testing.T) {
	// disable-mechanism:complete — completion is permitted exactly when
	// every requirement passed.
	err := CheckCompletion(pendingState(), []protocol.Requirement{
		{Claim: "tests pass", Passed: true},
	})
	if err != nil {
		t.Fatalf("completion with passed requirements must be allowed: %v", err)
	}
}

func TestRefusesDigestMismatchWithoutMigration(t *testing.T) {
	s := pendingState()
	if err := CheckDigest(s, "sha256:aaa", false); err != nil {
		t.Fatalf("matching digest must pass: %v", err)
	}
	err := CheckDigest(s, "sha256:bbb", false)
	wantReject(t, err, RejectDigestMismatch)
	if !strings.Contains(err.Error(), "--accept-new-digest") {
		t.Fatalf("rejection must prescribe the migrate verb: %v", err)
	}
	// migrate_digest is the controllable recovery that keeps DIVERGED
	// non-blocking in the lifecycle model.
	if err := CheckDigest(s, "sha256:bbb", true); err != nil {
		t.Fatalf("explicit migration must be allowed: %v", err)
	}
}

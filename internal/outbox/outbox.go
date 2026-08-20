// Package outbox provides independent, resumable delivery of materialized
// receipts. It never reads or writes the authoritative run journal.
package outbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/operatorstack/yield/internal/receipt"
)

type Manager struct {
	Root string
	Now  func() time.Time
}

type AttemptEvent struct {
	Sequence         int    `json:"sequence"`
	At               string `json:"at"`
	Type             string `json:"type"`
	Code             string `json:"code,omitempty"`
	ExitCode         *int   `json:"exit_code,omitempty"`
	DiagnosticDigest string `json:"diagnostic_digest,omitempty"`
}

type EntryStatus struct {
	SinkID        string `json:"sink_id"`
	ReceiptDigest string `json:"receipt_digest"`
	State         string `json:"state"`
	Attempts      int    `json:"attempts"`
	LastCode      string `json:"last_code,omitempty"`
}

func New(yieldDir string) *Manager {
	return &Manager{Root: filepath.Join(yieldDir, "outbox"), Now: time.Now}
}

var sinkIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func validateSinkID(sinkID string) error {
	if len(sinkID) == 0 || len(sinkID) > 64 || !sinkIdentifier.MatchString(sinkID) {
		return fmt.Errorf("outbox: sink id must be a portable identifier of at most 64 characters")
	}
	return nil
}

// Enqueue atomically adds one canonical receipt to a sink's pending set.
func (m *Manager) Enqueue(sinkID string, r *receipt.RunReceipt, raw []byte) error {
	if err := validateSinkID(sinkID); err != nil {
		return err
	}
	if r == nil || r.ReceiptDigest == "" {
		return fmt.Errorf("outbox: incomplete receipt")
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if err := receipt.VerifyCanonical(r, raw); err != nil {
		return err
	}
	path, err := m.pendingPath(sinkID, r.ReceiptDigest)
	if err != nil {
		return err
	}
	return writeImmutable(path, raw)
}

// Deliver sends every selected pending receipt. Different digests are
// independent; each digest is serialized with its own file lock.
func (m *Manager) Deliver(ctx context.Context, sinkID string, argv []string, timeout time.Duration) ([]EntryStatus, error) {
	if err := validateSinkID(sinkID); err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("outbox: sink command is empty")
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	digests, err := m.pendingDigests(sinkID)
	if err != nil {
		return nil, err
	}
	statuses := make([]EntryStatus, 0, len(digests))
	var deliveryErrors []error
	for _, digest := range digests {
		status, deliverErr := m.deliverOne(ctx, sinkID, digest, argv, timeout)
		statuses = append(statuses, status)
		if deliverErr != nil {
			deliveryErrors = append(deliveryErrors, deliverErr)
		}
	}
	return statuses, errors.Join(deliveryErrors...)
}

func (m *Manager) deliverOne(parent context.Context, sinkID, digest string, argv []string, timeout time.Duration) (EntryStatus, error) {
	lockPath, err := m.lockPath(sinkID, digest)
	if err != nil {
		return EntryStatus{}, err
	}
	if err := secureMkdirAll(filepath.Dir(lockPath)); err != nil {
		return EntryStatus{}, err
	}
	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return EntryStatus{}, err
	}
	_ = os.Chmod(lockPath, 0o600)
	defer func() { _ = lock.Unlock(); _ = lock.Close() }()

	status, err := m.statusOne(sinkID, digest)
	if err != nil {
		return EntryStatus{}, err
	}
	if status.State == "accepted" {
		if err := m.ensureAcceptedMarker(sinkID, digest); err != nil {
			return status, err
		}
		return status, nil
	}
	if status.State == "failed" || status.State == "delivery_unknown" {
		return status, nil
	}
	raw, err := os.ReadFile(m.mustPendingPath(sinkID, digest))
	if err != nil {
		return status, err
	}
	if err := verifyReceipt(raw, digest); err != nil {
		if _, appendErr := m.appendAttempt(sinkID, digest, AttemptEvent{Type: "delivery_failed", Code: "invalid_pending_receipt"}); appendErr != nil {
			return status, errors.Join(err, appendErr)
		}
		status, _ = m.statusOne(sinkID, digest)
		return status, err
	}
	if _, err := m.appendAttempt(sinkID, digest, AttemptEvent{Type: "delivery_started"}); err != nil {
		return status, err
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Env = append(os.Environ(), "YIELD_RECEIPT_DIGEST="+digest, "YIELD_SINK_ID="+sinkID)
	stdout := newDigestWriter()
	stderr := newDigestWriter()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()
	if runErr == nil {
		if _, err := m.appendAttempt(sinkID, digest, AttemptEvent{Type: "delivery_accepted"}); err != nil {
			status, statusErr := m.statusOne(sinkID, digest)
			return status, errors.Join(err, statusErr)
		}
		if err := m.ensureAcceptedMarker(sinkID, digest); err != nil {
			status, statusErr := m.statusOne(sinkID, digest)
			return status, errors.Join(err, statusErr)
		}
		return m.statusOne(sinkID, digest)
	}
	event := AttemptEvent{Type: "delivery_failed", Code: "process_failed"}
	if ctx.Err() == context.DeadlineExceeded {
		event.Code = "timeout"
	} else {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code := exitErr.ExitCode()
			event.ExitCode = &code
			event.Code = "nonzero_exit"
		}
	}
	if stdout.n > 0 || stderr.n > 0 {
		event.DiagnosticDigest = digestBytes(append(stdout.h.Sum(nil), stderr.h.Sum(nil)...))
	}
	if _, err := m.appendAttempt(sinkID, digest, event); err != nil {
		return status, errors.Join(runErr, err)
	}
	status, statusErr := m.statusOne(sinkID, digest)
	return status, errors.Join(fmt.Errorf("deliver %s: %s", digest, event.Code), statusErr)
}

// Retry marks a failed or delivery-unknown entry as pending for explicit retry.
func (m *Manager) Retry(sinkID, digest string) error {
	if err := validateSinkID(sinkID); err != nil {
		return err
	}
	lockPath, err := m.lockPath(sinkID, digest)
	if err != nil {
		return err
	}
	if err := secureMkdirAll(filepath.Dir(lockPath)); err != nil {
		return err
	}
	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return err
	}
	_ = os.Chmod(lockPath, 0o600)
	defer func() { _ = lock.Unlock(); _ = lock.Close() }()
	status, err := m.statusOne(sinkID, digest)
	if err != nil {
		return err
	}
	if status.State == "accepted" {
		return fmt.Errorf("outbox: receipt %s is already accepted", digest)
	}
	_, err = m.appendAttempt(sinkID, digest, AttemptEvent{Type: "retry_requested"})
	return err
}

func (m *Manager) Status(sinkFilter string) ([]EntryStatus, error) {
	if sinkFilter != "" {
		if err := validateSinkID(sinkFilter); err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(m.Root)
	if errors.Is(err, os.ErrNotExist) {
		return []EntryStatus{}, nil
	}
	if err != nil {
		return nil, err
	}
	var statuses []EntryStatus
	for _, entry := range entries {
		if !entry.IsDir() || sinkFilter != "" && entry.Name() != sinkFilter {
			continue
		}
		digests, err := m.pendingDigests(entry.Name())
		if err != nil {
			return nil, err
		}
		for _, digest := range digests {
			status, err := m.statusOne(entry.Name(), digest)
			if err != nil {
				return nil, err
			}
			statuses = append(statuses, status)
		}
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].SinkID != statuses[j].SinkID {
			return statuses[i].SinkID < statuses[j].SinkID
		}
		return statuses[i].ReceiptDigest < statuses[j].ReceiptDigest
	})
	return statuses, nil
}

func (m *Manager) statusOne(sinkID, digest string) (EntryStatus, error) {
	status := EntryStatus{SinkID: sinkID, ReceiptDigest: digest, State: "pending"}
	attempts, err := m.readAttempts(sinkID, digest)
	if err != nil {
		return status, err
	}
	for _, attempt := range attempts {
		if attempt.Type == "delivery_started" {
			status.Attempts++
		}
	}
	if len(attempts) == 0 {
		return status, nil
	}
	last := attempts[len(attempts)-1]
	status.LastCode = last.Code
	switch last.Type {
	case "delivery_accepted":
		status.State = "accepted"
	case "delivery_started":
		status.State = "delivery_unknown"
	case "delivery_failed":
		status.State = "failed"
	case "retry_requested":
		status.State = "pending"
	default:
		return status, fmt.Errorf("outbox: unknown attempt type %q", last.Type)
	}
	return status, nil
}

func (m *Manager) appendAttempt(sinkID, digest string, event AttemptEvent) (AttemptEvent, error) {
	path, err := m.attemptPath(sinkID, digest)
	if err != nil {
		return AttemptEvent{}, err
	}
	if err := secureMkdirAll(filepath.Dir(path)); err != nil {
		return AttemptEvent{}, err
	}
	attempts, err := m.readAttempts(sinkID, digest)
	if err != nil {
		return AttemptEvent{}, err
	}
	event.Sequence = len(attempts) + 1
	event.At = m.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(event)
	if err != nil {
		return AttemptEvent{}, err
	}
	if err := repairTrailingPartial(path); err != nil {
		return AttemptEvent{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return AttemptEvent{}, err
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		f.Close()
		return AttemptEvent{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return AttemptEvent{}, err
	}
	return event, f.Close()
}

func (m *Manager) readAttempts(sinkID, digest string) ([]AttemptEvent, error) {
	path, err := m.attemptPath(sinkID, digest)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []AttemptEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	var attempts []AttemptEvent
	lines := bytes.Split(raw, []byte{'\n'})
	for index, line := range lines {
		if len(line) == 0 {
			continue
		}
		if index == len(lines)-1 && raw[len(raw)-1] != '\n' {
			break
		}
		var event AttemptEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, fmt.Errorf("outbox: corrupt attempt journal: %w", err)
		}
		if event.Sequence != len(attempts)+1 {
			return nil, fmt.Errorf("outbox: attempt sequence is not monotone")
		}
		if err := validateAttempt(event); err != nil {
			return nil, err
		}
		attempts = append(attempts, event)
	}
	return attempts, nil
}

func validateAttempt(event AttemptEvent) error {
	if event.Sequence < 1 {
		return fmt.Errorf("outbox: invalid attempt sequence")
	}
	if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
		return fmt.Errorf("outbox: invalid attempt timestamp")
	}
	switch event.Type {
	case "delivery_started", "delivery_accepted", "retry_requested":
		if event.Code != "" || event.ExitCode != nil || event.DiagnosticDigest != "" {
			return fmt.Errorf("outbox: unexpected attempt diagnostics")
		}
	case "delivery_failed":
		validCode := event.Code == "process_failed" || event.Code == "timeout" || event.Code == "nonzero_exit" || event.Code == "invalid_pending_receipt"
		if !validCode {
			return fmt.Errorf("outbox: invalid delivery failure code")
		}
		if event.DiagnosticDigest != "" {
			if _, err := digestName(event.DiagnosticDigest); err != nil {
				return fmt.Errorf("outbox: invalid diagnostic digest")
			}
		}
	default:
		return fmt.Errorf("outbox: unknown attempt type %q", event.Type)
	}
	return nil
}

func repairTrailingPartial(path string) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(raw) == 0 || raw[len(raw)-1] == '\n' {
		return nil
	}
	lastNewline := bytes.LastIndexByte(raw, '\n')
	return os.Truncate(path, int64(lastNewline+1))
}

func (m *Manager) pendingDigests(sinkID string) ([]string, error) {
	dir := filepath.Join(m.Root, sinkID, "pending")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var digests []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		hexDigest := strings.TrimSuffix(entry.Name(), ".json")
		if len(hexDigest) == 64 {
			digests = append(digests, "sha256:"+hexDigest)
		}
	}
	sort.Strings(digests)
	return digests, nil
}

func (m *Manager) ensureAcceptedMarker(sinkID, digest string) error {
	path, err := m.acceptedPath(sinkID, digest)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]string{"receipt_digest": digest})
	return writeImmutable(path, raw)
}

func (m *Manager) pendingPath(sinkID, digest string) (string, error) {
	name, err := digestName(digest)
	return filepath.Join(m.Root, sinkID, "pending", name+".json"), err
}
func (m *Manager) mustPendingPath(sinkID, digest string) string {
	path, _ := m.pendingPath(sinkID, digest)
	return path
}
func (m *Manager) attemptPath(sinkID, digest string) (string, error) {
	name, err := digestName(digest)
	return filepath.Join(m.Root, sinkID, "attempts", name+".jsonl"), err
}
func (m *Manager) acceptedPath(sinkID, digest string) (string, error) {
	name, err := digestName(digest)
	return filepath.Join(m.Root, sinkID, "accepted", name+".json"), err
}
func (m *Manager) lockPath(sinkID, digest string) (string, error) {
	name, err := digestName(digest)
	return filepath.Join(m.Root, sinkID, "locks", name+".lock"), err
}

func digestName(digest string) (string, error) {
	value := strings.TrimPrefix(digest, "sha256:")
	if len(value) != 64 {
		return "", fmt.Errorf("outbox: invalid receipt digest")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("outbox: invalid receipt digest")
	}
	return value, nil
}

func writeImmutable(path string, content []byte) error {
	if err := secureMkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(existing, content) {
			return fmt.Errorf("outbox: content collision at %s", path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".outbox-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpPath, path); err != nil {
		if existing, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(existing, content) {
			return nil
		}
		return err
	}
	return syncDir(filepath.Dir(path))
}

func secureMkdirAll(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func digestBytes(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func verifyReceipt(raw []byte, digest string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var r receipt.RunReceipt
	if err := decoder.Decode(&r); err != nil {
		return fmt.Errorf("outbox: pending receipt does not decode: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("outbox: pending receipt has trailing content")
	}
	if r.ReceiptDigest != digest {
		return fmt.Errorf("outbox: pending receipt digest does not match its name")
	}
	return receipt.VerifyCanonical(&r, raw)
}

type digestWriter struct {
	h hash.Hash
	n int64
}

func newDigestWriter() *digestWriter { return &digestWriter{h: sha256.New()} }

func (writer *digestWriter) Write(value []byte) (int, error) {
	written, err := writer.h.Write(value)
	writer.n += int64(written)
	return written, err
}

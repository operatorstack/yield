package outbox

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
	receipt "github.com/operatorstack/yield/observation"
)

func testReceipt(t *testing.T) (*receipt.RunReceipt, []byte) {
	t.Helper()
	event := runlog.Event{
		Seq:  1,
		Type: runlog.RunStarted,
		At:   time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC),
	}
	event.Data, _ = json.Marshal(map[string]any{
		"run_id": "run_1",
		"skill":  protocol.SkillRef{Name: "test", Digest: digestBytes([]byte("skill"))},
	})
	journal, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	journal = append(journal, '\n')
	r, err := receipt.Project(journal)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := receipt.CanonicalBytes(r)
	if err != nil {
		t.Fatal(err)
	}
	return &r, raw
}

func TestEnqueueIsByteIdempotent(t *testing.T) {
	m := New(t.TempDir())
	r, raw := testReceipt(t)
	if err := m.Enqueue("test-sink", r, raw); err != nil {
		t.Fatal(err)
	}
	if err := m.Enqueue("test-sink", r, raw); err != nil {
		t.Fatal(err)
	}
	statuses, err := m.Status("test-sink")
	if err != nil || len(statuses) != 1 || statuses[0].State != "pending" {
		t.Fatalf("unexpected status: %+v, %v", statuses, err)
	}
}

func TestDeliveryFailureIsRetryableAndDoesNotPersistOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	m := New(t.TempDir())
	r, raw := testReceipt(t)
	if err := m.Enqueue("test-sink", r, raw); err != nil {
		t.Fatal(err)
	}
	_, err := m.Deliver(context.Background(), "test-sink", []string{"sh", "-c", "echo SECRET_SINK_OUTPUT >&2; exit 7"}, time.Second)
	if err == nil {
		t.Fatal("failed sink was accepted")
	}
	statuses, err := m.Status("test-sink")
	if err != nil || statuses[0].State != "failed" {
		t.Fatalf("unexpected status: %+v, %v", statuses, err)
	}
	attemptPath, _ := m.attemptPath("test-sink", r.ReceiptDigest)
	attempts, err := os.ReadFile(attemptPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(attempts), "SECRET_SINK_OUTPUT") || strings.Contains(string(attempts), "echo") {
		t.Fatalf("attempt journal leaked output or argv: %s", attempts)
	}
	if err := m.Retry("test-sink", r.ReceiptDigest); err != nil {
		t.Fatal(err)
	}
	statuses, _ = m.Status("test-sink")
	if statuses[0].State != "pending" {
		t.Fatalf("retry state = %s", statuses[0].State)
	}
}

func TestDeliveryUnknownAndAcceptedRecovery(t *testing.T) {
	m := New(t.TempDir())
	m.Now = func() time.Time { return time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC) }
	r, raw := testReceipt(t)
	if err := m.Enqueue("test-sink", r, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := m.appendAttempt("test-sink", r.ReceiptDigest, AttemptEvent{Type: "delivery_started"}); err != nil {
		t.Fatal(err)
	}
	statuses, _ := m.Status("test-sink")
	if statuses[0].State != "delivery_unknown" {
		t.Fatalf("status = %s", statuses[0].State)
	}
	if err := m.Retry("test-sink", r.ReceiptDigest); err != nil {
		t.Fatal(err)
	}
	if _, err := m.appendAttempt("test-sink", r.ReceiptDigest, AttemptEvent{Type: "delivery_accepted"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Deliver(context.Background(), "test-sink", []string{"unused"}, time.Second); err != nil {
		t.Fatal(err)
	}
	acceptedPath, _ := m.acceptedPath("test-sink", r.ReceiptDigest)
	marker, err := os.ReadFile(acceptedPath)
	if err != nil {
		t.Fatal(err)
	}
	var accepted map[string]string
	if err := json.Unmarshal(marker, &accepted); err != nil || accepted["receipt_digest"] != r.ReceiptDigest {
		t.Fatalf("invalid accepted marker: %s", marker)
	}
	if _, err := os.Stat(filepath.Join(m.Root, "test-sink", "pending")); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentExportersDeliverOneAttemptPerDigest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	m := New(t.TempDir())
	r, raw := testReceipt(t)
	if err := m.Enqueue("test-sink", r, raw); err != nil {
		t.Fatal(err)
	}
	counter := filepath.Join(t.TempDir(), "counter")
	argv := []string{"sh", "-c", `cat >/dev/null; echo delivered >> "$1"`, "sink", counter}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.Deliver(context.Background(), "test-sink", argv, time.Second)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(content), "delivered") != 1 {
		t.Fatalf("sink deliveries = %q", content)
	}
	statuses, _ := m.Status("test-sink")
	if statuses[0].State != "accepted" || statuses[0].Attempts != 1 {
		t.Fatalf("unexpected concurrent status: %+v", statuses[0])
	}
}

func TestPartialAttemptWriteIsRepairable(t *testing.T) {
	m := New(t.TempDir())
	r, raw := testReceipt(t)
	if err := m.Enqueue("test-sink", r, raw); err != nil {
		t.Fatal(err)
	}
	path, _ := m.attemptPath("test-sink", r.ReceiptDigest)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"sequence":1,"at":"2026`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.Retry("test-sink", r.ReceiptDigest); err != nil {
		t.Fatal(err)
	}
	attempts, err := m.readAttempts("test-sink", r.ReceiptDigest)
	if err != nil || len(attempts) != 1 || attempts[0].Type != "retry_requested" {
		t.Fatalf("partial attempt was not repaired: %+v, %v", attempts, err)
	}
}

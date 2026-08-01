package runlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendOnlyMonotoneSequence(t *testing.T) {
	dir := t.TempDir()
	l, err := Create(dir, "run_x")
	if err != nil {
		t.Fatal(err)
	}
	for i, typ := range []EventType{RunStarted, OperationRequested, OperationCompleted} {
		e, err := l.Append(typ, map[string]int{"i": i})
		if err != nil {
			t.Fatal(err)
		}
		if e.Seq != i+1 {
			t.Fatalf("want seq %d, got %d", i+1, e.Seq)
		}
	}
	reloaded, err := Open(dir, "run_x")
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Events()) != 3 {
		t.Fatalf("want 3 events after reload, got %d", len(reloaded.Events()))
	}
}

func TestCreateRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	if _, err := Create(dir, "run_x"); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(dir, "run_x"); err == nil {
		t.Fatal("creating over an existing run log must be refused")
	}
}

func TestOpenRefusesBrokenSequence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run_bad.jsonl")
	lines := `{"seq":1,"type":"run.started","at":"2026-08-01T00:00:00Z"}
{"seq":3,"type":"run.completed","at":"2026-08-01T00:00:01Z"}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(dir, "run_bad")
	if err == nil || !strings.Contains(err.Error(), "sequence broken") {
		t.Fatalf("want sequence-broken refusal, got %v", err)
	}
}

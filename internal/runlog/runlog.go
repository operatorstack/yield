// Package runlog is the append-only event log that owns every run's state.
// One JSONL file per run; sequence numbers are monotone; nothing is ever
// rewritten. Replaying the log is the only way run state is reconstructed.
package runlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type EventType string

const (
	RunStarted         EventType = "run.started"
	OperationRequested EventType = "operation.requested"
	OperationCompleted EventType = "operation.completed"
	ResponseRejected   EventType = "response.rejected"
	RequirementPassed  EventType = "requirement.passed"
	RequirementFailed  EventType = "requirement.failed"
	DigestMigrated     EventType = "digest.migrated"
	ReplayDiverged     EventType = "replay.diverged"
	RunCompleted       EventType = "run.completed"
	RunBlocked         EventType = "run.blocked"
	RunRefused         EventType = "run.refused"
)

// Event is one appended fact. Seq is monotone from 1 within a run.
type Event struct {
	Seq  int             `json:"seq"`
	Type EventType       `json:"type"`
	At   time.Time       `json:"at"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Log is an open run log. Appends go straight to disk with a sync.
type Log struct {
	Path   string
	events []Event
}

// RunsDir returns the runs directory under a root (usually the skill dir
// or the cwd), creating it if needed.
func RunsDir(root string) (string, error) {
	dir := filepath.Join(root, ".yield", "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Create starts a new log file for a run. It refuses to overwrite.
func Create(runsDir, runID string) (*Log, error) {
	path := filepath.Join(runsDir, runID+".jsonl")
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("run log already exists: %s", path)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		return nil, err
	}
	return &Log{Path: path}, nil
}

// Open loads an existing run log and verifies sequence monotonicity.
func Open(runsDir, runID string) (*Log, error) {
	path := filepath.Join(runsDir, runID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("no such run: %s: %w", runID, err)
	}
	defer f.Close()
	l := &Log{Path: path}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		if len(sc.Bytes()) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("corrupt run log at line %d: %w", line, err)
		}
		if e.Seq != len(l.events)+1 {
			return nil, fmt.Errorf("run log sequence broken at line %d: got seq %d, want %d", line, e.Seq, len(l.events)+1)
		}
		l.events = append(l.events, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return l, nil
}

// Append writes one event with the next sequence number, fsynced.
func (l *Log) Append(t EventType, data any) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, err
	}
	e := Event{Seq: len(l.events) + 1, Type: t, At: time.Now().UTC(), Data: raw}
	line, err := json.Marshal(e)
	if err != nil {
		return Event{}, err
	}
	f, err := os.OpenFile(l.Path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Event{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Event{}, err
	}
	if err := f.Sync(); err != nil {
		return Event{}, err
	}
	l.events = append(l.events, e)
	return e, nil
}

// Events returns the loaded events in order.
func (l *Log) Events() []Event { return l.events }

// Decode unmarshals an event's data into v.
func (e Event) Decode(v any) error { return json.Unmarshal(e.Data, v) }

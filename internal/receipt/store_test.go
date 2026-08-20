package receipt

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

func TestStoreMaterializationConverges(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "store", Digest: protocol.DigestBytes([]byte("store"))}
	events := []runlog.Event{event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_store", "skill": skill})}
	snapshot := Snapshot{Events: events, Bytes: journalBytes(t, events)}
	store := NewStore(t.TempDir())
	first, firstBytes, err := store.Materialize(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, secondBytes, err := store.Materialize(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if first.ReceiptDigest != second.ReceiptDigest || !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("rematerialization did not converge")
	}
	loaded, loadedBytes, err := store.LoadRun("run_store")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ReceiptDigest != first.ReceiptDigest || !bytes.Equal(loadedBytes, firstBytes) {
		t.Fatal("loaded object differs from materialized object")
	}
	if runtime.GOOS != "windows" {
		digest := first.ReceiptDigest[len("sha256:"):]
		for _, path := range []string{
			filepath.Join(store.Root, "objects", "sha256", digest[:2], digest+".json"),
			filepath.Join(store.Root, "runs", "run_store.ref"),
		} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("%s permissions = %o", path, info.Mode().Perm())
			}
		}
	}
}

func TestStoreRepairsObjectWithoutRunReference(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "store", Digest: protocol.DigestBytes([]byte("store"))}
	events := []runlog.Event{event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_store", "skill": skill})}
	snapshot := Snapshot{Events: events, Bytes: journalBytes(t, events)}
	store := NewStore(t.TempDir())
	r, _, err := store.Materialize(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	ref := filepath.Join(store.Root, "runs", "run_store.ref")
	if err := os.Remove(ref); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Materialize(snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := store.LoadRun("run_store")
	if err != nil || loaded.ReceiptDigest != r.ReceiptDigest {
		t.Fatalf("orphaned object was not repaired: %+v, %v", loaded, err)
	}
}

func TestStoreRejectsContentMismatchAtDigestPath(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "store", Digest: protocol.DigestBytes([]byte("store"))}
	events := []runlog.Event{event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_store", "skill": skill})}
	store := NewStore(t.TempDir())
	r, raw, err := store.Materialize(Snapshot{Events: events, Bytes: journalBytes(t, events)})
	if err != nil {
		t.Fatal(err)
	}
	digest := r.ReceiptDigest[len("sha256:"):]
	path := filepath.Join(store.Root, "objects", "sha256", digest[:2], digest+".json")
	if err := os.WriteFile(path, []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(r, raw); err == nil {
		t.Fatal("content mismatch was accepted")
	}
}

func TestStoreRejectsUnsealedReceiptDigest(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	skill := protocol.SkillRef{Name: "store", Digest: protocol.DigestBytes([]byte("store"))}
	events := []runlog.Event{event(t, 1, runlog.RunStarted, t0, map[string]any{"run_id": "run_store", "skill": skill})}
	r, err := Project(Snapshot{Events: events, Bytes: journalBytes(t, events)})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := CanonicalBytes(r)
	r.ReceiptDigest = protocol.DigestBytes([]byte("wrong"))
	if err := NewStore(t.TempDir()).Put(r, raw); err == nil {
		t.Fatal("store accepted a receipt whose digest did not name its bytes")
	}
}

func TestDecodeExperimentIsClosed(t *testing.T) {
	if _, err := DecodeExperiment([]byte(`{"experiment_id":"exp-1","variant_id":"candidate-a","role":"candidate"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeExperiment([]byte(`{"experiment_id":"exp-1","variant_id":"candidate-a","role":"candidate","person":"alice"}`)); err == nil {
		t.Fatal("unknown personal field was accepted")
	}
}

package observation_test

import (
	"bytes"
	"testing"

	"github.com/operatorstack/yield/observation"
)

func TestPublicProjectionVerificationAndStore(t *testing.T) {
	journal := []byte(`{"seq":1,"type":"run.started","at":"2026-08-20T10:00:00Z","data":{"run_id":"run_public","skill":{"name":"public","digest":"sha256:0000000000000000000000000000000000000000000000000000000000000000"},"input_digest":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}}` + "\n")
	receipt, err := observation.Project(journal)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := observation.CanonicalBytes(receipt)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := observation.Parse(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if err := observation.VerifyCanonical(parsed, canonical); err != nil {
		t.Fatal(err)
	}

	store := observation.NewStore(t.TempDir())
	if err := store.Put(parsed, canonical); err != nil {
		t.Fatal(err)
	}
	loaded, loadedBytes, err := store.LoadRun("run_public")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ReceiptDigest != receipt.ReceiptDigest || !bytes.Equal(loadedBytes, canonical) {
		t.Fatal("public store round trip changed the receipt")
	}
}

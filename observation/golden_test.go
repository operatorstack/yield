package observation

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedCanonicalReceiptFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "ir", "yield.observation.v1", "testdata", "run-receipt.canonical.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	canonical := bytes.TrimSuffix(raw, []byte("\n"))
	if len(canonical) == len(raw) || bytes.Contains(canonical, []byte("\n")) {
		t.Fatal("shared receipt fixture must be one canonical JSON line followed by one file newline")
	}
	receipt, err := Parse(canonical)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := CanonicalBytes(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, canonical) {
		t.Fatal("shared receipt fixture differs from Go canonical bytes")
	}
}

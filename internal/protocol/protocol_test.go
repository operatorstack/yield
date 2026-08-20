package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRequestDigestBindsAllFields(t *testing.T) {
	base := Request{ID: "a", Kind: OpAskUser, Payload: json.RawMessage(`{"q":1}`)}
	same := Request{ID: "a", Kind: OpAskUser, Payload: json.RawMessage(`{"q":1}`)}
	if RequestDigest(base) != RequestDigest(same) {
		t.Fatal("identical requests must digest identically")
	}
	for _, changed := range []Request{
		{ID: "b", Kind: OpAskUser, Payload: json.RawMessage(`{"q":1}`)},
		{ID: "a", Kind: OpAgentTask, Payload: json.RawMessage(`{"q":1}`)},
		{ID: "a", Kind: OpAskUser, Payload: json.RawMessage(`{"q":2}`)},
		{ID: "a", Kind: OpAskUser, Payload: json.RawMessage(`{"q":1}`), OutputSchema: json.RawMessage(`{}`)},
	} {
		if RequestDigest(base) == RequestDigest(changed) {
			t.Fatalf("digest must bind every field; collision on %+v", changed)
		}
	}
}

func TestValidateResult(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["value"],"properties":{"value":{"type":"string"}}}`)
	if err := ValidateResult(schema, json.RawMessage(`{"value":"ok"}`)); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}
	if err := ValidateResult(schema, json.RawMessage(`{"other":1}`)); err == nil {
		t.Fatal("schema-invalid result must be rejected")
	}
	if err := ValidateResult(nil, json.RawMessage(`{"anything":true}`)); err != nil {
		t.Fatalf("nil schema accepts any JSON: %v", err)
	}
}

func TestDigestSkillDirIsContentBound(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main")
	write("SKILL.md", "prose")
	d1, err := DigestSkillDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := DigestSkillDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatal("digest must be deterministic")
	}
	write("main.go", "package main // changed")
	d3, err := DigestSkillDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d3 == d1 {
		t.Fatal("source change must change the digest")
	}
	// Run state must not affect the digest.
	if err := os.MkdirAll(filepath.Join(dir, ".yield", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(".yield", "runs", "x.jsonl"), "")
	d4, err := DigestSkillDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d4 != d3 {
		t.Fatal("run logs must not affect the skill digest")
	}
}

func TestSkillSourceProfileV1CoversRustAndLockfiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.rs", "fn main() {}")
	write("Cargo.lock", "version = 3")
	write("skill.json", `{"version":1,"yield_version":"1.0.0","language":"rust","run":["cargo","run"]}`)
	profiled, err := DigestSkillDirProfile(dir, SkillSourceProfileV1)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := DigestSkillDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if profiled == legacy {
		t.Fatal("profiled digest must include sources omitted by the legacy profile")
	}
	write("Cargo.lock", "version = 4")
	changed, err := DigestSkillDirProfile(dir, SkillSourceProfileV1)
	if err != nil {
		t.Fatal(err)
	}
	if changed == profiled {
		t.Fatal("lockfile change must change the profiled digest")
	}
	if err := os.MkdirAll(filepath.Join(dir, "target", "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target", "generated", "build.rs"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	stable, err := DigestSkillDirProfile(dir, SkillSourceProfileV1)
	if err != nil {
		t.Fatal(err)
	}
	if stable != changed {
		t.Fatal("generated build tree changed the source digest")
	}
}

func TestRequestDigestIsCompactionInvariant(t *testing.T) {
	pretty := Request{ID: "a", Kind: OpAgentTask,
		Payload:      json.RawMessage("{\n  \"q\": 1\n}"),
		OutputSchema: json.RawMessage("{\n  \"type\": \"object\"\n}")}
	compact := Request{ID: "a", Kind: OpAgentTask,
		Payload:      json.RawMessage(`{"q":1}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`)}
	if RequestDigest(pretty) != RequestDigest(compact) {
		t.Fatal("digest must be invariant under JSON compaction (log round-trips compact RawMessage)")
	}
}

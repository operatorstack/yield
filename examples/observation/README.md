# RunReceipt readers

These standalone examples read and verify one already-materialized
`yield.observation.v1` receipt in Go, TypeScript, Python, and Rust. They are
post-run consumers, not skill workflows: the Go supervisor projects the
receipt after a foreground stopping point, and the language readers never read
the journal or participate in replay.

Pass the exact bytes of a content-addressed receipt object. The input must be
canonical JSON with no transport newline or other trailing data.

```bash
RECEIPT=/path/to/.yield/receipts/objects/sha256/ab/abcdef.json

go run ./examples/observation/go "$RECEIPT"
node examples/observation/typescript/main.ts "$RECEIPT"
PYTHONDONTWRITEBYTECODE=1 python3 examples/observation/python/main.py "$RECEIPT"
cargo run --quiet --manifest-path examples/observation/rust/Cargo.toml -- "$RECEIPT"
```

Every example verifies the closed schema, canonical encoding, and embedded
SHA-256 digest before writing output. They emit the same privacy-safe subset:
the schema and receipt digest, run and skill identity, lifecycle phase,
terminal disposition, and operation summaries. Invalid input produces no
partial summary.

The repository examples import the adjacent SDK sources so they always test
the current checkout. In an installed application, use the public packages:

- Go: `github.com/operatorstack/yield/observation`
- TypeScript: `@operatorstack/yield`
- Python: `yieldskill`
- Rust: `yieldskill`

Run all four readers, including strict rejection cases, from the repository
root:

```bash
bash ./examples/observation/test-all.sh
```

The test harness removes the single JSONL framing newline from the shared
golden fixture before invoking the readers. The readers themselves never
normalize receipt bytes.

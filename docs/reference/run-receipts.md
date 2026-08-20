# Portable run receipts

Yield keeps the append-only run journal as the source of truth. At each
foreground stopping point, the supervisor projects the latest journal prefix
into a portable `RunReceipt` and stores it locally before returning success.
Receipts do not participate in replay and cannot change a workflow result.

The public schema is
`ir/yield.observation.v1/run-receipt.schema.json`. It records structured facts
such as run and skill identity, source and runtime versions when known,
operation kinds and timing, typed rejection and requirement outcomes,
divergence digests, terminal disposition, and optional experiment identifiers.
All four SDKs use the same supervisor projection.

## Go API

Go hosts can project, verify, store, and aggregate receipts without invoking
the CLI. The public package is
[`github.com/operatorstack/yield/observation`](https://pkg.go.dev/github.com/operatorstack/yield/observation).

```go
receipt, err := observation.Project(journalPrefix)
if err != nil {
	return err
}
canonical, err := observation.CanonicalBytes(receipt)
if err != nil {
	return err
}

store := observation.NewStore(yieldDir)
if err := store.Put(receipt, canonical); err != nil {
	return err
}
```

`Project` accepts one exact, complete `.yield/runs/<run-id>.jsonl` prefix and
performs no I/O. It rejects partial lines, malformed ordering, unknown event
types, and unknown journal-envelope fields. `Parse` accepts only canonical
receipt bytes whose digest verifies. These checks make the journal prefix the
explicit input and keep receipt files derived, rather than authoritative.

## Privacy boundary

Receipts do not contain prompts, instructions, model responses, user answers,
command arguments, stdout, stderr, source contents, credentials, tokens,
environment values, or free-form failure details. Input, result, evidence,
claim, and operation keys are represented by SHA-256 digests.

Receipts can group runs by recorded source and runtime versions. Installation
paths, package-manager state, adapter state, and environment diagnostics remain
part of `yskill doctor` and are not copied into receipts.

A digest supports integrity and correlation. It is not anonymization. A party
can guess a low-entropy value and compare its digest, and the same digest can
link observations across receipts. Do not use personal identifiers for
experiment or cohort fields.

Yield records only supervisor-observed facts. A receipt does not prove what a
coding agent did outside the Yield protocol.

## Local storage

```text
.yield/
  runs/<run-id>.jsonl
  receipts/
    objects/sha256/<prefix>/<digest>.json
    runs/<run-id>.ref
  outbox/<sink-id>/
    pending/<digest>.json
    attempts/<digest>.jsonl
    accepted/<digest>.json
    locks/<digest>.lock
```

Receipt objects are immutable and content-addressed. A per-run reference points
to the latest projected prefix. Materialization uses a synced temporary file,
an atomic installation, and a synced reference update. A crash between object
creation and reference update is repaired by materializing the same journal
again. Garbage collection is not part of this release.

Rust source profiles include `Cargo.toml` and `Cargo.lock` when those files are
inside the skill directory. Workspace-managed Rust skills without a local
lockfile remain valid; parent workspace files are outside the skill's recorded
source-digest boundary.

Run age is query-relative. `yskill report` can say that an open run is older
than a supplied threshold, but it does not declare the run abandoned.

## Deferred export

Export is always explicit and separate from foreground execution:

```bash
yskill outbox enqueue <run-id> --sink local-analysis --skill ./my-skill
yskill outbox deliver --sink local-analysis --skill ./my-skill -- ./receipt-sink
yskill outbox status --sink local-analysis --skill ./my-skill
yskill outbox retry --failed --sink local-analysis --skill ./my-skill
```

Yield executes the sink command directly, without a shell. It provides one
complete receipt on standard input and sets `YIELD_RECEIPT_DIGEST` and
`YIELD_SINK_ID`. Exit zero means the sink accepted responsibility for that
digest. The sink must accept repeated delivery of the same digest
idempotently.

Yield never stores sink arguments, environment values, stdout, or stderr. A
failed attempt stores only typed process information and optional diagnostic
digests. Credentials belong in the sink's environment or credential store.

A crash after remote acceptance but before local acceptance is recorded as
`delivery_unknown`. Retrying sends the same digest. Delivery order has no
meaning. A per-digest lock prevents concurrent delivery of one receipt while
allowing unrelated receipts to progress independently.

Yield supplies observation and experiment primitives. It does not select a
winning variant or grant any consumer authority to rewrite, install, merge,
release, or activate a proposal.

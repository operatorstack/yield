# Guarantees and limits

## What Yield enforces

- operation order expressed by the program;
- typed request and response envelopes;
- persistent append-only run state;
- per-step digest checks during replay;
- rejection of stale, duplicate, wrong-run, and schema-invalid responses;
- real command execution by the supervisor;
- requirements that prevent later completion after failure;
- recorded completed, blocked, and refused outcomes.

## What remains outside the guarantee

- A schema-valid model response may still be factually wrong.
- Portable mode cannot prove that an agent performed no work outside the
  requested operation.
- Yield does not sandbox commands or supply deployment, migration, or publishing
  logic.
- A fixture demonstrates declared paths; it is not automatically a complete
  behavioral-equivalence proof for converted prose.
- Yield is not a hosted runtime, workflow marketplace, or multi-agent
  orchestrator.

Use `RunCommand` for facts the machine can observe, `AskUser` for human
authority, and explicit tests for the paths that matter. The formal scope is
documented in [`locus-yield.md`](../locus-yield.md) and
[`locus-conformance.md`](../locus-conformance.md).

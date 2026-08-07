# yield.v1 IR — the canonical protocol surface

This directory is the language-neutral definition of everything that
crosses a Yield process boundary. The Go types in `internal/protocol` are
the reference implementation; `internal/protocol/ir_test.go` binds them to
these schemas so the IR cannot drift from the runtime. Every language SDK
(`sdk/typescript`, `sdk/python`, `sdk/yield` for Go) implements this
surface and nothing else.

## Files

| schema | what it defines |
|---|---|
| `yield.v1/request-envelope.schema.json` | one yielded operation, bound to run, sequence, and skill digest |
| `yield.v1/response-envelope.schema.json` | the answer for exactly one pending request |
| `yield.v1/journal.schema.json` | the replay input: run identity + answered operations in order |
| `yield.v1/program-output.schema.json` | the single output of one skill-program execution: `request` \| `terminal` \| `diverged` |

## The SDK execution contract

Every SDK must implement this tested contract:

1. Read the journal from the file named by `YIELD_JOURNAL`.
2. Re-execute the program from the top. For every operation the program
   produces while journal entries remain: recompute the request digest and
   compare with the recorded entry **before consuming its response**. On
   mismatch, emit `diverged` and exit. This per-step check is not
   optional. Without it, a changed operation could consume a recorded
   response meant for a different question.
3. At the first operation past the journal, emit a `request` output and
   exit. When the program returns, emit a `terminal` output
   (`completed` | `blocked` | `refused`; a failed requirement emits
   `requirement_failed` immediately).
4. Exactly one program-output object per execution, as a single JSON
   document on stdout. Nothing else goes to stdout.

## Request digest

`sha256( kind \0 id \0 compact(payload) \0 compact(output_schema) )`,
rendered as `sha256:<hex>`. `compact` removes insignificant JSON
whitespace; digests are therefore invariant under the log's
marshal/unmarshal round-trip. SDK-internal comparisons only ever compare
digests the same SDK computed, so byte-level JSON style differences
between languages cannot cause cross-language divergence.

## Determinism obligation

Code between yields must be deterministic — same journal, same operations,
in every language. Wall clocks, RNGs, environment reads, and filesystem
state are side effects: cross them through a yielded operation or leave
them out. The contract detects divergence; it cannot prevent
nondeterminism.

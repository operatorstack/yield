# Locus derivation — the Yield run lifecycle

Formal grounding for the V1 architecture. Models and the candidate
derivation live in `docs/locus/`; they were fidelity-certified against the
design artifact before implementation, so every claim below is a theorem
about the design model, discharged into system claims by the tests named
here.

## Models

| model | operators | verdict |
|---|---|---|
| `yield-protocol.json` | control.supervisory-rw | **controllable** — violations: `[]` |
| `yield-protocol.json` | control.nonblockingness | **nonblocking** — blocking states: `[]` |
| `yield-diag-portable.json` | control.diagnosability | **not diagnosable** — witness below |
| `yield-diag-correlated.json` | control.diagnosability | **diagnosable** — no indistinguishable pairs |

## What the theorems fixed in the design

1. **Protocol integrity is supervisable.** With `accept_stale` and
   `complete_unproven` modeled as controllable forbidden transitions, a
   supervisor can always prevent stale-response acceptance and
   completion-after-failed-requirement. The kernel's obligations name the
   refusing mechanisms the implementation must own; they are discharged by
   the refusing tests in `internal/guard/guard_test.go`:
   - `disable-mechanism:accept_stale` → `TestRefusesStaleResponse`,
     `TestRefusesDuplicateWithDifferentContent`
   - `disable-mechanism:accept_response` → `TestRefusesSchemaInvalidResult`
   - `disable-mechanism:complete_unproven` →
     `TestRefusesCompletionAfterFailedRequirement`,
     `engine.TestFailedRequirementBlocksRun`
   - `disable-mechanism:complete` → `TestAllowsCompletionWithPassedRequirements`

2. **Every run reaches an honest terminal — because the migrate verb
   exists.** Nonblockingness holds only with two controllable exits from
   `DIVERGED`: `migrate_digest` (`resume --accept-new-digest`) and
   `declare_blocked`. Without the migrate verb the design blocks; it is
   load-bearing, not a convenience. Discharged by
   `engine.TestDigestMismatchRefusedThenMigrates` and
   `engine.TestReplayDivergenceFailsLoudly`.

3. **Portable mode is provably non-diagnosable for off-protocol agent
   action.** Verbatim witness (indistinguishable pair): faulty
   `OFF_PROTOCOL` vs normal `PENDING_OP` — a run where the agent acted
   outside the yielded operation produces the same observable trace as an
   honest one. This is why the README's "not guaranteed" column exists,
   and why a correlated host adapter (same alphabet, `agent_off_protocol`
   observable) is the principled post-V1 slice: the rival-design
   derivation (`drv-998aec…`) shows it diagnosable with zero
   indistinguishable pairs.

## Claim scope

The models were certified against the design description, not a running
system; the run lifecycle claims descend to the implementation exactly as
far as the named tests carry them. The remaining undischarged honesty gap
is recorded in each model's `unknowns`: a schema-valid `agent_task` result
can still be fabricated — schema validity is not truth. `run_command` is
the exception by construction: the engine executes commands itself, so
their results enter the log as observed fact
(`engine.TestEndToEndRunResumeComplete` asserts the observed output).

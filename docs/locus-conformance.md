# Locus derivation — the SDK contract and the end-to-end conformance suite

Companion to `locus-yield.md` (the run-lifecycle models). This document
covers the language-interface design and the conformance suite that
discharges the whole program's obligations. Models and derivations live in
`docs/locus/`.

## Verdicts

| model | operator | verdict |
|---|---|---|
| `sdk-contract.json` | verification.trace-refinement | **refines** — the SDK execution automaton's observable stream is included in the supervisor's expectation (exactly one of request/terminal/diverged, then exit) |
| `sdk-contract.json` | control.nonblockingness | **nonblocking** — every SDK execution reaches an emit |
| `divergence-in-sdk.json` vs `divergence-supervisor-only.json` | verification.safety-reachability (rival designs, `drv-ad50b13e…`) | **decided** — in-SDK per-step checking satisfies `forbidden-unreachable`; supervisor-only is rejected with the verbatim trace `op_drifts → consume_unchecked → CONSUMED_MISMATCHED` |

The decided comparison is why per-step digest comparison is a MANDATORY
part of the SDK contract in every language, not an optional nicety: without
it, a drifted operation silently consumes a recorded response meant for a
different question, and every later step compounds the corruption.

## The contract every SDK implements

Go (`sdk/yield`), TypeScript (`sdk/typescript`), Python (`sdk/python`),
Rust (`sdk/rust`) each implement, and only implement, the certified
automaton: load journal → replay with per-step digest compare before
consuming → emit exactly one program output → exit. The canonical wire
surface is `ir/yield.v1/*.schema.json`;
`internal/protocol/ir_test.go` binds the Go reference types to the IR.

## The conformance suite (`internal/conformance`)

One IDENTICAL program in all four languages
(`testdata/skill-{go,ts,py,rs}`), driven through the real supervisor. The
scenario matrix and what each observes:

| test | observes |
|---|---|
| `TestCrossLanguageTraceEquality` | the core claim: one program, any language, the same observable protocol trace (sequence/kind/id + terminal + requirement count); every envelope IR-validated; `run_command` results are observed output; replay reproduces the terminal |
| `TestRefusedTerminal` | `declare_refused` in every language |
| `TestBlockedTerminal` | `declare_blocked` in every language |
| `TestFailedRequirementNeverCompletes` | `complete_unproven` refused; `run.blocked`, never `run.completed` |
| `TestGuardRefusals` | schema-invalid, duplicate-rewrite, wrong-run refusals through the live engine |
| `TestDivergenceFailsLoudlyEverywhere` | the decided design: a tampered recorded operation is detected by every SDK at replay |

Languages whose toolchain is missing are skipped locally; CI provides all
four (`.github/workflows/yield-lab.yml`).

## Discharge

`docs/locus/protocol-rw.discharge.json` records mechanism + refusing test
for every supervisory obligation of the lifecycle theorem. Verified:

```
locus obligations --operator control.supervisory-rw \
  --model docs/locus/yield-protocol.json --check docs/locus/protocol-rw.discharge.json
→ complete: true, undischarged: []
```

## Honest scope

The conformance suite proves the protocol machinery end-to-end: typed
operations, replay, refusals, terminals, cross-language equivalence. It
does not (and cannot) prove that a live agent performed only the requested
operations — that is the portable-mode diagnosability gap certified in
`locus-yield.md`, and the correlated-adapter slice is its answer.

# Yield

Programmable skills for coding agents. **Turn `SKILL.md` workflows into
resumable programs.**

> The skill yields the next typed operation. The coding agent performs it
> and resumes the skill.

Write control flow in Go. Yield user questions, agent tasks, and commands
to the coding agent. Resume from the result. No custom agent runtime
required — the agent only runs a CLI and follows envelopes.

A skill keeps its thin `SKILL.md` (so it works wherever skills work today)
and moves the part prose loses under context pressure — order, branching,
retries, approval, state, completion — into a deterministic program. The
model keeps reasoning, exploration, editing, and judgment.

## How it works

Deterministic re-execution: on every run/resume, `yskill` re-executes the
skill program from the top, feeding recorded responses back in order. At
the first unanswered operation the SDK emits a `yield.v1` request envelope
and the process exits — no daemon. A replayed step that produces a
different operation than the journal recorded is a divergence and fails
the run loudly; it never silently forks.

- **`yskill` (supervisor)** owns the append-only run log
  (`.yield/runs/<id>.jsonl`), sequence and digest binding, response
  validation, and every refusal (stale, duplicate, wrong-run,
  schema-invalid, digest-mismatch, completion-unproven).
- **The skill program** is an ordinary Go `main` using `sdk/yield`; every
  side effect crosses a yielded primitive.

Five primitives, two exits:

| primitive | who acts |
|---|---|
| `AskUser` | the agent asks through its normal interface |
| `AgentTask` | the model reasons; the result must be schema-valid JSON |
| `RunCommand` | **yskill executes it itself** — results are observed fact, not transcription |
| `Require` | a claim bound to evidence; failure makes completion structurally unreachable |
| `Complete` / `Blocked` / `Refused` | honest terminals, always recorded |

## Try it

```
go build -o yskill ./cmd/yskill
./yskill test examples/investigate     # scripted fixture run to completion
./yskill run examples/investigate      # prints the first operation envelope
./yskill init my-skill                 # scaffold, or wrap an existing prose skill
```

The reference skill, `examples/investigate`, encodes an investigation
discipline in code: at least three hypotheses, cheapest-to-disprove
first, at most three failed attempts, completion requires a causal chain
— or an honest `Blocked` at the frontier.

## What it guarantees — and what it doesn't

Guaranteed: deterministic control flow, typed requests/responses,
persistent state, replay (divergence fails loudly), stale/duplicate
rejection, evidence-bound completion.

Not guaranteed: that the agent performed *only* the requested operation,
or that a schema-valid `agent_task` result is true — schema validity is
not truth. `RunCommand` is the exception by construction: commands are
executed by the supervisor, so exit codes and output enter the log as
observed fact. The formal analysis behind this line is in
`docs/locus-yield.md`.

## What it is not

Not a daemon, not a hosted runtime, not a workflow DSL, not a
marketplace, not a new agent loop, not a multi-agent orchestrator, not a
security sandbox.

---

This repository is a one-directional projection of
`operatorstack/intelligence-flow` (`labs/22-yield`). Changes land via the
automated sync PR; do not edit files here directly. MIT licensed.

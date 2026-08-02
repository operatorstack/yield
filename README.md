# Yield

Programmable skills for coding agents. **Turn `SKILL.md` workflows into
resumable programs.**

> The skill yields the next typed operation. The coding agent performs it
> and resumes the skill.

Write control flow in TypeScript, Python, Go, or Rust. Yield user
questions, agent tasks, and commands to the coding agent. Resume from the
result. No custom agent runtime required — the agent only runs a CLI and
follows envelopes.

A skill keeps its thin `SKILL.md` (so it works wherever skills work today)
and moves the part prose loses under context pressure — order, branching,
retries, approval, state, completion — into a deterministic program. The
model keeps reasoning, exploration, editing, and judgment.

## Install

Choose one language package. It includes the SDK and the matching `yskill`
runtime.

```bash
# TypeScript
npm install @operatorstack/yield --registry=https://get.operatorstack.systems/npm/
npm exec -- yskill --version

# Python
python -m pip install yieldskill --index-url https://get.operatorstack.systems/pip/simple/
python -m yieldskill --version

# Go
GOPROXY=https://get.operatorstack.systems/go,direct \
  go install github.com/operatorstack/yield/cmd/yskill@latest
yskill --version

# Rust
cargo install yieldskill \
  --index sparse+https://get.operatorstack.systems/cargo/index/ --locked
yskill --version
```

## Create and register a workflow

Keep the real workflow beside the language dependencies it uses. Yield writes
small adapters into each coding agent's project skill directory; it does not
copy the workflow or install its dependencies again.

```bash
# TypeScript example
npm exec -- yskill init skills/review \
  --language typescript \
  --description "Review changed code when the user wants a branch checked before shipping."

# Detect installed agents, or pass --agent cursor,codex,claude-code
npm exec -- yskill register skills/review
npm exec -- yskill doctor skills/review --test
```

`yskill agents` lists the available agent IDs and project paths. Cursor,
Codex, and Claude Code are verified. Remaining entries support explicit path
registration from the pinned open registry; they are not presented as
end-to-end verified.

## How it works

Deterministic re-execution: on every run/resume, `yskill` re-executes the
skill program from the top, feeding recorded responses back in order. At
the first unanswered operation the SDK emits a `yield.v1` request envelope
and the process exits — no daemon. A replayed step that produces a
different operation than the journal recorded is a divergence and fails
the run loudly; it never silently forks.

- **`yskill`** owns the append-only run log
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

## Four languages, one protocol

Write the skill program in Go, TypeScript, Python, or Rust — the
runtime doesn't care. Every SDK implements the same certified
execution contract over the canonical `ir/yield.v1` schemas, and the
conformance suite (`internal/conformance`) runs the *same program* in all
four languages and asserts identical observable protocol behavior.

| language | SDK | example |
|---|---|---|
| Go | `sdk/yield` | `examples/investigate` — bounded hypothesis loop |
| TypeScript | `sdk/typescript` (`@operatorstack/yield`) | `examples/release-checklist` — human-gated deploy |
| Python | `sdk/python` (`yieldskill`) | `examples/env-doctor` — probe, branch, resume after the human |
| Rust | `sdk/rust` (`yieldskill`) | `examples/data-migration` — dry-run → approve → apply → verify |

Skills declare their language and runner in `skill.json`:
`{"version": 1, "language": "typescript", "run": ["node", "main.ts"]}`.

## Ten workflows, every language

The [example library](examples/library/) recreates ten common coding-agent
workflows independently in all four SDKs: branch review, failure
investigation, web QA, package release, issue triage, CI repair, dependency
upgrade, database migration, security audit, and iOS publishing.

Each language has the same workflow, a thin `SKILL.md`, and a scripted
fixture. Start from the work you already do instead of starting from a
framework tutorial.

## Documentation

Start with the [ten-minute TypeScript quickstart](docs/quickstart.md), then
use the documentation by job:

- [primitive guides](docs/primitives/README.md) — commands, model work,
  human input, evidence gates, and outcomes;
- [tutorials](docs/tutorials/README.md) — review, approval, environment
  repair, bounded debugging, and migration;
- [examples](docs/examples.md) — working programs in all four languages;
- [coding-agent setup](docs/agent-setup.md) — register one workflow with the
  agents used by the project;
- [evaluations](evals/README.md) — first-party workflow conformance and runtime
  invariant results, including the exact claim boundary;
- [convert an existing skill](docs/convert-existing-skill.md) — move
  control flow into code without claiming that fixture execution proves
  every reading of the original prose;
- [CLI and runtime reference](docs/reference/cli.md).

## Try it

```
go build -o yskill ./cmd/yskill
./yskill test examples/library/typescript/review-branch
./yskill test examples/library/python/review-branch
./yskill test examples/library/go/review-branch
./yskill test examples/library/rust/review-branch
YSKILL="$PWD/yskill" bash ./examples/library/test-all.sh
./yskill test examples/investigate        # Go: scripted fixture run to completion
./yskill test examples/release-checklist  # TypeScript (Node >= 23.6)
./yskill test examples/env-doctor         # Python 3.10+
./yskill test examples/data-migration     # Rust (cargo)
./yskill run  examples/investigate        # prints the first operation envelope
./yskill init my-skill --description "Run this workflow when ..."
./yskill register my-skill --agent codex # write a thin project adapter
./yskill doctor my-skill --agent codex   # verify package + adapter wiring
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
executed by the Yield CLI, so exit codes and output enter the log as
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

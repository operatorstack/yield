# Yield

Yield is an open-source execution runtime for programmable Agent Skill workflows.

**Write one skill workflow. Run it from your coding agents.**

Skill workflows are portable, executable processes that combine agent skills
with deterministic code, state, and verification.

Write the workflow in TypeScript, Python, Go, or Rust. Combine agent judgment,
real commands, human input, checks, and saved state. Yield generates the small
adapter each coding agent expects.

The split is small:

| term | meaning |
|---|---|
| **skill** | one reusable capability |
| **workflow** | order, branches, checks, and saved state |
| **skill workflow** | an executable composition of skills, code, commands, and human input |
| **adapter** | a generated `SKILL.md` that lets one coding agent discover the workflow |

The canonical skill workflow stays beside your code. Generated adapters are
disposable. The model keeps reasoning, exploration, editing, and judgment;
normal code owns the repeatable control flow.

## Install

Choose one language package. TypeScript and Python include a package-local
runtime. Go and Rust install the matching runtime under `.yield/bin` in the
repository. Generated adapters never use a global `yskill` from `PATH`.

```bash
# TypeScript (public npm)
npm install --save-exact @operatorstack/yield@0.1.29
npm exec -- yskill --version

# Python, after creating and activating .venv
python -m pip install yieldskill==0.1.29 --index-url https://get.operatorstack.systems/pip/simple/
python -m yieldskill --version

# Go, from the repository root
mkdir -p .yield/bin
GOBIN="$PWD/.yield/bin" GOPROXY=https://get.operatorstack.systems/go,direct \
  go install github.com/operatorstack/yield/cmd/yskill@v0.1.29
.yield/bin/yskill --version

# Rust, from the repository root
cargo install yieldskill@0.1.29 --root .yield \
  --index sparse+https://get.operatorstack.systems/cargo/index/ --locked
.yield/bin/yskill --version
```

Yield creates `.yield/.gitignore` when it registers a Go or Rust workflow, so
the local runtime and run state stay out of Git.
On Windows, run the local binary as `.\.yield\bin\yskill.exe`.

## Create and register a skill workflow

Keep the canonical workflow beside the language dependencies it uses. Yield writes
small adapters into each coding agent's project skill directory; it does not
copy the workflow or install its dependencies again.

```bash
# TypeScript example
npm exec -- yskill init skills/review \
  --language typescript \
  --description "Review changed code when the user wants a branch checked before shipping."

# Replace the intentionally incomplete starter and fixture, then check it.
npm exec -- yskill doctor skills/review --test

# Detect installed agents, or pass --agent cursor,codex,claude-code.
npm exec -- yskill register skills/review
```

`yskill agents` lists the available agent IDs and project paths. Cursor,
Codex, and Claude Code are verified. Remaining entries support explicit path
registration from the pinned open registry; they are not presented as
end-to-end verified.

## How a skill workflow runs

Deterministic re-execution: on every run/resume, `yskill` re-executes the
skill workflow from the top, feeding recorded responses back in order. At
the first unanswered operation the SDK emits a `yield.v1` request envelope
and the process exits — no daemon. A replayed step that produces a
different operation than the journal recorded is a divergence and fails
the run loudly; it never silently forks.

- **`yskill`** owns the append-only run log
  (`.yield/runs/<id>.jsonl`), sequence and digest binding, response
  validation, and every refusal (stale, duplicate, wrong-run,
  schema-invalid, digest-mismatch, completion-unproven).
- **The skill workflow** is an ordinary program using one Yield SDK; every
  side effect crosses a yielded primitive.

Five primitives, two exits:

| primitive | who acts |
|---|---|
| `AskUser` | the agent asks through its normal interface |
| `AgentTask` | the model reasons; the result must be schema-valid JSON |
| `RunCommand` | **yskill executes it itself** — results are observed fact, not transcription |
| `Require` | a claim bound to evidence; failure makes completion structurally unreachable |
| `Complete` / `Blocked` / `Refused` | honest terminals, always recorded |

## Four languages, one execution contract

Write the skill workflow in Go, TypeScript, Python, or Rust. Every SDK
implements the same certified execution contract, and the conformance suite
(`internal/conformance`) runs the same program in all four languages and
asserts identical observable behavior. The language-neutral schemas are
documented in the [runtime reference](docs/reference/sdk-parity.md).

| language | SDK | example |
|---|---|---|
| Go | `sdk/yield` | `examples/investigate` — bounded hypothesis loop |
| TypeScript | `sdk/typescript` (`@operatorstack/yield`) | `examples/release-checklist` — human-gated deploy |
| Python | `sdk/python` (`yieldskill`) | `examples/env-doctor` — probe, branch, resume after the human |
| Rust | `sdk/rust` (`yieldskill`) | `examples/data-migration` — dry-run → approve → apply → verify |

Skills declare their language and runner in `skill.json`:
`{"version": 1, "language": "typescript", "run": ["node", "main.ts"]}`.

## Ten skill workflows, every language

The [example library](examples/library/) implements ten common skill workflows
independently in all four SDKs: branch review, failure
investigation, web QA, package release, issue triage, CI repair, dependency
upgrade, database migration, security audit, and iOS publishing.

Each language has the same skill workflow, a thin adapter, and a scripted
fixture. Start from the work you already do instead of starting from a
framework tutorial.

## Documentation

Start with [what a skill workflow is](docs/skill-workflows.md), then build one
with the [ten-minute TypeScript quickstart](docs/quickstart.md). Continue with
the documentation for your job:

- [primitive guides](docs/primitives/README.md) — commands, model work,
  human input, evidence gates, and outcomes;
- [tutorials](docs/tutorials/README.md) — review, approval, environment
  repair, bounded debugging, and migration;
- [examples](docs/examples.md) — working programs in all four languages;
- [coding-agent setup](docs/agent-setup.md) — register one skill workflow with the
  agents used by the project;
- [Agent Plugins and Yield](docs/agent-plugins.md) — where portable packaging ends
  and workflow execution begins;
- [test workflow effects](docs/testing-fixtures.md) — deterministic fixture
  setup, response effects, standard-input JSON, and cleanup;
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
./yskill init my-skill --description "Run this skill workflow when ..."
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
observed fact. Runtime and conformance tests enforce these guarantees.

## What it is not

Not a daemon, not a hosted runtime, not a workflow DSL, not a
marketplace, not a new agent loop, not a multi-agent orchestrator, not a
security sandbox.

---

This is Yield's canonical source repository. Changes, verification, release
intent, and publishing control all live here. MIT licensed.

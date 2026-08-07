<p align="center">
  <a href="https://yield.operatorstack.systems/">
    <img src="https://raw.githubusercontent.com/operatorstack/yield/main/assets/yield-mark.svg" alt="Yield" />
  </a>
</p>

<h1 align="center">Yield</h1>

<p align="center"><strong>Move repeatable coding-agent instructions from words into code.</strong></p>

<p align="center">
  In-repository workflows for TypeScript, Python, Go, and Rust.
</p>

<p align="center">
  <a href="https://www.npmjs.com/package/@operatorstack/yield"><img alt="npm version" src="https://img.shields.io/npm/v/@operatorstack/yield?style=flat-square" /></a>
  <a href="https://github.com/operatorstack/yield/actions/workflows/verify.yml"><img alt="Build status" src="https://img.shields.io/github/actions/workflow/status/operatorstack/yield/verify.yml?branch=main&amp;style=flat-square&amp;label=build" /></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/npm/l/@operatorstack/yield?style=flat-square" /></a>
</p>

<p align="center">
  <a href="https://yield.operatorstack.systems/">Website</a> ·
  <a href="https://yield.operatorstack.systems/docs/">Documentation</a> ·
  <a href="https://www.npmjs.com/package/@operatorstack/yield">npm</a> ·
  <a href="https://github.com/operatorstack/yield">GitHub</a>
</p>

Yield turns repeated instructions for coding agents into typed, resumable
programs. The canonical workflow stays inside your repository beside the code
and dependencies it uses. Generated `SKILL.md` files only help coding agents
discover it.

Verified with Cursor, Codex, and Claude Code. Registry-backed project paths are
available for 73 more coding agents.

## Move repeated instructions into code

A release skill often starts as prose:

> Run the tests. Review the release. Stop if the review finds a critical issue.
> Ask me before publishing. Publish the package, then verify the registry.

Yield makes the order and stopping rules executable:

<!-- release-example:start -->
```typescript
import { defineSkill } from "@operatorstack/yield";

type Review = { critical: number; summary: string };

defineSkill((ctx) => {
  // Yield runs commands itself and records their output and exit status.
  const tests = ctx.runCommand("test", "echo tests-ok", 300);

  // A failed requirement stops the workflow and keeps its evidence.
  ctx.require(tests.exit_code === 0, "the test command succeeds", tests);

  // Review gives TypeScript its compile-time type. The JSON schema checks the
  // coding agent's response at runtime before this workflow can continue.
  const review = ctx.agentTask<Review>(
    "review-release",
    "Review this release. Report critical findings and a short summary.",
    { stdout: tests.stdout, stderr: tests.stderr },
    {
      type: "object",
      required: ["critical", "summary"],
      properties: {
        critical: { type: "integer", minimum: 0 },
        summary: { type: "string", minLength: 1 },
      },
    },
  );
  ctx.require(review.critical === 0, "the review has no critical findings", review);

  // Yield emits these fixed choices. A supported host may show native controls;
  // otherwise the coding agent asks through its normal interface.
  const approval = ctx.askUser("approve-publish", "Publish this package?", [
    { value: "yes", label: "Publish" },
    { value: "no", label: "Stop" },
  ]);
  if (approval !== "yes") ctx.refused("the operator declined publication");

  // Publishing cannot start before approval. Verification is a separate step,
  // so completion requires evidence that the registry contains the release.
  const publish = ctx.runCommand("publish", "echo publish-ok", 600);
  ctx.require(publish.exit_code === 0, "the publish command succeeds", publish);

  const registry = ctx.runCommand("verify-registry", "echo registry-ok", 300);
  ctx.require(registry.exit_code === 0, "the registry contains the release", registry);

  return { published: true, summary: review.summary };
});
```
<!-- release-example:end -->

The example uses harmless commands so its fixture can run in any checkout.
Replace them with the test, publish, and registry commands for your project.
The complete tested source is in
[`examples/release-checklist`](examples/release-checklist/).

## Use Yield in five steps

### 1. Install Yield

Install the TypeScript SDK and its repository-local CLI in your project:

```bash
npm install --save-exact @operatorstack/yield
npm exec -- yskill --version
```

[Public npm releases](https://www.npmjs.com/package/@operatorstack/yield)
use trusted publishing. The SDK package and all six runtime packages include
SLSA v1 provenance.

### 2. Create the workflow

```bash
npm exec -- yskill init skills/release \
  --language typescript \
  --description "Test, review, approve, publish, and verify a package."
```

The command creates one canonical workflow inside your repository:

```text
skills/
└── release/
    ├── SKILL.md
    ├── fixtures/
    │   ├── responses.json
    │   └── test.json
    ├── main.ts
    ├── package.json
    └── skill.json
```

Replace the starter in `skills/release/main.ts` with your workflow. Update
`skills/release/fixtures/responses.json` with deterministic answers for agent
and user operations.

### 3. Test the workflow

```bash
npm exec -- yskill doctor skills/release --test
```

This runs commands for real and supplies agent and user responses from the
fixture. A successful test reaches `completed` without leaving a run journal.

### 4. Register the skill

Registration is the discovery step. This command detects installed verified
agents and writes a small adapter for each one:

```bash
npm exec -- yskill register skills/release
```

Select verified agents explicitly when you do not want automatic detection:

```bash
npm exec -- yskill register skills/release \
  --agent cursor,codex,claude-code
```

If all three are selected, Yield creates these generated files:

```text
.cursor/skills/release/SKILL.md   # Cursor
.agents/skills/release/SKILL.md   # Codex
.claude/skills/release/SKILL.md   # Claude Code
```

The adapters point back to `skills/release`. They do not copy the workflow or
install its dependencies again.

### 5. Run the skill

Start a new coding-agent session so it discovers the registered skill. Where
slash skills are supported, run:

```text
/release
```

Otherwise, ask the agent in plain language:

```text
Use the release skill to publish this package.
```

The agent follows the generated adapter, runs the canonical workflow in
`skills/release`, and asks for each required agent or user response.

## How Yield runs and resumes

1. Your workflow emits one typed operation.
2. Yield records the request and exits. It does not run a daemon.
3. The coding agent, user, or CLI supplies the result.
4. Yield resumes from the journal and replays the program to the next operation.

If replay produces a different operation, the run fails instead of silently
forking. Every side effect crosses one of these primitives:

| Primitive | Purpose |
|---|---|
| `runCommand` | Execute a command and record its exit code and output. |
| `agentTask` | Ask the coding agent for schema-valid JSON. |
| `askUser` | Request an explicit human decision. |
| `require` | Bind a required claim to recorded evidence. |
| `blocked` / `refused` | Stop honestly when work cannot or must not continue. |

See the [primitive guides](docs/primitives/README.md) and
[runtime reference](docs/reference/cli.md) for the full contract.

## Languages and coding agents

All four SDKs implement the same execution contract. The conformance suite runs
the same program in every language and compares observable behavior.

| Language | SDK | Example |
|---|---|---|
| TypeScript | [`@operatorstack/yield`](sdk/typescript/) | [`release-checklist`](examples/release-checklist/) |
| Python | [`yieldskill`](sdk/python/) | [`env-doctor`](examples/env-doctor/) |
| Go | [`sdk/yield`](sdk/yield/) | [`investigate`](examples/investigate/) |
| Rust | [`yieldskill`](sdk/rust/) | [`data-migration`](examples/data-migration/) |

Cursor, Codex, and Claude Code are verified integrations. Yield also includes
registry-backed project paths for 73 more coding agents. Those paths support
explicit registration; they are not presented as end-to-end verified.

Run `yskill agents` to inspect the pinned registry and available project paths.

## Guarantees and limits

Yield provides deterministic control flow, typed requests and responses,
persistent run state, replay with divergence detection, stale and duplicate
response rejection, and evidence-bound completion.

Schema validity is not truth. Yield cannot prove that an agent performed only
the requested work. `runCommand` is different: the Yield CLI executes the
command, so the recorded exit code and output are observed facts.

Yield is not a daemon, hosted runtime, workflow DSL, marketplace, new agent
loop, multi-agent orchestrator, or security sandbox.

## Documentation and development

- [Read the public documentation](https://yield.operatorstack.systems/docs/)
- [What a skill workflow is](docs/skill-workflows.md)
- [Ten-minute TypeScript quickstart](docs/quickstart.md)
- [Working examples in all four languages](docs/examples.md)
- [Coding-agent setup](docs/agent-setup.md)
- [Testing workflow effects](docs/testing-fixtures.md)
- [Guarantees and evaluation results](evals/README.md)

Run the main checks from the repository root:

```bash
go test ./...
npm run test:release
```

The [example library](examples/library/) contains ten common workflows in all
four SDKs, including code review, failure investigation, CI repair, dependency
updates, database migration, security audit, and package release.

---

Yield is MIT licensed. This repository contains its canonical source and
versioned technical documentation.

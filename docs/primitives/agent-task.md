# AgentTask

Use the coding agent as a typed judgment step inside your workflow.

The agent interprets. With a JSON Schema, Yield checks the returned shape. Your
program still owns what happens next.

```text
workflow code
    ↓
AgentTask
    ↓
coding-agent judgment
    ↓
JSON result, schema-validated when requested
    ↓
workflow code continues
```

Use `AgentTask` when one step needs interpretation: review what a check may
have missed, diagnose captured output, compare designs against criteria, or
extract structured information from repository material.

## One workflow, three roles

| Role               | Primitive                  | Responsibility                                         |
| ------------------ | -------------------------- | ------------------------------------------------------ |
| deterministic work | `RunCommand` and `Require` | run commands and enforce requirements                  |
| agent judgment     | `AgentTask`                | interpret a bounded request and return structured data |
| human authority    | `AskUser`                  | make a decision before a protected effect              |

Not every workflow needs all three. Yield lets them work together without
asking the coding agent to rediscover order, retry limits, or completion rules
on every run.

## Example: check, review, approve, publish

This tested workflow runs a deterministic check, asks the coding agent to
review what that check may miss, requires a safe review result, then asks a
person before publishing.

<!-- release-example:start -->

```typescript
import { defineSkill } from "@operatorstack/yield"

type Review = { critical: number; summary: string }

defineSkill((ctx) => {
  // Yield runs commands itself and records their output and exit status.
  const tests = ctx.runCommand("test", "echo tests-ok", 300)

  // A failed requirement stops the workflow and keeps its evidence.
  ctx.require(tests.exit_code === 0, "the test command succeeds", tests)

  // Review gives TypeScript its compile-time type. The JSON schema checks the
  // coding agent's response at runtime before this workflow can continue.
  const review = ctx.agentTask<Review>(
    "review-release",
    "Review this release for correctness problems that the test command may miss. Report critical findings and a short summary.",
    { stdout: tests.stdout, stderr: tests.stderr },
    {
      type: "object",
      required: ["critical", "summary"],
      properties: {
        critical: { type: "integer", minimum: 0 },
        summary: { type: "string", minLength: 1 },
      },
    },
  )
  ctx.require(review.critical === 0, "the review has no critical findings", review)

  // Yield emits these fixed choices. A supported host may show native controls;
  // otherwise the coding agent asks through its normal interface.
  const approval = ctx.askUser("approve-publish", "Publish this package?", [
    { value: "yes", label: "Publish" },
    { value: "no", label: "Stop" },
  ])
  if (approval !== "yes") ctx.refused("the operator declined publication")

  // Publishing cannot start before approval. Verification is a separate step,
  // so completion requires evidence that the registry contains the release.
  const publish = ctx.runCommand("publish", "echo publish-ok", 600)
  ctx.require(publish.exit_code === 0, "the publish command succeeds", publish)

  const registry = ctx.runCommand("verify-registry", "echo registry-ok", 300)
  ctx.require(registry.exit_code === 0, "the registry contains the release", registry)

  return { published: true, summary: review.summary }
})
```

<!-- release-example:end -->

## Give the task the evidence it needs

The third argument is explicit workflow context. Pass the result that the
judgment depends on, such as command output, a diff summary, or a previous
structured result. Explicit context is easier to understand, test, and replay.

Yield sends the instruction and explicit context in the request. It does not
promise access to a complete conversation, the repository, or any hidden host
context. A coding agent may have additional working capabilities in its host,
but those capabilities are host-dependent.

Cursor, Codex, and Claude Code are verified integrations. The host still owns
the UI and working capabilities available to the agent.

## What schema validation proves

When you provide a JSON Schema, Yield validates the returned JSON before the
workflow continues. It proves that required fields and declared structural
constraints are present. It does not prove that the analysis is correct, files
were inspected, or the requested work happened.

Use `RunCommand` for machine-observed output, `Require` to control
continuation, and `AskUser` for human authority. Another `AgentTask` can offer
another judgment, but it is still model judgment.

## Test the surrounding workflow

In production, `AgentTask` waits for the coding agent. In a test,
`yskill test` reads a deterministic fixture response instead. The same
surrounding workflow logic still runs, including commands and requirements.

See [testing fixtures](../testing-fixtures.md) for
`fixtures/responses.json` and test-only effects.

## Common mistake

Do not put retry order, approval rules, or completion policy inside the
instruction. Keep those rules in the surrounding program where they can be
replayed and tested.

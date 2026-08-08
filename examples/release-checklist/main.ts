// Example skill (TypeScript): a release checklist where tests, review,
// approval, publishing, and registry verification cannot be skipped.
import { defineSkill } from "../../sdk/typescript/src/index.ts"

// README_EXAMPLE_START
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
// README_EXAMPLE_END

import { defineSkill } from "./yield-sdk.ts"

type Review = { status: "pass" | "needs_work"; critical: number; summary: string }
const reviewSchema = {
  type: "object",
  required: ["status", "critical", "summary"],
  properties: {
    status: { enum: ["pass", "needs_work"] },
    critical: { type: "integer", minimum: 0 },
    summary: { type: "string", minLength: 1 },
  },
}

defineSkill((ctx) => {
  const tests = ctx.runCommand("test-package", "node ../../bin/step.mjs test-package", 60)
  ctx.require(tests.exit_code === 0, "the package tests pass", tests)

  const review = ctx.agentTask<Review>(
    "review-release",
    "Read .eval/release.json from the project root. Return that exact JSON object as the release review result.",
    { stdout: tests.stdout, stderr: tests.stderr },
    reviewSchema,
  )
  ctx.require(review.status === "pass" && review.critical === 0, "the package is ready to publish", review)

  const approval = ctx.askUser("approve-publish", "Publish this package release?", [
    { value: "continue", label: "Continue" },
    { value: "stop", label: "Stop" },
  ])
  if (approval !== "continue") ctx.refused("the user declined to continue")

  const publish = ctx.runCommand("publish-package", "node ../../bin/step.mjs publish-package", 60)
  ctx.require(publish.exit_code === 0, "the package publish command succeeds", publish)

  const verify = ctx.runCommand("verify-package", "node ../../bin/step.mjs verify-package", 60)
  ctx.require(verify.exit_code === 0, "the published package resolves from the registry", verify)

  return { workflow: "release-package", summary: review.summary }
})

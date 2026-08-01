// Release a package. Replace the illustrative commands with your project commands.
import { defineSkill } from "../../../../sdk/typescript/src/index.ts";

type Decision = { status: "pass" | "needs_work"; critical: number; summary: string };
const decisionSchema = {
  "type": "object",
  "required": [
    "status",
    "critical",
    "summary"
  ],
  "properties": {
    "status": {
      "enum": [
        "pass",
        "needs_work"
      ]
    },
    "critical": {
      "type": "integer",
      "minimum": 0
    },
    "summary": {
      "type": "string",
      "minLength": 1
    }
  }
};

defineSkill((ctx) => {
  const preflight = ctx.runCommand("test-package", "printf 'package tests passed\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the package tests pass", preflight)

  const decision = ctx.agentTask<Decision>(
    "review-release",
    "Review the pending package release for breaking changes, missing notes, and rollback risk. Return pass only when it is ready to publish.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the package is ready to publish", decision)

  const approval = ctx.askUser("approve-publish", "Publish this package release?", [
    { value: "continue", label: "Continue" },
    { value: "stop", label: "Stop" },
  ])
  if (approval !== "continue") ctx.refused("the operator declined to continue")

  const action = ctx.runCommand("publish-package", "printf 'package published\\n'", 600)
  ctx.require(action.exit_code === 0, "the package publish command succeeds", action)

  const verify = ctx.runCommand("verify-package", "printf 'published package resolved from registry\\n'", 300)
  ctx.require(verify.exit_code === 0, "the published package resolves from the registry", verify)

  return { workflow: "release-package", summary: decision.summary }
});

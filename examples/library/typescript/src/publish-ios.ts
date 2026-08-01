// Publish an iOS build. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("archive-ios", "printf 'iOS archive and tests passed\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the iOS archive and tests pass", preflight)

  const decision = ctx.agentTask<Decision>(
    "review-ios-release",
    "Review the iOS release metadata, versioning, privacy notes, and rollout risk. Return pass only when the build is ready for upload.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the iOS build is ready for upload", decision)

  const approval = ctx.askUser("approve-ios-upload", "Upload this iOS build to App Store Connect?", [
    { value: "continue", label: "Continue" },
    { value: "stop", label: "Stop" },
  ])
  if (approval !== "continue") ctx.refused("the operator declined to continue")

  const action = ctx.runCommand("upload-ios", "printf 'iOS build uploaded\\n'", 600)
  ctx.require(action.exit_code === 0, "the iOS upload command succeeds", action)

  const verify = ctx.runCommand("verify-ios-processing", "printf 'uploaded build entered processing\\n'", 300)
  ctx.require(verify.exit_code === 0, "the uploaded iOS build entered processing", verify)

  return { workflow: "publish-ios", summary: decision.summary }
});

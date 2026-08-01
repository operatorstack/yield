// Upgrade a dependency. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("baseline-tests", "printf 'baseline tests passed\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the baseline tests pass", preflight)

  const decision = ctx.agentTask<Decision>(
    "review-upgrade",
    "Review the dependency upgrade for API changes, migration work, and rollback risk. Return pass only when the change is bounded.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the dependency upgrade has a bounded plan", decision)

  const approval = ctx.askUser("approve-upgrade", "Apply the reviewed dependency upgrade?", [
    { value: "continue", label: "Continue" },
    { value: "stop", label: "Stop" },
  ])
  if (approval !== "continue") ctx.refused("the operator declined to continue")

  const action = ctx.runCommand("apply-upgrade", "printf 'dependency upgraded\\n'", 600)
  ctx.require(action.exit_code === 0, "the dependency upgrade command succeeds", action)

  const verify = ctx.runCommand("post-upgrade-tests", "printf 'post-upgrade tests passed\\n'", 300)
  ctx.require(verify.exit_code === 0, "the tests pass after the dependency upgrade", verify)

  return { workflow: "upgrade-dependency", summary: decision.summary }
});

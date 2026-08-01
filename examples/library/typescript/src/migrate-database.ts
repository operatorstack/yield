// Run a database migration. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("dry-run-migration", "printf 'dry run: add users_email_idx concurrently\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the migration dry-run succeeds", preflight)

  const decision = ctx.agentTask<Decision>(
    "review-migration",
    "Review the migration plan for lock risk, irreversible work, and rollback. Return pass only when the plan is safe to apply.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the migration plan has acceptable risk", decision)

  const approval = ctx.askUser("approve-migration", "Apply the reviewed database migration?", [
    { value: "continue", label: "Continue" },
    { value: "stop", label: "Stop" },
  ])
  if (approval !== "continue") ctx.refused("the operator declined to continue")

  const action = ctx.runCommand("apply-migration", "printf 'migration applied\\n'", 600)
  ctx.require(action.exit_code === 0, "the migration applies cleanly", action)

  const verify = ctx.runCommand("verify-migration", "printf 'migration verification passed\\n'", 300)
  ctx.require(verify.exit_code === 0, "the migrated database passes verification", verify)

  return { workflow: "migrate-database", summary: decision.summary }
});

// Review a branch. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("check-branch", "printf 'typecheck and tests passed\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the branch passes mechanical checks", preflight)

  const decision = ctx.agentTask<Decision>(
    "review-diff",
    "Review the branch for correctness, security, data-loss risks, and missing tests. Return pass only when no critical finding remains.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the review has no critical findings", decision)

  return { workflow: "review-branch", summary: decision.summary }
});

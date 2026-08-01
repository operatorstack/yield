// Audit a change for security. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("run-security-checks", "printf 'dependency and secret scans completed\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the mechanical security checks complete", preflight)

  const decision = ctx.agentTask<Decision>(
    "review-trust-boundaries",
    "Review authentication, authorization, input handling, secrets, and trust-boundary changes. Return pass only when no critical risk remains.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the change has no critical security finding", decision)

  return { workflow: "audit-security", summary: decision.summary }
});

// Triage an issue. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("read-issue", "printf 'issue: intermittent timeout after retry change\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the issue report is available", preflight)

  const decision = ctx.agentTask<Decision>(
    "classify-issue",
    "Classify severity, identify missing evidence, and propose exactly one next action. Return pass only when the summary is actionable.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the issue has one actionable next step", decision)

  return { workflow: "triage-issue", summary: decision.summary }
});

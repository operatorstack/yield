// Investigate a failure. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("capture-failure", "printf 'failing test captured with recent diff\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the failure evidence is captured", preflight)

  const decision = ctx.agentTask<Decision>(
    "diagnose-cause",
    "Use the failure output and recent change to identify the most likely root cause. Return pass only when the summary states a causal chain.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the diagnosis states a supported cause", decision)

  return { workflow: "investigate-failure", summary: decision.summary }
});

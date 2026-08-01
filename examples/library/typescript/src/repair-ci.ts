// Repair a CI failure. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("capture-ci-log", "printf 'ci log: test shard 2 failed after cache restore\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the failing CI evidence is captured", preflight)

  const decision = ctx.agentTask<Decision>(
    "plan-ci-repair",
    "Diagnose the CI failure and describe the smallest supported repair. Return pass only when the repair is tied to the observed log.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the CI repair is supported by the failure evidence", decision)

  const action = ctx.runCommand("apply-ci-repair", "printf 'ci repair applied\\n'", 600)
  ctx.require(action.exit_code === 0, "the CI repair command succeeds", action)

  const verify = ctx.runCommand("rerun-ci-check", "printf 'failing CI check now passes\\n'", 300)
  ctx.require(verify.exit_code === 0, "the previously failing CI check passes", verify)

  return { workflow: "repair-ci", summary: decision.summary }
});

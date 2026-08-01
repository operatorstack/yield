// QA a web change. Replace the illustrative commands with your project commands.
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
  const preflight = ctx.runCommand("build-web", "printf 'build passed; changed routes: / and /settings\\n'", 300)
  ctx.require(preflight.exit_code === 0, "the web application builds", preflight)

  const decision = ctx.agentTask<Decision>(
    "test-changed-routes",
    "Test the changed routes at desktop and mobile sizes, including keyboard navigation and form errors. Return pass only when no blocking regression remains.",
    { stdout: preflight.stdout, stderr: preflight.stderr },
    decisionSchema,
  )
  ctx.require(decision.status === "pass" && decision.critical === 0, "the changed routes have no blocking regression", decision)

  return { workflow: "qa-web-change", summary: decision.summary }
});

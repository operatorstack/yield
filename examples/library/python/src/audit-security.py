# Audit a change for security. Replace the illustrative commands with your project commands.
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "sdk" / "python"))
from yieldskill import define_skill  # noqa: E402

DECISION_SCHEMA = {
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
}

def program(ctx):
    preflight = ctx.run_command("run-security-checks", "printf 'dependency and secret scans completed\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the mechanical security checks complete", preflight)

    decision = ctx.agent_task(
        "review-trust-boundaries",
        "Review authentication, authorization, input handling, secrets, and trust-boundary changes. Return pass only when no critical risk remains.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the change has no critical security finding", decision)

    return {"workflow": "audit-security", "summary": decision["summary"]}

define_skill(program)

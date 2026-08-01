# QA a web change. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("build-web", "printf 'build passed; changed routes: / and /settings\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the web application builds", preflight)

    decision = ctx.agent_task(
        "test-changed-routes",
        "Test the changed routes at desktop and mobile sizes, including keyboard navigation and form errors. Return pass only when no blocking regression remains.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the changed routes have no blocking regression", decision)

    return {"workflow": "qa-web-change", "summary": decision["summary"]}

define_skill(program)

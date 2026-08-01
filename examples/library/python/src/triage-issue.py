# Triage an issue. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("read-issue", "printf 'issue: intermittent timeout after retry change\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the issue report is available", preflight)

    decision = ctx.agent_task(
        "classify-issue",
        "Classify severity, identify missing evidence, and propose exactly one next action. Return pass only when the summary is actionable.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the issue has one actionable next step", decision)

    return {"workflow": "triage-issue", "summary": decision["summary"]}

define_skill(program)

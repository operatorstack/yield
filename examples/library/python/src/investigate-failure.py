# Investigate a failure. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("capture-failure", "printf 'failing test captured with recent diff\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the failure evidence is captured", preflight)

    decision = ctx.agent_task(
        "diagnose-cause",
        "Use the failure output and recent change to identify the most likely root cause. Return pass only when the summary states a causal chain.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the diagnosis states a supported cause", decision)

    return {"workflow": "investigate-failure", "summary": decision["summary"]}

define_skill(program)

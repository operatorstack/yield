# Upgrade a dependency. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("baseline-tests", "printf 'baseline tests passed\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the baseline tests pass", preflight)

    decision = ctx.agent_task(
        "review-upgrade",
        "Review the dependency upgrade for API changes, migration work, and rollback risk. Return pass only when the change is bounded.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the dependency upgrade has a bounded plan", decision)

    approval = ctx.ask_user(
        "approve-upgrade",
        "Apply the reviewed dependency upgrade?",
        options=[{"value": "continue", "label": "Continue"}, {"value": "stop", "label": "Stop"}],
    )
    if approval != "continue":
        ctx.refused("the operator declined to continue")

    action = ctx.run_command("apply-upgrade", "printf 'dependency upgraded\\n'", 600)
    ctx.require(action.exit_code == 0, "the dependency upgrade command succeeds", action)

    verify = ctx.run_command("post-upgrade-tests", "printf 'post-upgrade tests passed\\n'", 300)
    ctx.require(verify.exit_code == 0, "the tests pass after the dependency upgrade", verify)

    return {"workflow": "upgrade-dependency", "summary": decision["summary"]}

define_skill(program)

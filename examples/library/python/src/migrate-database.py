# Run a database migration. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("dry-run-migration", "printf 'dry run: add users_email_idx concurrently\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the migration dry-run succeeds", preflight)

    decision = ctx.agent_task(
        "review-migration",
        "Review the migration plan for lock risk, irreversible work, and rollback. Return pass only when the plan is safe to apply.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the migration plan has acceptable risk", decision)

    approval = ctx.ask_user(
        "approve-migration",
        "Apply the reviewed database migration?",
        options=[{"value": "continue", "label": "Continue"}, {"value": "stop", "label": "Stop"}],
    )
    if approval != "continue":
        ctx.refused("the operator declined to continue")

    action = ctx.run_command("apply-migration", "printf 'migration applied\\n'", 600)
    ctx.require(action.exit_code == 0, "the migration applies cleanly", action)

    verify = ctx.run_command("verify-migration", "printf 'migration verification passed\\n'", 300)
    ctx.require(verify.exit_code == 0, "the migrated database passes verification", verify)

    return {"workflow": "migrate-database", "summary": decision["summary"]}

define_skill(program)

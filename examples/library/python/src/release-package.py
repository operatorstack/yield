# Release a package. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("test-package", "printf 'package tests passed\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the package tests pass", preflight)

    decision = ctx.agent_task(
        "review-release",
        "Review the pending package release for breaking changes, missing notes, and rollback risk. Return pass only when it is ready to publish.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the package is ready to publish", decision)

    approval = ctx.ask_user(
        "approve-publish",
        "Publish this package release?",
        options=[{"value": "continue", "label": "Continue"}, {"value": "stop", "label": "Stop"}],
    )
    if approval != "continue":
        ctx.refused("the operator declined to continue")

    action = ctx.run_command("publish-package", "printf 'package published\\n'", 600)
    ctx.require(action.exit_code == 0, "the package publish command succeeds", action)

    verify = ctx.run_command("verify-package", "printf 'published package resolved from registry\\n'", 300)
    ctx.require(verify.exit_code == 0, "the published package resolves from the registry", verify)

    return {"workflow": "release-package", "summary": decision["summary"]}

define_skill(program)

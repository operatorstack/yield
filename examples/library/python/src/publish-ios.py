# Publish an iOS build. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("archive-ios", "printf 'iOS archive and tests passed\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the iOS archive and tests pass", preflight)

    decision = ctx.agent_task(
        "review-ios-release",
        "Review the iOS release metadata, versioning, privacy notes, and rollout risk. Return pass only when the build is ready for upload.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the iOS build is ready for upload", decision)

    approval = ctx.ask_user(
        "approve-ios-upload",
        "Upload this iOS build to App Store Connect?",
        options=[{"value": "continue", "label": "Continue"}, {"value": "stop", "label": "Stop"}],
    )
    if approval != "continue":
        ctx.refused("the operator declined to continue")

    action = ctx.run_command("upload-ios", "printf 'iOS build uploaded\\n'", 600)
    ctx.require(action.exit_code == 0, "the iOS upload command succeeds", action)

    verify = ctx.run_command("verify-ios-processing", "printf 'uploaded build entered processing\\n'", 300)
    ctx.require(verify.exit_code == 0, "the uploaded iOS build entered processing", verify)

    return {"workflow": "publish-ios", "summary": decision["summary"]}

define_skill(program)

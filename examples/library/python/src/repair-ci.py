# Repair a CI failure. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("capture-ci-log", "printf 'ci log: test shard 2 failed after cache restore\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the failing CI evidence is captured", preflight)

    decision = ctx.agent_task(
        "plan-ci-repair",
        "Diagnose the CI failure and describe the smallest supported repair. Return pass only when the repair is tied to the observed log.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the CI repair is supported by the failure evidence", decision)

    action = ctx.run_command("apply-ci-repair", "printf 'ci repair applied\\n'", 600)
    ctx.require(action.exit_code == 0, "the CI repair command succeeds", action)

    verify = ctx.run_command("rerun-ci-check", "printf 'failing CI check now passes\\n'", 300)
    ctx.require(verify.exit_code == 0, "the previously failing CI check passes", verify)

    return {"workflow": "repair-ci", "summary": decision["summary"]}

define_skill(program)

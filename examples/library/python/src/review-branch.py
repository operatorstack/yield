# Review a branch. Replace the illustrative commands with your project commands.
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
    preflight = ctx.run_command("check-branch", "printf 'typecheck and tests passed\\n'", 300)
    ctx.require(preflight.exit_code == 0, "the branch passes mechanical checks", preflight)

    decision = ctx.agent_task(
        "review-diff",
        "Review the branch for correctness, security, data-loss risks, and missing tests. Return pass only when no critical finding remains.",
        context={"stdout": preflight.stdout, "stderr": preflight.stderr},
        schema=DECISION_SCHEMA,
    )
    ctx.require(decision["status"] == "pass" and decision["critical"] == 0, "the review has no critical findings", decision)

    return {"workflow": "review-branch", "summary": decision["summary"]}

define_skill(program)

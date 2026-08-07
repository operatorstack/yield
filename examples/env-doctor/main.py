# Example skill (Python): an environment doctor. Probing and branching are
# code; interpreting the probe is the model's; the user is only asked when
# something needs their hands.
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "sdk", "python"))

from yieldskill import define_skill  # noqa: E402


# README_EXAMPLE_START
def program(ctx):
    probe = ctx.run_command("probe-python", "python3 --version || python --version", timeout_seconds=60)

    diagnosis = ctx.agent_task(
        "diagnose",
        "Given the probe output, is this environment healthy for the project? "
        "If not, state the single most likely fix.",
        context={"exit_code": probe.exit_code, "stdout": probe.stdout, "stderr": probe.stderr},
        schema={
            "type": "object",
            "required": ["healthy"],
            "properties": {
                "healthy": {"type": "boolean"},
                "fix_hint": {"type": "string"},
            },
        },
    )

    if not diagnosis["healthy"]:
        answer = ctx.ask_user(
            "apply-fix",
            f"The environment needs a fix: {diagnosis.get('fix_hint', 'unknown')}. Apply it now and reply done.",
            options=[{"value": "done"}, {"value": "skip"}],
        )
        if answer != "done":
            ctx.blocked("the environment fix was not applied")
        recheck = ctx.run_command("recheck-python", "python3 --version || python --version", timeout_seconds=60)
        ctx.require(recheck.exit_code == 0, "the environment probe passes after the fix", recheck)
        return {"healthy": True, "fixed": True}

    ctx.require(probe.exit_code == 0, "the environment probe passes", probe)
    return {"healthy": True, "fixed": False}


define_skill(program)
# README_EXAMPLE_END

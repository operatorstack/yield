# Conformance program (Python). The SAME program exists in Go, TypeScript,
# and Rust; the harness asserts identical observable protocol behavior.
import os
import sys

sys.path.insert(
    0,
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..", "..", "sdk", "python"),
)

from yieldskill import define_skill  # noqa: E402


def program(ctx):
    proceed = ctx.ask_user(
        "q1-proceed",
        "Proceed with the conformance run?",
        options=[{"value": "yes", "label": "Yes"}, {"value": "no", "label": "No"}],
    )
    if proceed == "no":
        ctx.refused("operator declined")

    t = ctx.agent_task(
        "t2-analyze",
        'Return {"n": <integer>}.',
        context={"proceed": proceed},
        schema={"type": "object", "required": ["n"], "properties": {"n": {"type": "integer"}}},
    )
    if t["n"] == 0:
        ctx.blocked("n is zero: a true frontier")

    c = ctx.run_command("c3-echo", "echo conform-ok")
    ctx.require(t["n"] > 0, "n is positive", {"n": t["n"]})
    ctx.require(c.exit_code == 0, "the echo command passes", {"exit_code": c.exit_code})

    return {"n": t["n"]}


define_skill(program)

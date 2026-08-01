# yieldskill — Yield skill-program SDK for Python

The Python implementation of the yield.v1 SDK execution contract (see
`ir/README.md`). The import name is `yieldskill` because `yield` is a
Python keyword.

```python
from yieldskill import define_skill

def program(ctx):
    answer = ctx.ask_user("confirm-start", "Ready to start?")
    if answer != "yes":
        ctx.refused("user declined to start")
    tests = ctx.run_command("run-tests", "true", timeout_seconds=60)
    ctx.require(tests.exit_code == 0, "the test command passes", tests)
    return {"status": "ok"}

define_skill(program)
```

Declare the runner in the skill's `skill.json`:

```json
{ "run": ["python3", "main.py"] }
```

Programs must be deterministic between yields — same journal, same
operations, every execution. Clocks, RNGs, and filesystem reads are side
effects: cross them through a yielded operation or leave them out.

# Tutorial: resume after an environment repair

Some workflows need a person to change something outside the agent session.
The run should wait without losing its earlier evidence.

The Python environment-doctor example follows this path:

1. `run_command` records the current probe.
2. `agent_task` interprets the output.
3. If unhealthy, `ask_user` explains the fix and waits.
4. A second `run_command` checks the environment again.
5. `require` prevents completion unless the recheck succeeds.

```python
probe = ctx.run_command("probe-python", "python3 --version", timeout_seconds=60)
diagnosis = ctx.agent_task("diagnose", instruction, context=probe_context, schema=schema)

if not diagnosis["healthy"]:
    answer = ctx.ask_user("apply-fix", diagnosis["fix_hint"], options=options)
    if answer != "done":
        ctx.blocked("the environment fix was not applied")
    recheck = ctx.run_command("recheck-python", "python3 --version", timeout_seconds=60)
    ctx.require(recheck.exit_code == 0, "the environment probe passes", recheck)
```

Run the complete example:

```bash
yskill test examples/env-doctor
```

Source: [`examples/env-doctor/main.py`](../../examples/env-doctor/main.py).

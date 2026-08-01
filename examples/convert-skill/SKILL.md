---
name: convert-skill
description: Convert an existing prose SKILL.md into a Yield program in the language of your choice, verified by executing the result.
---

Run:

    yskill run .

Follow each returned operation exactly.

- `ask_user`: ask the user using the host's normal interface.
- `agent_task`: perform the task and return schema-valid JSON. For
  `write-skill` and `fix-generated-*`, actually create or edit the files
  on disk — the next operation executes them.
- `run_command`: yskill executes it itself; you will not see this kind.

Resume the run after each operation:

    yskill resume <run-id> --response response.json --skill .

Do not skip an operation or invent its response. The program owns the
pipeline: read the prose, extract the flow, pick the language, write the
program — and completion requires the generated skill to pass its own
fixture run under `yskill test`. A conversion that was never executed is
never "done".

---
name: release-checklist
description: Deploy with an explicit human gate, build verification, and evidence-bound completion.
---

Run:

    yskill run .

Follow each returned operation exactly.

- `ask_user`: ask the user using the host's normal interface.
- `agent_task`: perform the task and return schema-valid JSON.
- `run_command`: yskill executes it itself; you will not see this kind.

Resume the run after each operation:

    yskill resume <run-id> --response response.json --skill .

Do not skip an operation or invent its response. The program enforces the
order: approval before build, build before notes, notes before deploy —
and completion requires both commands' observed exit codes.

---
name: data-migration
description: Dry-run, human approval, apply, verify — with the audit trail in the run log.
---

Run:

    yskill run .

Follow each returned operation exactly.

- `ask_user`: ask the user using the host's normal interface.
- `agent_task`: perform the task and return schema-valid JSON.
- `run_command`: yskill executes it itself; you will not see this kind.

Resume the run after each operation:

    yskill resume <run-id> --response response.json --skill .

Do not skip an operation or invent its response. The program orders the
flow — dry-run before approval, approval before apply, verify before
completion — and every irreversible step has a recorded, approved request
before it.

---
name: release-checklist
description: Test, review, approve, publish, and verify a package in a fixed order.
---

Run:

    yskill run .

Follow each returned operation exactly.

- `agent_task`: perform the task and return schema-valid JSON.
- `ask_user`: ask the user through the host's normal interface.
- `run_command`: yskill executes it itself; you will not see this kind.

Resume the run after each operation:

    yskill resume <run-id> --response response.json --skill .

Do not skip an operation or invent its response. The program enforces this
order: test, review, approval, publish, then registry verification. Critical
review findings and failed commands stop completion.

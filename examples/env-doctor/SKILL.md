---
name: env-doctor
description: Probe the environment, branch on findings, ask the user only when something needs their hands.
---

Run:

    yskill run .

Follow each returned operation exactly.

- `ask_user`: ask the user using the host's normal interface.
- `agent_task`: perform the task and return schema-valid JSON.
- `run_command`: yskill executes it itself; you will not see this kind.

Resume the run after each operation:

    yskill resume <run-id> --response response.json --skill .

Do not skip an operation or invent its response. The run waits on disk
while the user fixes their environment — resume it from any session.

---
name: investigate
description: Investigate a failure with bounded hypotheses, cheap-first probes, and an evidence-bound conclusion.
---

Run:

    yskill run .

Follow each returned operation exactly.

- `ask_user`: ask the user using the host's normal interface.
- `agent_task`: perform the task and return schema-valid JSON.
- `run_command`: yskill executes it itself; you will not see this kind.

Resume the run after each operation:

    yskill resume <run-id> --response response.json --skill .

Do not skip an operation or invent its response. The program bounds the
investigation: at least three hypotheses, cheapest-to-disprove first, at
most three failed attempts, and completion requires a causal chain.

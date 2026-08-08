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
pipeline: read the prose, pick the language, project its meaning, extract the
flow, and write the program. The projection is:

    C = clauses(S)
    Pi(S) = {(c, d, T, r) | c in C}

Each clause is control, guidance, both, or excluded. Control reaches code.
Guidance stays in the canonical SKILL.md or a relevant `agent_task`. Both
reaches both. An excluded clause has no destination and has a reason. Every
destination remains reachable by the coding agent. The map stays in the Yield
run log. It is not a destination artifact.

Completion requires the generated skill to pass its own fixture run under
`yskill test`. A conversion that was never executed is never "done".

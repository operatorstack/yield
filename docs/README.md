# Yield documentation

Move repeatable coding-agent instructions from words into code.

The [public documentation](https://yield.operatorstack.systems/docs/) is the
best place to start. This directory contains the versioned technical source
that stays with each Yield revision.

You may already be repeating a workflow in a prompt:

> Run the checks. Review the diff. Stop if anything critical remains. Ask me
> before publishing. If the session ends, start again without losing our place.

Yield lets you keep the useful words and put the repeatable flow in normal
code. The coding agent still investigates, reviews, edits, and explains. Your
program decides which operation comes next, what evidence must exist, and when
the run is finished.

A **skill workflow** is a portable, executable process that combines agent
skills with deterministic code, state, and verification. The canonical
workflow is the source you edit. Generated adapters let coding agents discover
and start it.

## Start here

1. [Read the public guide](https://yield.operatorstack.systems/docs/) — the
   quickest path from installation to a running workflow.
2. [Understand skill workflows](skill-workflows.md) — the canonical workflow,
   generated adapter, and execution boundary.
3. [Build and run your first skill workflow](quickstart.md) — a TypeScript
   workflow you can test in about ten minutes.
4. [Register it with your coding agents](agent-setup.md) — keep one workflow
   and generate the small discovery adapters each agent needs.
5. [Learn the primitives](primitives/README.md) — commands, model work, human
   input, gates, and honest outcomes.
6. [Follow a complete tutorial](tutorials/README.md) — review, approval,
   environment repair, bounded debugging, and migration.
7. [Browse the examples](examples.md) — working skill workflows in Go,
   TypeScript, Python, and Rust.
8. [Convert an existing prose skill](convert-existing-skill.md) — use Yield's
   verified converter after you understand one ordinary workflow.

## The split to remember

| Put in code | Leave with the model |
|---|---|
| order and branching | investigation and judgment |
| retry limits | reading unfamiliar code |
| commands that must really run | proposing changes |
| approval points | writing explanations |
| evidence required to finish | interpreting evidence |

This is not a new agent loop or a hosted agent runtime. A thin `SKILL.md` starts
the program, the program emits one typed operation, and the coding agent
performs that operation through its normal interface. Yield records the
response and resumes from the next unanswered operation.

## Reference

- [Skill workflow concepts](skill-workflows.md)
- [CLI commands](reference/cli.md)
- [Coding-agent registration](agent-setup.md)
- [Run, pause, resume, and replay](reference/execution-model.md)
- [The four SDKs](reference/sdk-parity.md)
- [Guarantees and limits](reference/guarantees.md)

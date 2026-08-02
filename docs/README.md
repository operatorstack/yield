# Yield documentation

You already know the workflow. You may be repeating it in a prompt:

> Run the checks. Review the diff. Stop if anything critical remains. Ask me
> before publishing. If the session ends, start again without losing our place.

Yield lets you keep the useful words and put the order in normal code. The
coding agent still investigates, reviews, edits, and explains. Your program
decides which operation comes next, what evidence must exist, and when the run
is finished.

## Start here

1. [Build and run your first skill](quickstart.md) — a TypeScript workflow you
   can test in about ten minutes.
2. [Register it with your coding agents](agent-setup.md) — keep one workflow
   and generate the small discovery adapters each agent needs.
3. [Learn the primitives](primitives/README.md) — commands, model work, human
   input, gates, and honest outcomes.
4. [Follow a complete tutorial](tutorials/README.md) — review, approval,
   environment repair, bounded debugging, and migration.
5. [Browse the examples](examples.md) — working programs in Go, TypeScript,
   Python, and Rust.
6. [Convert an existing prose skill](convert-existing-skill.md) — use Yield's
   verified converter after you understand one ordinary workflow.

## The split to remember

| Put in code | Leave with the model |
|---|---|
| order and branching | investigation and judgment |
| retry limits | reading unfamiliar code |
| commands that must really run | proposing changes |
| approval points | writing explanations |
| evidence required to finish | interpreting evidence |

This is not a new agent runtime. A thin `SKILL.md` starts the program, the
program emits one typed operation, and the coding agent performs that operation
through its normal interface. Yield records the response and resumes from the
next unanswered operation.

## Reference

- [CLI commands](reference/cli.md)
- [Coding-agent registration](agent-setup.md)
- [Run, pause, resume, and replay](reference/execution-model.md)
- [The four SDKs](reference/sdk-parity.md)
- [Guarantees and limits](reference/guarantees.md)

# Yield primitives

Yield has a deliberately small API. Each primitive has one clear owner.

| Primitive | What it does | Who performs it |
|---|---|---|
| [`RunCommand`](run-command.md) | Runs a command and records its real output | `yskill` |
| [`AgentTask`](agent-task.md) | Requests model judgment with an optional JSON schema | coding agent |
| [`AskUser`](ask-user.md) | Pauses for a human answer | coding agent and user |
| [`Require`](require.md) | Prevents completion unless a claim passes | skill program |
| [Outcomes](outcomes.md) | Completes, blocks, or refuses with a recorded reason | skill program |

Ordinary language features provide the rest. Use `if` for choices, `for` or
`while` for bounded retries, functions for reusable flows, and your language's
types for local data.

## Names in each SDK

| Meaning | TypeScript | Python | Go | Rust |
|---|---|---|---|---|
| ask a person | `ctx.askUser` | `ctx.ask_user` | `ctx.AskUser` | `ctx.ask_user` |
| ask the model | `ctx.agentTask` | `ctx.agent_task` | `ctx.AgentTask` | `ctx.agent_task` |
| run a command | `ctx.runCommand` | `ctx.run_command` | `ctx.RunCommand` | `ctx.run_command` |
| enforce a claim | `ctx.require` | `ctx.require` | `ctx.Require` | `ctx.require` |
| finish | `return value` | `return value` | `ctx.Complete` | `Ok(value)` |
| cannot continue | `ctx.blocked` | `ctx.blocked` | `ctx.Blocked` | `Err(ctx.blocked(...))` |
| decline to continue | `ctx.refused` | `ctx.refused` | `ctx.Refused` | `Err(ctx.refused(...))` |

All four SDKs emit the same `yield.v1` protocol. Choose the language that best
fits the repository containing the skill.

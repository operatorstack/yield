# Create your first skill workflow

Bootstrap installs a tested workflow builder for your coding agent. Run one
command from the repository root.

## 1. Run bootstrap

Choose the command for the project language:

```bash
# TypeScript
npm create @operatorstack/yield@latest

# Python
uvx --from yieldskill yskill bootstrap --language python

# Rust
cargo install yieldskill --root .yield --locked
.yield/bin/yskill bootstrap --root . --language rust

# Go
go run github.com/operatorstack/yield/cmd/yskill@latest bootstrap --root . --language go
```

Bootstrap detects installed Codex, Claude Code, and Cursor project adapters.
Use `--agent codex,claude-code,cursor` to select them explicitly.

## 2. Review the plan

Yield prints every file, dependency, and command that it will change. Confirm
the plan to continue. Use `--dry-run` to stop after the plan. Use `--yes` only
when another trusted process already approved the changes.

Yield writes the builder under `skills/yield-workflow-builder`. It stores local
bootstrap state under ignored `.yield/`. It does not use an install hook.

## 3. Restart the coding agent

Restart the coding-agent session after registration. This lets the agent find
the new adapter.

## 4. Ask for the skill workflow

To create a new skill workflow, ask:

```text
Use Yield to create a tested skill workflow for releasing my package.
```

To convert an existing `SKILL.md`, ask:

```text
Use Yield to convert my existing release SKILL.md into a tested skill workflow.
```

The builder can start from a description. It can also convert an existing
`SKILL.md`. It writes the workflow, runs `doctor --test`, allows two repair
attempts, registers adapters, and verifies them.

The workflow remains under `skills/`. Generated agent adapters contain only
the commands that start and resume it.

## Advanced: build manually

Use [`yskill init`](reference/cli.md#init) when you want to write the program
and fixtures yourself. See the [primitive guides](primitives/README.md) and
[working examples](examples.md).

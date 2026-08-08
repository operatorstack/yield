# Run one skill workflow from your coding agents

The canonical skill workflow belongs beside the application and language
dependencies it uses. `yskill register` creates only the small `SKILL.md`
adapters required for agent discovery.

## Register an existing skill workflow

```bash
# Detect installed verified agents
yskill register skills/review

# Or choose agents explicitly
yskill register skills/review --agent cursor,codex,claude-code

# Check the workflow itself
yskill doctor skills/review --test

# Check selected adapters too
yskill doctor skills/review --agent cursor,codex,claude-code --test

# Register a complete workflow directory in one pass
yskill register-all skills --agent cursor,codex,claude-code --dry-run
yskill register-all skills --agent cursor,codex,claude-code --prune
```

Use the launcher installed by the selected language package:

| Language | Launcher |
|---|---|
| TypeScript | `npm exec -- yskill` |
| Python | `python -m yieldskill` |
| Go | `.yield/bin/yskill` |
| Rust | `.yield/bin/yskill` |

Go and Rust keep one version-locked runtime in `.yield/bin` at the repository
root. Registration checks that runtime, the workflow SDK, and the generated
adapter all use the same Yield version. It refuses a missing or mismatched
runtime and prints the exact repair command. A global `yskill` is not used.
Windows adapters use `.\.yield\bin\yskill.exe`.

Run `yskill agents` to see every supported ID and project directory. Cursor,
Codex, and Claude Code are verified. Other entries use paths from a pinned
snapshot of the open `vercel-labs/skills` registry and are labelled
`registry`: path generation is tested, but the product itself has not been run
end to end by Yield.

Generated adapters are safe to commit. They are disposable discovery files,
not copies of the workflow. Regenerate them after changing the canonical skill
workflow. Yield refuses to overwrite a user-owned skill with the
same name. Names must also be unique across languages because coding agents use
one project-level skill namespace.

## Run the registered skill

Start a new coding-agent session after registration. Where slash skills are
supported, run the generated skill by name:

```text
/review
```

Otherwise, ask the agent to use it:

```text
Use the review skill to check the current branch.
```

The host owns how the request is presented. The generated adapter starts the
canonical workflow under `skills/review`; it does not contain a second copy.

## Set up the workflow builder

Run the native bootstrap command from the repository root:

```bash
# TypeScript
npm create @operatorstack/yield@latest

# Python
uvx --from yieldskill yskill bootstrap --language python

# Rust
cargo install yieldskill --locked
yskill bootstrap --language rust

# Go
go run github.com/operatorstack/yield/cmd/yskill@latest bootstrap --language go
```

Bootstrap shows every proposed change. Confirm the plan. Restart the coding
agent after registration. To create a new skill workflow, ask:

```text
Use Yield to create a tested skill workflow for releasing my package.
```

To convert an existing `SKILL.md`, ask:

```text
Use Yield to convert my existing release SKILL.md into a tested skill workflow.
```

The builder collects the specification, writes the skill workflow, runs its
fixture, allows two repair attempts, registers adapters, and verifies them.

## Questions and agent results

The skill workflow emits a typed operation. The coding agent may show that operation using
its native question UI. Yield does not render the UI. After collecting an
answer, the adapter uses `yskill respond`; it does not create `response.json`.

Use `--value` for a person’s answer and `--result-json` for structured agent
work. The file-based `resume --response` command remains available for CI.

Workflow-only `doctor` works without `.git`. A Go or Rust runtime under
`.yield/bin` also identifies the project root for `init`, `doctor`, and
registration. For other non-Git layouts, pass `--root` so Yield knows where
agent adapters belong.
```

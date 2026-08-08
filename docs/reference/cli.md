# CLI reference

`yskill` runs and resumes skill workflows. It owns run logs, validates responses, executes
commands, and starts the skill workflow. It comes with each language package.

## `bootstrap`

```bash
yskill bootstrap
  [--language typescript|python|rust|go]
  [--agent auto|cursor,codex,claude-code]
  [--root repository]
  [--dry-run]
  [--yes]
```

Detects the repository, language, and installed coding agents. It shows every
proposed change before it writes. It asks for confirmation unless `--yes` is
set. It installs and tests `skills/yield-workflow-builder`, registers the
selected adapters, and verifies them.

Bootstrap stores local state under ignored `.yield/`. It refuses paths outside
the repository, symlink escapes, existing destinations, and user-owned adapter
files. Use `--root` for a directory that is not a Git repository.

Bootstrap can change only these repository locations:

- `.yield/.gitignore` and `.yield/bootstrap.json`
- `.yield/bin/yskill` for Go and Rust
- `skills/yield-workflow-builder/` and its language dependency lockfile
- selected generated agent adapter paths

The TypeScript dependency install also creates ignored `node_modules/` content.
Bootstrap runs only the dependency preparation command shown in the plan,
`doctor --test`, and adapter registration. It does not use install hooks.

## `init`

```bash
yskill init <directory> --description "What it does and when to use it"
  [--language typescript|python|go|rust]
```

Scaffolds a new skill workflow or adds a Yield program beside an existing
prose skill.
Generated dependencies are pinned to the installed `yskill` version.
New skills require a real trigger-oriented description. Existing `SKILL.md`
files are preserved and validated.

## `register`

```bash
yskill register <skill-directory> [--agent cursor,codex,...|auto]
  [--root repository]
```

Writes generated, project-local `SKILL.md` adapters for the selected coding
agents. Without `--agent`, Yield detects verified agents. Explicit IDs work for
every entry printed by `yskill agents`. Workflow code, dependencies, fixtures,
and run state remain in the canonical skill directory.

Registration updates only adapters previously generated from the same source.
It refuses user-owned files, workflows outside the repository, and canonical
workflows stored inside a selected agent's discovery directory.

## `agents`

```bash
yskill agents
```

Lists agent IDs, project skill directories, detection state, and whether each
entry is verified or registry-supported.

## `doctor`

```bash
yskill doctor <skill-directory> [--agent cursor,codex,...|auto]
  [--root repository] [--test]
```

Checks the canonical skill workflow and package launcher. `--test` also runs the
workflow against `fixtures/responses.json` without leaving a run journal.
Adapter checks run only when `--agent` is supplied, and all adapter problems
are reported together.

## `version`

```bash
yskill --version
yskill version
```

Prints the runtime version and platform.

## `run`

```bash
yskill run <skill-directory> [--input input.json]
```

Starts a run and prints the first unanswered operation. The run is stored under
the skill's `.yield/runs/` directory.

## `resume`

```bash
yskill resume <run-id> --response response.json [--skill directory]
```

Validates one response and prints the next operation or terminal outcome.
`--accept-new-digest` explicitly rebinds a saved run after intentional skill
source changes; do not use it to hide accidental drift.

`resume` is the file-based interface for CI and audit tooling. For normal
agent use, prefer `respond`.

## `respond`

```bash
yskill respond <run-id> --value <answer> [--skill directory]
yskill respond <run-id> --result-json '<json>' [--skill directory]
yskill respond <run-id> --result-json - [--skill directory]
```

Reads the pending operation, builds the response envelope, validates the
result, and advances the run as one locked transition. `--value` answers an
`ask_user` operation. `--result-json` supplies a structured agent result; `-`
reads JSON from standard input. Completed results are printed in full.

## `register-all`

```bash
yskill register-all <skills-directory> --agent cursor,codex
  [--root repository] [--dry-run] [--prune]
```

Registers every immediate skill workflow in one directory. It checks all names and
destinations before writing. `--prune` removes only obsolete adapters generated
from that workflow directory. Agent-facing names must be unique.

## `inspect`

```bash
yskill inspect [run-id] [--skill directory]
```

Without a run ID, lists saved runs. With an ID, prints the append-only event
log.

## `replay`

```bash
yskill replay <run-id> [--skill directory]
```

Re-executes the program from the log and verifies that recorded operations lead
to the same frontier. Operation drift fails loudly.

## `test`

```bash
yskill test <skill-directory> [--keep-run]
```

Uses `fixtures/responses.json` for `ask_user` and `agent_task` operations.
`run_command` operations still execute for real. The command succeeds only when
the program reaches `completed`. Test journals are temporary unless
`--keep-run` is supplied. Optional `fixtures/test.json` setup, per-response,
and teardown commands use argv arrays and never run through a shell.

```json
{
  "version": 1,
  "setup": [["node", "fixtures/setup.mjs"]],
  "after_response": {
    "approve": [["node", "fixtures/apply-approval.mjs"]]
  },
  "teardown": [["node", "fixtures/teardown.mjs"]]
}
```

Each `after_response` command receives that fixture response as JSON on
standard input. Hooks run only during `yskill test`.

`setup` runs before the first workflow step. `after_response` runs after the
named fixture response is accepted. `teardown` always runs after success or
failure. Every hook receives `YIELD_FIXTURE=1`. This keeps test-only effects
out of live workflows.

## `prune`

```bash
yskill prune <skill-directory> --older-than 720h
  [--keep-last 10] [--dry-run]
```

Removes old terminal runs. Active runs are never selected.

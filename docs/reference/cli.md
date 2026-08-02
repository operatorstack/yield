# CLI reference

`yskill` runs workflows. It owns run logs, validates responses, executes
commands, and starts the skill program. It comes with each language package.

## `init`

```bash
yskill init <directory> --description "What it does and when to use it"
  [--language typescript|python|go|rust]
```

Scaffolds a new skill or adds a Yield program beside an existing prose skill.
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

Checks the manifest, package launcher, portable metadata, adapter ownership,
source path, and source digest. `--test` also runs the workflow against
`fixtures/responses.json`.

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
yskill test <skill-directory>
```

Uses `fixtures/responses.json` for `ask_user` and `agent_task` operations.
`run_command` operations still execute for real. The command succeeds only when
the program reaches `completed`.

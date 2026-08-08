<p align="center">
  <a href="https://yield.operatorstack.systems/">
    <img src="https://raw.githubusercontent.com/operatorstack/yield/main/assets/yield-mark.svg" width="96" alt="Yield" />
  </a>
</p>

<h1 align="center">Yield for Python</h1>

<p align="center"><strong>Move repeatable coding-agent instructions from words into Python.</strong></p>

<p align="center">
  Build typed, resumable workflows that stay beside the code they operate on.
</p>

<p align="center">
  <a href="https://pypi.org/project/yieldskill/"><img alt="PyPI version" src="https://img.shields.io/pypi/v/yieldskill?style=flat-square" /></a>
  <a href="https://pypi.org/project/yieldskill/"><img alt="Python versions" src="https://img.shields.io/pypi/pyversions/yieldskill?style=flat-square" /></a>
  <a href="https://github.com/operatorstack/yield/actions/workflows/verify.yml"><img alt="Build status" src="https://img.shields.io/github/actions/workflow/status/operatorstack/yield/verify.yml?branch=main&amp;style=flat-square&amp;label=build" /></a>
  <a href="https://github.com/operatorstack/yield/blob/main/LICENSE"><img alt="MIT license" src="https://img.shields.io/pypi/l/yieldskill?style=flat-square" /></a>
</p>

<p align="center">
  <a href="https://yield.operatorstack.systems/">Website</a> ·
  <a href="https://yield.operatorstack.systems/docs/">Documentation</a> ·
  <a href="https://pypi.org/project/yieldskill/">PyPI</a> ·
  <a href="https://github.com/operatorstack/yield">GitHub</a>
</p>

The package name and import name are both `yieldskill`. Python reserves
`yield` as a keyword.

## Start with your coding agent

```bash
uvx --from yieldskill yskill bootstrap --language python
```

Review and confirm the plan. Restart your coding agent. Then ask it to create
or convert a skill workflow.

## Advanced: build manually

### 1. Install Yield

Yield supports Python 3.10 or later on macOS, Linux, and Windows. Create a
virtual environment and install the public package:

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install yieldskill
python -m yieldskill --version
```

On Windows PowerShell:

```powershell
py -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install yieldskill
python -m yieldskill --version
```

Each wheel contains the matching `yskill` runtime for its platform. You do not
need Go, Node.js, or a separate CLI installation.

### 2. Create the workflow

Create a Python workflow inside your repository:

```bash
python -m yieldskill init skills/env-doctor \
  --language python \
  --description "Check the Python environment and explain the required fix."
```

Replace `skills/env-doctor/main.py` with this tested workflow:

<!-- python-example:start -->
```python
from yieldskill import define_skill

def program(ctx):
    probe = ctx.run_command("probe-python", "python3 --version || python --version", timeout_seconds=60)

    diagnosis = ctx.agent_task(
        "diagnose",
        "Given the probe output, is this environment healthy for the project? "
        "If not, state the single most likely fix.",
        context={"exit_code": probe.exit_code, "stdout": probe.stdout, "stderr": probe.stderr},
        schema={
            "type": "object",
            "required": ["healthy"],
            "properties": {
                "healthy": {"type": "boolean"},
                "fix_hint": {"type": "string"},
            },
        },
    )

    if not diagnosis["healthy"]:
        answer = ctx.ask_user(
            "apply-fix",
            f"The environment needs a fix: {diagnosis.get('fix_hint', 'unknown')}. Apply it now and reply done.",
            options=[{"value": "done"}, {"value": "skip"}],
        )
        if answer != "done":
            ctx.blocked("the environment fix was not applied")
        recheck = ctx.run_command("recheck-python", "python3 --version || python --version", timeout_seconds=60)
        ctx.require(recheck.exit_code == 0, "the environment probe passes after the fix", recheck)
        return {"healthy": True, "fixed": True}

    ctx.require(probe.exit_code == 0, "the environment probe passes", probe)
    return {"healthy": True, "fixed": False}


define_skill(program)
```
<!-- python-example:end -->

The generated `skill.json` declares Python as the runner. The generated
`SKILL.md` tells a coding agent how to start and resume the workflow.

### 3. Test the workflow

Use deterministic fixture responses during tests. Save this as
`skills/env-doctor/fixtures/responses.json`:

```json
{
  "diagnose": { "healthy": true }
}
```

Then test the workflow:

```bash
python -m yieldskill doctor skills/env-doctor --test
```

Yield runs commands for real and supplies agent and user responses from the
fixture. A successful test reaches `completed` without leaving a run journal.

### 4. Register the skill

Registration lets installed coding agents discover the workflow:

```bash
python -m yieldskill register skills/env-doctor
```

Select the verified agents explicitly when you do not want automatic
detection:

```bash
python -m yieldskill register skills/env-doctor \
  --agent cursor,codex,claude-code
```

The generated adapters point back to `skills/env-doctor`. They do not copy the
workflow or install its dependencies again.

### 5. Run the skill

Start a new coding-agent session so it discovers the registered skill. Where
slash skills are supported, run:

```text
/env-doctor
```

Otherwise, ask the agent in plain language:

```text
Use the env-doctor skill to check this project.
```

The agent follows the adapter, starts the canonical Python workflow, and asks
for each required agent or user response.

## How Yield runs and resumes

1. Your Python function emits one typed operation.
2. Yield records the request and exits. It does not run a daemon.
3. The coding agent, user, or CLI supplies the result.
4. Yield replays the function from its journal until it reaches the next
   operation.

Replay must produce the same operation sequence. Yield reports divergence
instead of giving a recorded response to a different operation.

| Python primitive | Purpose |
|---|---|
| `ctx.run_command()` | Execute a command and record its exit code and output. |
| `ctx.agent_task()` | Ask the coding agent for schema-valid JSON. |
| `ctx.ask_user()` | Request an explicit human decision. |
| `ctx.require()` | Bind a required claim to recorded evidence. |
| `ctx.blocked()` / `ctx.refused()` | Stop honestly when work cannot or must not continue. |

See the [primitive guides](https://yield.operatorstack.systems/docs/primitives/)
and [CLI reference](https://github.com/operatorstack/yield/blob/main/docs/reference/cli.md)
for the complete contract.

## Guarantees and limits

Yield provides deterministic control flow, typed requests and responses,
persistent run state, replay with divergence detection, stale and duplicate
response rejection, and evidence-bound completion.

Schema validity is not truth. Yield cannot prove that a coding agent performed
only the requested work. `run_command` is different: the Yield CLI executes the
command, so its recorded exit code and output are observed facts.

Programs must remain deterministic between operations. Do not read clocks,
random values, environment variables, or changing files to choose the next
operation. Cross those boundaries through a Yield operation instead.

Yield is not a daemon, hosted runtime, workflow DSL, marketplace, coding-agent
loop, multi-agent orchestrator, or security sandbox.

## Coding agents and source

Cursor, Codex, and Claude Code are verified integrations. Yield also provides
registry-backed project paths for other coding agents; those paths are not
presented as end-to-end verified.

- [Read the documentation](https://yield.operatorstack.systems/docs/)
- [Explore tested examples](https://github.com/operatorstack/yield/tree/main/examples)
- [View the Python source](https://github.com/operatorstack/yield/tree/main/sdk/python)
- [Report an issue](https://github.com/operatorstack/yield/issues)

Yield is available under the
[MIT license](https://github.com/operatorstack/yield/blob/main/LICENSE).

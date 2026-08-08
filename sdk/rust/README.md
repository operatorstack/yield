<p align="center">
  <a href="https://yield.operatorstack.systems/">
    <img src="https://raw.githubusercontent.com/operatorstack/yield/main/assets/yield-mark.svg" width="96" alt="Yield" />
  </a>
</p>

<h1 align="center">Yield for Rust</h1>

<p align="center"><strong>Move repeatable coding-agent instructions from words into Rust.</strong></p>

<p align="center">
  Build typed, resumable workflows that stay beside the code they operate on.
</p>

<p align="center">
  <a href="https://crates.io/crates/yieldskill"><img alt="crates.io version" src="https://img.shields.io/crates/v/yieldskill?style=flat-square" /></a>
  <a href="https://docs.rs/yieldskill"><img alt="docs.rs" src="https://img.shields.io/docsrs/yieldskill?style=flat-square" /></a>
  <a href="https://github.com/operatorstack/yield/actions/workflows/verify.yml"><img alt="Build status" src="https://img.shields.io/github/actions/workflow/status/operatorstack/yield/verify.yml?branch=main&amp;style=flat-square&amp;label=build" /></a>
  <a href="https://github.com/operatorstack/yield/blob/main/LICENSE"><img alt="MIT license" src="https://img.shields.io/crates/l/yieldskill?style=flat-square" /></a>
</p>

<p align="center">
  <a href="https://yield.operatorstack.systems/">Website</a> ·
  <a href="https://yield.operatorstack.systems/docs/">Documentation</a> ·
  <a href="https://crates.io/crates/yieldskill">crates.io</a> ·
  <a href="https://docs.rs/yieldskill">docs.rs</a> ·
  <a href="https://github.com/operatorstack/yield">GitHub</a>
</p>

The crate and library names are both `yieldskill`. The installed command is
`yskill`.

## Start with your coding agent

```bash
cargo install yieldskill --locked
yskill bootstrap --language rust
```

Review and confirm the plan. Restart your coding agent. Then ask it to create
a new skill workflow:

```text
Use Yield to create a tested skill workflow for releasing my package.
```

To convert an existing `SKILL.md`, ask:

```text
Use Yield to convert my existing release SKILL.md into a tested skill workflow.
```

## Advanced: build manually

### 1. Install Yield

Yield supports Rust on macOS, Linux, and Windows. Install the public crate:

```bash
cargo install yieldskill --locked
yskill --version
```

The crate contains the matching `yskill` runtime for your platform. You do not
need Go, Node.js, or a separate CLI download.

### 2. Create the workflow

Create a Rust workflow inside your repository:

```bash
yskill init skills/data-migration \
  --language rust \
  --description "Dry-run, approve, apply, and verify a database migration."
```

Replace `skills/data-migration/src/main.rs` with this tested workflow:

<!-- rust-example:start -->
```rust
use serde_json::json;
use yieldskill::{define_skill, Context, SkillResult};

fn program(ctx: &mut Context) -> SkillResult {
    let dry_run = ctx.run_command("dry-run", "echo 'plan: add index users_email_idx'", 300);
    ctx.require(
        dry_run.exit_code == 0,
        "the dry run succeeds",
        Some(&json!({ "exit_code": dry_run.exit_code })),
    );

    let review = ctx.agent_task(
        "summarize-plan",
        "Summarize the migration plan for the operator: what changes, what is irreversible, what the rollback is.",
        Some(json!({ "plan": dry_run.stdout })),
        Some(json!({
            "type": "object",
            "required": ["summary"],
            "properties": { "summary": { "type": "string", "minLength": 1 } }
        })),
    );

    let approval = ctx.ask_user(
        "approve-apply",
        "Apply the migration to the live database?",
        &[("apply", "Apply now"), ("abort", "Abort")],
    );
    if approval != "apply" {
        return Err(ctx.refused("the operator declined to apply the migration"));
    }

    let apply = ctx.run_command("apply", "echo apply-ok", 600);
    ctx.require(
        apply.exit_code == 0,
        "the migration applies cleanly",
        Some(&json!({ "exit_code": apply.exit_code })),
    );

    let verify = ctx.run_command("verify", "echo verify-ok", 300);
    ctx.require(
        verify.exit_code == 0,
        "post-apply verification passes",
        Some(&json!({ "exit_code": verify.exit_code })),
    );

    Ok(json!({
        "applied": true,
        "summary": review["summary"],
    }))
}

fn main() {
    define_skill(program);
}
```
<!-- rust-example:end -->

The generated `Cargo.toml` pins the public `yieldskill` crate to the installed
CLI version. The generated `skill.json` runs the named Rust binary.

### 3. Test the workflow

Use deterministic fixture responses during tests. Save this as
`skills/data-migration/fixtures/responses.json`:

<!-- rust-fixture:start -->
```json
{
  "summarize-plan": {
    "summary": "Adds users_email_idx concurrently; no data rewrite; rollback is DROP INDEX. The only irreversible step is index creation time."
  },
  "approve-apply": { "value": "apply" }
}
```
<!-- rust-fixture:end -->

Then test the workflow:

```bash
yskill doctor skills/data-migration --test
```

Yield runs commands for real and supplies agent and user responses from the
fixture. A successful test reaches `completed` without leaving a run journal.

### 4. Register the skill

Registration lets installed coding agents discover the workflow:

```bash
yskill register skills/data-migration
```

Select the verified agents explicitly when you do not want automatic
detection:

```bash
yskill register skills/data-migration \
  --agent cursor,codex,claude-code
```

The generated adapters point back to `skills/data-migration`. They do not copy
the workflow or install its dependencies again.

### 5. Run the skill

Start a new coding-agent session so it discovers the registered skill. Where
slash skills are supported, run:

```text
/data-migration
```

Otherwise, ask the agent in plain language:

```text
Use the data-migration skill to apply this migration safely.
```

The agent follows the adapter, starts the canonical Rust workflow, and asks for
each required agent or user response.

## How Yield runs and resumes

1. Your Rust function emits one typed operation.
2. Yield records the request and exits. It does not run a daemon.
3. The coding agent, user, or CLI supplies the result.
4. Yield replays the function from its journal until it reaches the next
   operation.

Replay must produce the same operation sequence. Yield reports divergence
instead of giving a recorded response to a different operation.

| Rust primitive | Purpose |
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
- [View the Rust source](https://github.com/operatorstack/yield/tree/main/sdk/rust)
- [Read the API documentation](https://docs.rs/yieldskill)
- [Report an issue](https://github.com/operatorstack/yield/issues)

Yield is available under the
[MIT license](https://github.com/operatorstack/yield/blob/main/LICENSE).

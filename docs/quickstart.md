# Create your first skill workflow

Start with the runtime and manual workflow. Package installation alone never
creates a skill or coding-agent adapter.

## 1. Install Yield

Choose the package for your project:

```bash
# TypeScript
npm install --save-exact @operatorstack/yield

# Python
python -m pip install yieldskill

# Rust
cargo install yieldskill --root .yield --locked

# Go
mkdir -p .yield/bin
GOBIN="$PWD/.yield/bin" go install github.com/operatorstack/yield/cmd/yskill@latest
```

Use `npm exec -- yskill`, `python -m yieldskill`, or `.yield/bin/yskill` as the
launcher in the following steps. The examples below use TypeScript; substitute
your launcher and language when using another SDK.

## 2. Create the workflow and fixture

```bash
npm exec -- yskill init skills/release \
  --language typescript \
  --description "Test, review, approve, publish, and verify a package."
```

Edit the generated program under `skills/release/`. Keep deterministic command
execution, approval, gates, and finish rules in code. Put fixture answers for
agent and user operations in `skills/release/fixtures/responses.json`.

## 3. Test it

```bash
npm exec -- yskill doctor skills/release --test
```

This runs the fixture to a terminal outcome without leaving a run journal.

## 4. Register it

```bash
npm exec -- yskill register skills/release
npm exec -- yskill doctor skills/release --agent codex,cursor,claude-code --test
```

Registration creates only small discovery adapters. The canonical workflow,
dependencies, and fixtures remain under `skills/release/`. Restart the coding
agent after registration.

## 5. Run it

Ask the coding agent to use the registered skill:

```text
Use the release skill to publish this package.
```

## Optional: install the developer helper

After learning the manual flow, install guided assistance explicitly:

```bash
# TypeScript
npm exec -- yskill helper install --language typescript

# Python
python -m yieldskill helper install --language python

# Rust
.yield/bin/yskill helper install --root . --language rust

# Go
go run github.com/operatorstack/yield/cmd/yskill@latest helper install --root . --language go
```

Review the printed files and commands, approve the plan, then restart the
coding agent. The optional `yield-workflow-builder` can teach the primitives
and guide create, convert, check, repair, upgrade, and register operations.

`yskill bootstrap` and `npm create @operatorstack/yield@latest` remain
compatibility aliases.

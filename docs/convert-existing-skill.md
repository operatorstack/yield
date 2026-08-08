# Convert an existing skill

The workflow builder can convert an existing `SKILL.md` into a tested skill
workflow. The source and destination must stay inside the repository.

## 1. Install the builder

Run the bootstrap command for the project language. For example:

```bash
npm create @operatorstack/yield@latest
```

See the [quickstart](quickstart.md) for Python, Rust, and Go commands.

## 2. Restart the coding agent

Restart the session after bootstrap registers the adapter.

## 3. Request the conversion

Tell the agent which skill to convert and what must remain true:

```text
Use Yield to convert skills/release/SKILL.md into a tested skill workflow.
Keep the approval before publish. Require registry verification before completion.
```

The builder reads the source, extracts its control flow, and writes a new
destination. It refuses an existing destination. It never overwrites the
source.

The builder runs the generated fixture. It allows two repair attempts. It then
registers and verifies the selected coding-agent adapters. It reports success
only after these checks pass.

Review the generated program. A passing fixture proves the tested path. It
does not prove that every sentence in the prose skill has the same behavior.

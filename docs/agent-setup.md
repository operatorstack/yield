# Use one Yield workflow from your coding agents

Yield workflows belong beside the application and language dependencies they
use. `yskill register` creates only the small `SKILL.md` adapters required for
agent discovery.

## Register an existing workflow

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
| Go | `yskill` |
| Rust | `yskill` |

Run `yskill agents` to see every supported ID and project directory. Cursor,
Codex, and Claude Code are verified. Other entries use paths from a pinned
snapshot of the open `vercel-labs/skills` registry and are labelled
`registry`: path generation is tested, but the product itself has not been run
end to end by Yield.

Generated adapters are safe to commit. Regenerate them after changing the
canonical workflow. Yield refuses to overwrite a user-owned skill with the
same name. Names must also be unique across languages because coding agents use
one project-level skill namespace.

## Copy this to your agent

Replace the bracketed values, then paste this into the coding agent already
open in the project:

```text
Set up a Yield workflow named [skill-name] in skills/[skill-name].

1. Detect whether this project uses TypeScript, Python, Go, or Rust.
2. Install that language's Yield package using the project's existing package
   manager. Do not install a second global runtime.
3. Run yskill init with the detected language and this description:
   [what the workflow does and when it should run]
4. Replace the starter program and fixture with the requested workflow. The
   starter is intentionally blocked and must not pass tests unchanged.
5. Run yskill doctor with --test before registration.
6. Keep the workflow beside the project's language dependencies.
7. Run yskill register for the coding agent you are currently using.
8. Use the launcher from the installed language package for every yskill
   command: npm exec -- yskill, python -m yieldskill, or yskill.
9. Run yskill doctor with --agent and --test.
10. Report the commands, generated adapter path, and every changed file.

Do not move the workflow into an agent discovery directory and do not copy its
dependencies into an adapter.

## Questions and agent results

Yield emits a typed operation. The coding agent may show that operation using
its native question UI. Yield does not render the UI. After collecting an
answer, the adapter uses `yskill respond`; it does not create `response.json`.

Use `--value` for a person’s answer and `--result-json` for structured agent
work. The file-based `resume --response` command remains available for CI.

Workflow-only `doctor` works without `.git`. For registration in such a
directory, pass `--root` so Yield knows where agent adapters belong.
```

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

# Check the package and adapters; add --test to run fixture responses
yskill doctor skills/review --agent cursor,codex,claude-code --test
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
same name.

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
4. Keep the workflow beside the project's language dependencies.
5. Run yskill register for the coding agent you are currently using.
6. Use the launcher from the installed language package for every yskill
   command: npm exec -- yskill, python -m yieldskill, or yskill.
7. Run yskill doctor with --test.
8. Report the commands, generated adapter path, and every changed file.

Do not move the workflow into an agent discovery directory and do not copy its
dependencies into an adapter.
```

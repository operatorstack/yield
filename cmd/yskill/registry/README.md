# Agent path registry

`agents.json` is a Go-embedded snapshot of project skill directories from
[`vercel-labs/skills`](https://github.com/vercel-labs/skills), pinned to the
commit recorded in the file. That project is MIT licensed.

Yield overrides the verified Cursor, Codex, and Claude Code project paths with
the paths exercised by its integration fixtures. Other entries provide broad,
explicit path registration and are labelled `registry`, not end-to-end
verified.

The pinned upstream license is included in `VERCEL_SKILLS_LICENSE`.

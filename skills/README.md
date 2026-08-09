# Project skills

This directory contains the canonical Yield workflows shipped with this
repository. A skill is executable workflow source, not a copied prompt.
Generated Codex, Cursor, and Claude Code adapters point back here.

Run a skill from the repository root:

```sh
npm exec -- yskill run skills/<skill-name>
```

Use `yskill doctor skills/<skill-name>` to check a skill, and
`yskill register skills/<skill-name> --agent cursor,codex,claude-code` to make
it discoverable by coding agents. Add `--test` only when the workflow supplies
safe fixture responses.

## Included workflows

- [`release-yield`](release-yield/) guides a protected full-train Yield
  release. It asks for release choices, verifies the exact plan, and leaves
  registry credentials and publication to GitHub's protected workflows. Its
  safe contract tests run with `npm run test:selfhost`; do not fixture-run a
  workflow that can dispatch a protected release.

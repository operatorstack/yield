# Use one workflow from several coding agents

- Add `yskill register` to create small adapters for coding-agent skill directories.
- Add `yskill agents` to show supported agents, paths, detection state, and verification level.
- Add `yskill doctor` to check a workflow, its launcher, and generated adapters; `--test` also runs its fixture.
- Keep workflow code and dependencies in one canonical directory instead of copying them per agent.
- Verify Cursor, Codex, and Claude paths end to end, while keeping other registry entries explicitly selectable.
- Require a useful description when scaffolding a new workflow and print the exact register, doctor, and test commands.

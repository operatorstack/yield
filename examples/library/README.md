# Skill workflow library

Ten common skill workflows for coding agents, each implemented in TypeScript, Python,
Go, and Rust. Choose the language already used by your repository; the
workflow and fixture are otherwise the same.

| Skill workflow      | What the code keeps in order                                         |
| ------------------- | -------------------------------------------------------------------- |
| review-branch       | checks -> review -> zero-critical gate                               |
| investigate-failure | evidence -> diagnosis -> supported cause                             |
| qa-web-change       | build -> changed-route QA -> no blockers                             |
| release-package     | tests -> review -> approval -> publish -> verify                     |
| triage-issue        | read -> classify -> one next action                                  |
| repair-ci           | failed log -> supported repair -> rerun                              |
| upgrade-dependency  | baseline -> compatibility review -> approval -> update -> tests      |
| migrate-database    | dry-run -> risk review -> approval -> apply -> verify                |
| audit-security      | mechanical scans -> trust-boundary review -> zero-critical gate      |
| publish-ios         | archive -> metadata review -> approval -> upload -> processing check |

The source files live under:

    typescript/src/<workflow>.ts
    python/src/<workflow>.py
    go/src/<workflow>/main.go
    rust/src/bin/<workflow>.rs

Each language also has a runnable skill directory at
language/workflow/ containing a thin SKILL.md, runner manifest, and
scripted fixture. From the repository root:

    go build -o /tmp/yskill ./cmd/yskill
    YSKILL=/tmp/yskill bash ./examples/library/test-all.sh

Or run one:

    /tmp/yskill test examples/library/typescript/review-branch
    /tmp/yskill test examples/library/python/review-branch
    /tmp/yskill test examples/library/go/review-branch
    /tmp/yskill test examples/library/rust/review-branch

The shell commands deliberately produce harmless fixture evidence. Replace
them with the real commands from your repository before adopting a skill workflow.
The examples are independent implementations of recurring skill categories;
they do not copy another project's prompts or claim compatibility with them.

Regenerate the checked-in matrix after editing the catalog:

    node examples/library/scripts/generate.mjs

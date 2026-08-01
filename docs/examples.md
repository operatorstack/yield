# Example library

The library contains ten common coding-agent workflows. Every workflow is
implemented in TypeScript, Python, Go, and Rust, so the first choice is your
repository's language—not which example happens to exist.

| Workflow | Control flow moved into code |
|---|---|
| [Review a branch](../examples/library/typescript/review-branch/) | checks, review, zero-critical gate |
| [Investigate a failure](../examples/library/typescript/investigate-failure/) | evidence, diagnosis, supported cause |
| [QA a web change](../examples/library/typescript/qa-web-change/) | build, changed-route QA, no-blocker gate |
| [Release a package](../examples/library/typescript/release-package/) | tests, review, approval, publish, verify |
| [Triage an issue](../examples/library/typescript/triage-issue/) | read, classify, one next action |
| [Repair CI](../examples/library/typescript/repair-ci/) | failed log, supported repair, rerun |
| [Upgrade a dependency](../examples/library/typescript/upgrade-dependency/) | baseline, compatibility review, approval, update, tests |
| [Run a database migration](../examples/library/typescript/migrate-database/) | dry-run, risk review, approval, apply, verify |
| [Audit security](../examples/library/typescript/audit-security/) | mechanical scans, trust-boundary review, zero-critical gate |
| [Publish an iOS build](../examples/library/typescript/publish-ios/) | archive, metadata review, approval, upload, processing check |

Change the language segment in any link to python, go, or rust. Source files
are grouped separately for fast browsing:

- [TypeScript](../examples/library/typescript/src/)
- [Python](../examples/library/python/src/)
- [Go](../examples/library/go/src/)
- [Rust](../examples/library/rust/src/bin/)

Run all forty fixtures:

    go build -o /tmp/yskill ./cmd/yskill
    YSKILL=/tmp/yskill bash ./examples/library/test-all.sh

The included commands produce harmless evidence so the examples run in this
repository. Replace them with project commands before adopting a workflow.

## Complete walkthroughs

These examples show longer programs with a thin `SKILL.md` and scripted
responses under `fixtures/responses.json`.

| Example | Language | Pattern |
|---|---|---|
| [`release-checklist`](../examples/release-checklist/) | TypeScript | approval, build, model-authored notes, publish, verify |
| [`env-doctor`](../examples/env-doctor/) | Python | probe, diagnose, wait for a person, recheck |
| [`investigate`](../examples/investigate/) | Go | structured hypotheses, real probes, bounded attempts |
| [`data-migration`](../examples/data-migration/) | Rust | dry-run, approval, apply, verify |
| [`convert-skill`](../examples/convert-skill/) | Go | extract a prose workflow, generate code, execute its fixtures |

From the repository root:

```bash
go build -o /tmp/yskill ./cmd/yskill
/tmp/yskill test examples/release-checklist
/tmp/yskill test examples/env-doctor
/tmp/yskill test examples/investigate
/tmp/yskill test examples/data-migration
YSKILL=/tmp/yskill /tmp/yskill test examples/convert-skill
```

When adapting an example, change the repository-specific commands and model
instructions. Keep stable operation IDs for existing steps so saved runs can
replay them.

# Skill workflows

A **skill workflow** is a portable, executable process that combines agent
skills with deterministic code, state, and verification.

The terms have separate jobs:

| term | meaning |
|---|---|
| **skill** | one reusable capability, described for the coding agent |
| **workflow** | sequencing, branching, checks, and saved state |
| **skill workflow** | an executable composition of skills, code, commands, and human input |
| **adapter** | a generated `SKILL.md` that lets one coding agent discover the workflow |

## The two slices

The canonical skill workflow is the source you edit. It stays under `skills/`
beside its language dependencies, fixtures, and saved runs.

The agent adapter is generated. It contains only the description and the
package-correct commands needed to start or resume the canonical workflow.
Delete and regenerate it when the host changes; do not copy workflow code into
it.

```text
coding agent
  -> generated adapter
  -> canonical skill workflow
  -> agent task, command, or human question
  -> saved response
  -> next step or completed, blocked, or refused
```

## What belongs where

Keep goals, examples, tool guidance, and judgment with the skill and coding
agent. Put order, branches, retry limits, approval points, required evidence,
and finish rules in normal code.

Yield does not replace skills. It gives repeatable skill behavior an
executable boundary that can be tested, paused, resumed, and exposed to more
than one coding agent.

## Build one with a coding agent

`yskill bootstrap` installs the `yield-workflow-builder` skill workflow. The
builder accepts a description or an existing `SKILL.md`. It extracts the
control flow, writes one language implementation, runs its fixture, repairs at
most twice, and verifies the generated adapters. It refuses success when any
verification step is missing.

Next: [create your first skill workflow](quickstart.md) or [register an
existing one](agent-setup.md).

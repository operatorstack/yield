# Trail of Bits actions auditor — Yield conversion

This is an independent, measured conversion of
[Trail of Bits' agentic actions auditor](https://github.com/trailofbits/skills/blob/1256982d4d925a0acfe11e26c2253c32052c6247/plugins/agentic-actions-auditor/skills/agentic-actions-auditor/SKILL.md).
It is not a Trail of Bits artifact.

We used Yield's [`convert-skill`](../../../examples/convert-skill/) workflow to
split security judgment from repeatable control flow, then reviewed the
checked-in result and measured it against the pinned original.

## The converted version

- [`SKILL.md`](SKILL.md) keeps attack-path analysis and finding quality.
- [`workflow.ts`](workflow.ts) owns discovery, coverage, evidence requirements,
  report generation, and completion.

## Result

| Original | Thin skill | Yield program | Maintained change |
|---:|---:|---:|---:|
| 4,846 tokens | 132 tokens | 222 tokens | **−92.7%** |

## Benefits in this conversion

- Workflow files are discovered by a real command instead of inferred.
- Completion requires every discovered workflow to be reviewed.
- Every finding must carry concrete evidence.
- Report generation is an observed command with a checked exit code.

## Claim boundary

This result measures source size, not audit coverage or behavioral equivalence.
See the [evaluation methodology](../../README.md) for the evidence required
before making a behavior claim.

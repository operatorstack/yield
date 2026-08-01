# Systematic debugging — Yield conversion

This is an independent, measured conversion of
[Superpowers' systematic debugging skill](https://github.com/obra/superpowers/blob/44c9b2d6e889982ac18c27d05a19fefe335194e1/skills/systematic-debugging/SKILL.md).
It is not an upstream Superpowers artifact.

We used Yield's [`convert-skill`](../../../examples/convert-skill/) workflow to
split diagnostic judgment from repeatable control flow, then reviewed the
checked-in result and measured it against the pinned original.

## The converted version

- [`SKILL.md`](SKILL.md) keeps the standard for a falsifiable root-cause hypothesis.
- [`workflow.ts`](workflow.ts) owns reproduction, experiment bounds, approval,
  verification, and completion.

## Result

| Original | Thin skill | Yield program | Maintained change |
|---:|---:|---:|---:|
| 2,226 tokens | 137 tokens | 226 tokens | **−83.7%** |

## Benefits in this conversion

- The failure must reproduce before diagnosis starts.
- A proposed cause must include an executable falsification experiment.
- Applying a fix requires user approval.
- Completion requires the full test suite to pass.

## Claim boundary

This result measures source size, not diagnosis quality or behavioral
equivalence. See the [evaluation methodology](../../README.md) for the evidence
required before making a behavior claim.

# Anthropic doc co-authoring — Yield conversion

This is an independent, measured conversion of
[Anthropic's doc co-authoring skill](https://github.com/anthropics/skills/blob/b29e7cf65e5cb78a5ac33d582270551bc74a14eb/skills/doc-coauthoring/SKILL.md).
It is not an Anthropic artifact.

We used Yield's [`convert-skill`](../../../examples/convert-skill/) workflow to
split model judgment from repeatable control flow, then reviewed the checked-in
result and measured it against the pinned original.

## The converted version

- [`SKILL.md`](SKILL.md) keeps writing judgment, reader perspective, and voice.
- [`workflow.ts`](workflow.ts) owns the questions, outline approval, draft stages,
  reader test, saved answers, and completion gate.

## Result

| Original | Thin skill | Yield program | Maintained change |
|---:|---:|---:|---:|
| 3,289 tokens | 137 tokens | 209 tokens | **−89.5%** |

## Benefits in this conversion

- A returning run can continue from saved audience, outcome, and source context.
- Outline approval is an explicit gate instead of a prose suggestion.
- Reader testing happens before the final revision.
- The model spends its context on the document, not on remembering stage order.

## Claim boundary

This result measures source size, not writing quality or behavioral equivalence.
See the [evaluation methodology](../../README.md) for the evidence required
before making a behavior claim.

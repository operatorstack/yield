# GStack review — Yield conversion

This is an independent, measured conversion of
[GStack's review skill](https://github.com/garrytan/gstack/blob/a3259400a366593e0c909dd9ac3e59752efd2488/review/SKILL.md).
It is not an upstream GStack artifact.

We used Yield's [`convert-skill`](../../../examples/convert-skill/) workflow to
split model judgment from repeatable control flow, then reviewed the checked-in
result and measured it against the pinned original.

## The converted version

- [`SKILL.md`](SKILL.md) keeps review judgment, severity rules, and finding shape.
- [`workflow.ts`](workflow.ts) owns the diff, tests, typecheck, critical-finding
  gate, saved result, and completion.

## Result

| Original | Thin skill | Yield program | Maintained change |
|---:|---:|---:|---:|
| 27,162 tokens | 146 tokens | 160 tokens | **−98.9%** |

## Benefits in this conversion

- Repository checks run as commands instead of instructions the model must remember.
- A review cannot complete while a critical finding remains.
- The diff and check output become explicit evidence passed into the review.
- The model-facing prompt is small enough to focus on finding real defects.

## Claim boundary

The token result measures source size. A separate early GStack behavior study is
summarized in [`results/latest.json`](../../results/latest.json), but its raw
artifact is not yet published and it does not prove general equivalence.

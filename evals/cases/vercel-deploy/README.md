# Vercel deploy — Yield conversion

This is an independent, measured conversion of
[Vercel's deploy skill](https://github.com/vercel-labs/agent-skills/blob/7c180d9044c9ae2b442b567aad4e42a28dd5ed62/skills/deploy-to-vercel/SKILL.md).
It is not a Vercel artifact.

We used Yield's [`convert-skill`](../../../examples/convert-skill/) workflow to
split deployment explanation from repeatable control flow, then reviewed the
checked-in result and measured it against the pinned original.

## The converted version

- [`SKILL.md`](SKILL.md) keeps interpretation of project state and failures.
- [`workflow.ts`](workflow.ts) owns state detection, approval, authentication,
  linking, deployment, HTTPS verification, and completion.

## Result

| Original | Thin skill | Yield program | Maintained change |
|---:|---:|---:|---:|
| 2,898 tokens | 108 tokens | 290 tokens | **−86.3%** |

## Benefits in this conversion

- The deployment command runs only after explicit approval.
- Missing authentication produces an honest blocked outcome.
- A successful command is not enough; the deployed URL must answer over HTTPS.
- Cancellation and failure are recorded separately from success.

## Claim boundary

This result measures source size, not deployment reliability or behavioral
equivalence. See the [evaluation methodology](../../README.md) for the evidence
required before making a behavior claim.

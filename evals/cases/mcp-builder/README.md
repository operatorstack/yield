# Microsoft MCP builder — Yield conversion

This is an independent, measured conversion of
[Microsoft's MCP builder skill](https://github.com/microsoft/skills/blob/4a2873faffc1b101a33a0b59c24713d4ed78142f/.github/skills/mcp-builder/SKILL.md).
It is not a Microsoft artifact.

We used Yield's [`convert-skill`](../../../examples/convert-skill/) workflow to
split tool-design judgment from repeatable control flow, then reviewed the
checked-in result and measured it against the pinned original.

## The converted version

- [`SKILL.md`](SKILL.md) keeps tool-design and review judgment.
- [`workflow.ts`](workflow.ts) owns user inputs, the tool-count gate, design
  approval, scaffold, tests, evaluations, and completion.

## Result

| Original | Thin skill | Yield program | Maintained change |
|---:|---:|---:|---:|
| 2,648 tokens | 131 tokens | 291 tokens | **−84.1%** |

## Benefits in this conversion

- The tool surface stays bounded before code generation begins.
- Building requires explicit design approval.
- Scaffold, tests, review, and evaluations happen in a fixed order.
- Completion requires both review and evaluation gates to pass.

## Claim boundary

This result measures source size, not MCP server quality or behavioral
equivalence. See the [evaluation methodology](../../README.md) for the evidence
required before making a behavior claim.

# Conversion cases

These are measured rewrites, not automatic equivalence claims. Each case keeps
the model-facing judgment in a short `SKILL.md` and moves repeatable control
flow into a TypeScript Yield program. The pinned original remains in its source
repository and is identified by commit plus SHA-256 in `index.json`.

| Case | Thin skill | Yield program | Pinned original |
|---|---|---|---|
| GStack review | [SKILL.md](gstack-review/SKILL.md) | [workflow.ts](gstack-review/workflow.ts) | [source](https://github.com/garrytan/gstack/blob/a3259400a366593e0c909dd9ac3e59752efd2488/review/SKILL.md) |
| Anthropic doc co-authoring | [SKILL.md](doc-coauthoring/SKILL.md) | [workflow.ts](doc-coauthoring/workflow.ts) | [source](https://github.com/anthropics/skills/blob/b29e7cf65e5cb78a5ac33d582270551bc74a14eb/skills/doc-coauthoring/SKILL.md) |
| Superpowers systematic debugging | [SKILL.md](systematic-debugging/SKILL.md) | [workflow.ts](systematic-debugging/workflow.ts) | [source](https://github.com/obra/superpowers/blob/44c9b2d6e889982ac18c27d05a19fefe335194e1/skills/systematic-debugging/SKILL.md) |
| Vercel deploy | [SKILL.md](vercel-deploy/SKILL.md) | [workflow.ts](vercel-deploy/workflow.ts) | [source](https://github.com/vercel-labs/agent-skills/blob/7c180d9044c9ae2b442b567aad4e42a28dd5ed62/skills/deploy-to-vercel/SKILL.md) |
| Microsoft MCP builder | [SKILL.md](mcp-builder/SKILL.md) | [workflow.ts](mcp-builder/workflow.ts) | [source](https://github.com/microsoft/skills/blob/4a2873faffc1b101a33a0b59c24713d4ed78142f/.github/skills/mcp-builder/SKILL.md) |
| Trail of Bits actions auditor | [SKILL.md](actions-auditor/SKILL.md) | [workflow.ts](actions-auditor/workflow.ts) | [source](https://github.com/trailofbits/skills/blob/1256982d4d925a0acfe11e26c2253c32052c6247/plugins/agentic-actions-auditor/skills/agentic-actions-auditor/SKILL.md) |

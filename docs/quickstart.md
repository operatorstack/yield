# Create your first skill workflow

This tutorial turns a repeated review checklist into a small TypeScript
program: run a real check, ask the coding agent to review the branch, stop on
critical findings, and save the structured result.

You need Node.js 24 or newer.

## 1. Install Yield

```bash
mkdir yield-example
cd yield-example
npm init -y
npm install --save-exact @operatorstack/yield@0.1.29 \
  --registry=https://get.operatorstack.systems/npm/
npm exec -- yskill --version
```

The package includes the TypeScript SDK and its matching Yield runtime.

## 2. Initialize the skill workflow

```bash
npm exec -- yskill init skills/review \
  --language typescript \
  --description "Check and review the current branch before it is shipped."
```

The canonical workflow stays under `skills/review`, inside the same dependency tree as
`@operatorstack/yield`. The generated `skill.json` records the language and
program entry point:

```json
{"version":1,"language":"typescript","run":["node","main.ts"]}
```

The starter is intentionally incomplete. It cannot pass `doctor --test` until
you replace its program and fixture with the behavior described by the skill.

## 3. Implement the workflow and fixture

Replace `skills/review/main.ts`:

```ts
import { defineSkill } from "@operatorstack/yield";

type Review = {
  critical: number;
  summary: string;
};

defineSkill((ctx) => {
  const check = ctx.runCommand("check", "npm run check", 60);
  ctx.require(check.exit_code === 0, "the code check passes", check);

  const review = ctx.agentTask<Review>(
    "review",
    "Review the current branch. Find correctness, security, and data-loss risks.",
    undefined,
    {
      type: "object",
      required: ["critical", "summary"],
      properties: {
        critical: { type: "number" },
        summary: { type: "string" },
      },
    },
  );

  ctx.require(review.critical === 0, "no critical findings remain", review);
  return review;
});
```

Add the real project check to the root `package.json`:

```json
{"scripts":{"check":"node --check skills/review/main.ts"}}
```

The generated `skills/review/SKILL.md` remains short. It tells the agent when
to use the workflow and how to follow the yielded operations; the program owns
the order and finish rule.

## 4. Test the skill workflow

Replace `skills/review/fixtures/responses.json`:

```json
{
  "review": {
    "critical": 0,
    "summary": "No critical findings in the fixture run."
  }
}
```

Run the workflow check:

```bash
npm exec -- yskill doctor skills/review --test
```

`run_command` operations execute for real. The fixture supplies only model and
user responses. A successful result ends with `reached completed` and a doctor
summary.

## 5. Generate coding-agent adapters

```bash
# Detect installed verified agents
npm exec -- yskill register skills/review

# Or select them explicitly
npm exec -- yskill register skills/review \
  --agent cursor,codex,claude-code
```

Yield keeps one canonical skill workflow and writes only generated adapters:

```text
.cursor/skills/review/SKILL.md   # Cursor
.agents/skills/review/SKILL.md   # Codex
.claude/skills/review/SKILL.md   # Claude Code
```

Start a new agent session after registration, then invoke `/review` or ask for
the task described by the skill.

Check the generated adapters:

```bash
npm exec -- yskill doctor skills/review \
  --agent cursor,codex,claude-code
```

## Run an existing workflow

Initialization is only for creating or wrapping a workflow. For an existing
workflow, install the matching language package, register it for the agents in
the project, then run it directly when needed:

```bash
npm exec -- yskill register skills/review --agent cursor
npm exec -- yskill run skills/review
```

The agent reads the generated adapter, starts the canonical skill workflow, performs
each yielded operation, and answers with `yskill respond`. If the session
closes, the run remains on disk.

Next: [understand skill workflows](skill-workflows.md), [set up coding
agents](agent-setup.md), [understand each
primitive](primitives/README.md), or follow the [complete review
tutorial](tutorials/code-review.md).

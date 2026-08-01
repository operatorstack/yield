# Run your first Yield skill

This tutorial turns a repeated review checklist into a small TypeScript
program:

1. run a real check;
2. ask the coding agent to review the branch;
3. stop unless the result has zero critical findings;
4. save the structured review.

You need Go 1.24 or newer and Node.js 24 or newer.

## 1. Install the supervisor and SDK

```bash
go install github.com/operatorstack/yield/cmd/yskill@latest

mkdir review-skill
cd review-skill
mkdir -p fixtures
```

Make sure the Go bin directory is on your `PATH`. `go env GOPATH` prints its
parent directory; the binary normally lives in `$(go env GOPATH)/bin`.

## 2. Create the package

Create `package.json`:

```json
{
  "private": true,
  "type": "module",
  "scripts": {
    "check": "node --check main.ts"
  }
}
```

Install the SDK:

```bash
npm install @operatorstack/yield \
  --registry=https://get.operatorstack.systems/npm/
```

## 3. Add the workflow

Create `main.ts`:

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

Add `skill.json` so Yield knows how to start the program:

```json
{"run":["node","main.ts"]}
```

## 4. Add the thin skill file

Create `SKILL.md`:

```markdown
---
name: review
description: Check and review the current branch before it is shipped.
---

Run `yskill run .` and follow each returned operation exactly.

For `agent_task`, perform the task and return schema-valid JSON. Resume with
`yskill resume <run-id> --response response.json --skill .`.

Do not skip an operation or invent a response. The program owns the order and
the finish rule.
```

The file still tells the agent what the skill is for. It no longer has to
describe every branch and gate in prose.

## 5. Prove the workflow locally

Create `fixtures/responses.json`:

```json
{
  "review": {
    "critical": 0,
    "summary": "No critical findings in the fixture run."
  }
}
```

Run:

```bash
yskill test .
```

`run_command` operations execute for real. The fixture supplies only the model
and user responses. A successful result ends with:

```text
test: run <run-id> reached completed
```

## 6. Use it from your coding agent

Ask the agent to run the `review` skill in this directory. The agent reads
`SKILL.md`, starts `yskill`, performs the review operation, and resumes the
saved run. If the session closes, the run remains on disk.

Next: [understand each primitive](primitives/README.md), or follow the
[complete review tutorial](tutorials/code-review.md).

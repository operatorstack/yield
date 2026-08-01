// Example skill (TypeScript): a release runbook where the model cannot
// skip step four. Order, the human gate, and the verification requirement
// are code; judgment (the release notes) stays with the model.
import { defineSkill } from "../../sdk/typescript/src/index.ts";

defineSkill((ctx) => {
  const approval = ctx.askUser("approve-deploy", "Deploy to production?", [
    { value: "yes", label: "Deploy" },
    { value: "no", label: "Abort" },
  ]);
  if (approval !== "yes") ctx.refused("the operator declined the deploy");

  const build = ctx.runCommand("build", "echo build-ok", 300);
  ctx.require(build.exit_code === 0, "the build succeeds", build);

  const notes = ctx.agentTask<{ notes: string }>(
    "release-notes",
    "Draft one-paragraph release notes for this deploy.",
    { approved: approval },
    {
      type: "object",
      required: ["notes"],
      properties: { notes: { type: "string", minLength: 1 } },
    },
  );

  const deploy = ctx.runCommand("deploy", "echo deploy-ok", 600);
  ctx.require(deploy.exit_code === 0, "the deploy command succeeds", deploy);

  return { deployed: true, notes: notes.notes };
});

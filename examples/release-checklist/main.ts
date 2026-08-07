// Example skill (TypeScript): a release checklist where tests, review,
// approval, publishing, and registry verification cannot be skipped.
import { defineSkill } from "../../sdk/typescript/src/index.ts";

// README_EXAMPLE_START
type Review = { critical: number; summary: string };

defineSkill((ctx) => {
  const tests = ctx.runCommand("test", "echo tests-ok", 300);
  ctx.require(tests.exit_code === 0, "the test command succeeds", tests);

  const review = ctx.agentTask<Review>(
    "review-release",
    "Review this release. Report critical findings and a short summary.",
    { stdout: tests.stdout, stderr: tests.stderr },
    {
      type: "object",
      required: ["critical", "summary"],
      properties: {
        critical: { type: "integer", minimum: 0 },
        summary: { type: "string", minLength: 1 },
      },
    },
  );
  ctx.require(review.critical === 0, "the review has no critical findings", review);

  const approval = ctx.askUser("approve-publish", "Publish this package?", [
    { value: "yes", label: "Publish" },
    { value: "no", label: "Stop" },
  ]);
  if (approval !== "yes") ctx.refused("the operator declined publication");

  const publish = ctx.runCommand("publish", "echo publish-ok", 600);
  ctx.require(publish.exit_code === 0, "the publish command succeeds", publish);

  const registry = ctx.runCommand("verify-registry", "echo registry-ok", 300);
  ctx.require(registry.exit_code === 0, "the registry contains the release", registry);

  return { published: true, summary: review.summary };
});
// README_EXAMPLE_END

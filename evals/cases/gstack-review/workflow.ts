export default defineSkill((ctx) => {
  const diff = ctx.runCommand("diff", "git diff --merge-base origin/main HEAD", 60)
  ctx.require(diff.exit_code === 0 && diff.stdout.length > 0, "branch has a readable diff", diff)

  const checks = ctx.runCommand("checks", "npm test && npm run typecheck", 600)
  ctx.require(checks.exit_code === 0, "tests and types pass", checks)

  const review = ctx.agentTask("review", prompts.review, { diff: diff.stdout, checks })
  const critical = review.findings.some((finding) => finding.severity === "critical")
  ctx.require(!critical, "no unresolved critical findings", review)
  return ctx.complete(review)
})

export default defineSkill((ctx) => {
  const useCase = ctx.askUser("use-case", "What must this MCP server enable?")
  const constraints = ctx.askUser("constraints", "Which APIs, auth, and runtime apply?")
  const design = ctx.agentTask("design", prompts.design, { useCase, constraints })
  ctx.require(design.tools.length <= 8, "tool surface stays small", design)

  const approval = ctx.askUser("design-approval", "Build this tool surface?", ["build", "revise"])
  if (approval !== "build") ctx.blocked("design needs revision")
  const scaffold = ctx.runCommand("scaffold", "npm run scaffold:mcp", 120)
  ctx.require(scaffold.exit_code === 0, "server scaffolds", scaffold)
  const tests = ctx.runCommand("tests", "npm test", 600)
  ctx.require(tests.exit_code === 0, "tests pass", tests)

  const review = ctx.agentTask("review", prompts.review, { design, tests })
  ctx.require(review.blockers.length === 0, "review has no blockers", review)
  const evals = ctx.runCommand("evals", "npm run evals", 900)
  ctx.require(evals.exit_code === 0, "evaluations pass", evals)
  return ctx.complete({ design, review, evals })
})

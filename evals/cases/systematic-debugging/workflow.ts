export default defineSkill((ctx) => {
  const failure = ctx.runCommand("reproduce", "npm test -- --runInBand", 300)
  ctx.require(failure.exit_code !== 0, "failure reproduces", failure)

  const hypothesis = ctx.agentTask("hypothesis", prompts.hypothesis, { failure })
  ctx.require(Boolean(hypothesis.experiment), "hypothesis is falsifiable", hypothesis)
  const experiment = ctx.runCommand("experiment", hypothesis.experiment, 300)
  const verdict = ctx.agentTask("evaluate", prompts.evaluate, { hypothesis, experiment })
  ctx.require(verdict.supported, "root cause supported by evidence", verdict)

  const fix = ctx.askUser("fix", "Apply the proposed fix?", ["apply", "stop"])
  if (fix !== "apply") ctx.blocked("fix not approved")
  const verification = ctx.runCommand("verify", "npm test", 600)
  ctx.require(verification.exit_code === 0, "full test suite passes", verification)
  return ctx.complete({ hypothesis, verification })
})

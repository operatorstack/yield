export default defineSkill((ctx) => {
  const git = ctx.runCommand("git-state", "git remote get-url origin", 20)
  const linked = ctx.runCommand("vercel-state", "test -f .vercel/project.json", 20)
  const auth = ctx.runCommand("auth", "vercel whoami", 30)

  const state = { git: git.exit_code === 0, linked: linked.exit_code === 0, auth: auth.exit_code === 0 }
  const plan = ctx.agentTask("explain-plan", prompts.plan, state)
  const approval = ctx.askUser("deploy", "Deploy using this plan?", ["deploy", "cancel"])
  if (approval !== "deploy") ctx.refused("deployment cancelled")
  if (!state.auth) ctx.blocked("Vercel authentication required")
  if (!state.linked) ctx.runCommand("link", "vercel link", 120)

  const deploy = ctx.runCommand("deploy", "vercel deploy --prod --yes", 900)
  ctx.require(deploy.exit_code === 0, "deploy command succeeds", deploy)
  const verify = ctx.runCommand("verify", `curl -fsS ${deploy.url}`, 120)
  ctx.require(verify.exit_code === 0, "deployment responds over HTTPS", verify)
  return ctx.complete({ url: deploy.url, plan })
})

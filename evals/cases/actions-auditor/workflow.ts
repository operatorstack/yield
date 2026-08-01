export default defineSkill((ctx) => {
  const files = ctx.runCommand("discover", "find .github/workflows -type f -name '*.yml' -o -name '*.yaml'", 30)
  ctx.require(files.exit_code === 0 && files.stdout.length > 0, "workflow files found", files)

  const context = ctx.runCommand("context", "git ls-files && git status --short", 30)
  const audit = ctx.agentTask("audit", prompts.audit, { files: files.stdout, context })
  ctx.require(audit.coverage.reviewed === audit.coverage.discovered, "all workflows reviewed", audit.coverage)
  ctx.require(audit.findings.every((finding) => finding.evidence), "every finding has evidence", audit)

  const report = ctx.runCommand("report", "node scripts/render-audit.mjs", 60, { stdin: audit })
  ctx.require(report.exit_code === 0, "report generated", report)
  return ctx.complete({ findings: audit.findings, report: report.stdout })
})

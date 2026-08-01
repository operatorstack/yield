export default defineSkill((ctx) => {
  const audience = ctx.askUser("audience", "Who will read this document?")
  const outcome = ctx.askUser("outcome", "What should the reader know or decide?")
  const context = ctx.askUser("context", "Paste the source context.")

  const outline = ctx.agentTask("outline", prompts.outline, { audience, outcome, context })
  const chosen = ctx.askUser("outline-approval", "Use this outline?", ["use", "revise"])
  ctx.require(chosen === "use", "outline approved", outline)

  const draft = ctx.agentTask("draft", prompts.draft, { outline, context })
  const test = ctx.agentTask("reader-test", prompts.readerTest, { audience, draft })
  const final = ctx.agentTask("revise", prompts.revise, { draft, test })
  ctx.require(test.blocking_questions.length === 0, "reader has no blocking questions", test)
  return ctx.complete(final)
})

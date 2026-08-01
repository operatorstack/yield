import { createHash } from "node:crypto"
import { mkdir, readFile, writeFile } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { countTokens } from "gpt-tokenizer/encoding/cl100k_base"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const index = JSON.parse(await readFile(join(root, "cases/index.json"), "utf8"))
const measure = (text) => ({
  bytes: Buffer.byteLength(text),
  lines: text === "" ? 0 : text.split(/\r?\n/).length,
  tokens: countTokens(text),
})
const sha256 = (text) => createHash("sha256").update(text).digest("hex")
const round = (value) => Math.round(value * 10) / 10
const rows = []

for (const item of index.cases) {
  const rawUrl = `https://raw.githubusercontent.com/${item.source.repo}/${item.source.commit}/${item.source.path}`
  const response = await fetch(rawUrl, { headers: { "user-agent": "yield-evals/0.1" } })
  if (!response.ok) throw new Error(`${item.id}: source fetch failed (${response.status})`)
  const originalText = await response.text()
  if (sha256(originalText) !== item.source.sha256) throw new Error(`${item.id}: pinned source digest changed`)

  const skillText = await readFile(join(root, "cases", item.thin_skill), "utf8")
  const workflowText = await readFile(join(root, "cases", item.workflow), "utf8")
  const original = measure(originalText)
  const prompt = measure(skillText)
  const workflow = measure(workflowText)
  rows.push({
    id: item.id,
    label: item.label,
    source_url: `https://github.com/${item.source.repo}/blob/${item.source.commit}/${item.source.path}`,
    original_tokens: original.tokens,
    prompt_tokens: prompt.tokens,
    workflow_tokens: workflow.tokens,
    maintained_tokens: prompt.tokens + workflow.tokens,
    maintained_change_pct: round(((prompt.tokens + workflow.tokens) / original.tokens - 1) * 100),
  })
}

const sum = (field) => rows.reduce((total, row) => total + row[field], 0)
const output = {
  schema_version: 1,
  methodology_version: index.methodology_version,
  generated_at: new Date().toISOString(),
  tokenizer: "gpt-tokenizer 3.4.0 / cl100k_base",
  summary: {
    original_tokens: sum("original_tokens"),
    prompt_tokens: sum("prompt_tokens"),
    workflow_tokens: sum("workflow_tokens"),
    maintained_tokens: sum("maintained_tokens"),
  },
  rows,
}
const stamp = output.generated_at.replaceAll(":", "-")
await mkdir(join(root, "runs"), { recursive: true })
await writeFile(join(root, "runs", `source-size-${stamp}.json`), JSON.stringify(output, null, 2) + "\n")
console.log(JSON.stringify(output.summary, null, 2))

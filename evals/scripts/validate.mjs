import { readdir, readFile, stat } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { countTokens } from "gpt-tokenizer/encoding/cl100k_base"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const readJson = async (path) => JSON.parse(await readFile(join(root, path), "utf8"))
const fail = (message) => { throw new Error(message) }
const isSha256 = (value) => /^[0-9a-f]{64}$/.test(value)
const isCommit = (value) => /^[0-9a-f]{40}$/.test(value)
const round = (value) => Math.round(value * 10) / 10

const index = await readJson("cases/index.json")
const result = await readJson("results/latest.json")
if (index.schema_version !== 1) fail("unsupported case schema")
if (result.schema_version !== 1) fail("unsupported result schema")

const ids = new Set()
const localMeasurements = new Map()
for (const item of index.cases) {
  if (!item.id || ids.has(item.id)) fail(`duplicate or empty case id: ${item.id}`)
  ids.add(item.id)
  if (!isCommit(item.source.commit)) fail(`${item.id}: source commit must be a full SHA`)
  if (!isSha256(item.source.sha256)) fail(`${item.id}: source sha256 is invalid`)
  const skill = await readFile(join(root, "cases", item.thin_skill), "utf8")
  const workflow = await readFile(join(root, "cases", item.workflow), "utf8")
  if (!skill.trim() || !workflow.trim()) fail(`${item.id}: conversion source is empty`)
  localMeasurements.set(item.id, {
    prompt_tokens: countTokens(skill),
    workflow_tokens: countTokens(workflow),
  })
}

const rows = result.source_size.rows
if (rows.length !== ids.size) fail("result row count does not match cases")
for (const row of rows) {
  if (!ids.has(row.id)) fail(`result references unknown case: ${row.id}`)
  const local = localMeasurements.get(row.id)
  if (row.prompt_tokens !== local.prompt_tokens || row.workflow_tokens !== local.workflow_tokens) {
    fail(`${row.id}: published source-size row does not match the committed conversion`)
  }
  if (row.maintained_tokens !== row.prompt_tokens + row.workflow_tokens) {
    fail(`${row.id}: maintained token total is inconsistent`)
  }
  const change = round((row.maintained_tokens / row.original_tokens - 1) * 100)
  if (change !== row.maintained_change_pct) fail(`${row.id}: maintained change is inconsistent`)
}

const sum = (field) => rows.reduce((total, row) => total + row[field], 0)
const totals = result.source_size.summary
for (const [field, rowField] of [
  ["original_tokens", "original_tokens"],
  ["prompt_tokens", "prompt_tokens"],
  ["workflow_tokens", "workflow_tokens"],
  ["maintained_tokens", "maintained_tokens"],
]) {
  if (totals[field] !== sum(rowField)) fail(`summary ${field} is inconsistent`)
}
const maintainedReduction = round((1 - totals.maintained_tokens / totals.original_tokens) * 100)
if (maintainedReduction !== totals.maintained_reduction_pct) fail("summary maintained reduction is inconsistent")

const artifact = result.behavior.artifact
if (!["published", "unpublished"].includes(artifact.status)) fail("invalid artifact status")
if (artifact.status === "published" && (!artifact.uri || !isSha256(artifact.sha256))) {
  fail("published behavior result requires an artifact URI and SHA-256")
}

const forbidden = new Set(["runs", "raw", "artifacts", ".worktrees"])
const walk = async (directory) => {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (entry.name === "node_modules") continue
    const path = join(directory, entry.name)
    if (entry.isDirectory()) {
      if (directory !== root && forbidden.has(entry.name)) fail(`raw artifact directory is committed: ${path}`)
      await walk(path)
    } else if ((await stat(path)).size > 262144) {
      fail(`evaluation source file exceeds 256 KiB: ${path}`)
    }
  }
}
await walk(root)
console.log(`validated ${ids.size} conversion cases and ${rows.length} result rows`)

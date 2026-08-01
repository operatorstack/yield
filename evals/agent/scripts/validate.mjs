import { createHash } from "node:crypto"
import { readFile, readdir, stat } from "node:fs/promises"
import { dirname, join, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const agentRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const evalRoot = resolve(agentRoot, "..")
const yieldRoot = resolve(evalRoot, "..")
const result = JSON.parse(await readFile(join(evalRoot, "results/latest-agent.json"), "utf8"))
const fail = (message) => { throw new Error(message) }

async function filesUnder(path) {
  if ((await stat(path)).isFile()) return [path]
  const files = []
  for (const entry of await readdir(path, { withFileTypes: true })) {
    const child = join(path, entry.name)
    if (entry.isDirectory()) files.push(...await filesUnder(child))
    else files.push(child)
  }
  return files
}

async function sourceHash() {
  const roots = [
    [evalRoot, "agent/cases.json"],
    [evalRoot, "agent/fixtures"],
    [evalRoot, "agent/scripts"],
    [yieldRoot, "cmd/yskill"],
    [yieldRoot, "internal"],
    [yieldRoot, "sdk/typescript/src/index.ts"],
  ]
  const files = []
  for (const [base, root] of roots) files.push(...await filesUnder(join(base, root)))
  files.sort()
  const hash = createHash("sha256")
  for (const path of files) {
    hash.update(relative(yieldRoot, path)); hash.update("\0"); hash.update(await readFile(path)); hash.update("\0")
  }
  return hash.digest("hex")
}

if (result.schema_version !== 1) fail("unsupported agent result schema")
if (result.methodology_version !== "agent-equivalence-v1") fail("unsupported agent test method")
if (result.source_hash !== await sourceHash()) fail("published agent result has a stale source hash")
if (result.status !== "passed") fail("published agent result is not passing")
if (result.coverage.cases !== 6 || result.coverage.arms.length !== 2) fail("agent branch coverage is incomplete")
if (result.equivalence.total < 6 || result.equivalence.passed !== result.equivalence.total) fail("not every paired run is equivalent")
if (result.runs.some((run) => !run.passed)) fail("an agent run failed its expected trace")
console.log(`validated ${result.equivalence.passed}/${result.equivalence.total} paired agent runs`)

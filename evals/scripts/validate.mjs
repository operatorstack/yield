import { readFile } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const result = JSON.parse(await readFile(join(root, "results/latest.json"), "utf8"))
const fail = (message) => { throw new Error(message) }

if (result.schema_version !== 2) fail("unsupported result schema")
if (result.methodology_version !== "1.1") fail("unsupported methodology")
if (!/^[0-9a-f]{64}$/.test(result.source_digest)) fail("source hash is invalid")
if (result.status !== "passed") fail("published result is not passing")

const workflows = result.workflow_conformance
if (workflows.patterns !== 10 || workflows.languages.length !== 4) {
  fail("workflow matrix is incomplete")
}
if (workflows.total !== 40 || workflows.passed !== workflows.total) {
  fail("not every workflow test passed")
}
if (workflows.cases.length !== workflows.total) fail("workflow case list is incomplete")

const runtime = result.runtime_invariants
if (runtime.total !== 8 || runtime.passed !== runtime.total) {
  fail("not every runtime check passed")
}
if (runtime.cases.length !== runtime.total) fail("runtime case list is incomplete")

if (result.claim_boundary.compares_tools !== false) fail("result must not compare tools")
if (result.claim_boundary.tests_agent_judgment !== false) fail("result must isolate agent judgment")

console.log(`validated ${workflows.passed} workflow tests and ${runtime.passed} runtime checks`)

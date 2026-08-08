import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"
import { isSemanticPath, receiptSurface, semanticSurface, sourceHash } from "./surface.mjs"
import { receiptPath, validateReceipt, validateReceiptFile } from "./receipt.mjs"

test("router selects every inventoried path", () => {
  for (const path of semanticSurface) {
    const witness = path.endsWith("/") ? `${path}witness.txt` : path
    assert.equal(isSemanticPath(witness), true, witness)
  }
  assert.equal(isSemanticPath(receiptSurface), true, receiptSurface)
})

test("router skips explicit non-semantic paths", () => {
  for (const path of ["README.md", "docs/quickstart.md", "cmd/yskill/bootstrap.go", "cmd/yskill/scaffold.go", "sdk/typescript/src/index.ts", ".agents/skills/example/SKILL.md", "evals/conversion/README.md"]) {
    assert.equal(isSemanticPath(path), false, path)
  }
})

async function validReceipt() {
  const receipt = JSON.parse(await readFile(receiptPath, "utf8"))
  receipt.source_hash = await sourceHash()
  return receipt
}

test("validator rejects stale, malformed, failing, and rubber-stamping receipts", async () => {
  const mutations = [
    (r) => { r.source_hash = "stale" },
    (r) => { delete r.model },
    (r) => { r.status = "failed" },
    (r) => { r.negative_control_verdict = "accept" },
    (r) => { r.defect_detection.unreachable = false },
  ]
  for (const mutate of mutations) {
    const receipt = await validReceipt()
    mutate(receipt)
    await assert.rejects(validateReceipt(receipt))
  }
})

test("validator rejects a missing receipt", async () => {
  await assert.rejects(validateReceiptFile(`${receiptPath}.missing`))
})

test("published simple Sol receipt passes", async () => {
  await validateReceipt(await validReceipt())
})

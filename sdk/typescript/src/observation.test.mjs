import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { dirname, resolve } from "node:path"
import test from "node:test"
import { fileURLToPath } from "node:url"

import { parseRunReceipt, ReceiptError, verifyRunReceipt } from "./observation.ts"

const here = dirname(fileURLToPath(import.meta.url))
const fixture = readFileSync(
  resolve(here, "../../../ir/yield.observation.v1/testdata/run-receipt.canonical.jsonl"),
).subarray(0, -1)

test("parses and verifies the shared Go canonical receipt", () => {
  const receipt = parseRunReceipt(fixture)
  assert.equal(
    receipt.receipt_digest,
    "sha256:a0995d74d51e472f61dc2001f82b93fd8086f9c2f2816a22bb7e5c7fa3c01cdd",
  )
  assert.equal(receipt.operations[0].kind, "agent_task")
  verifyRunReceipt(receipt, fixture)
})

test("rejects unknown fields, noncanonical bytes, and digest mismatch", () => {
  const document = JSON.parse(fixture.toString("utf8"))
  document.prompt = "secret"
  assert.throws(() => parseRunReceipt(JSON.stringify(document)), ReceiptError)
  assert.throws(() => parseRunReceipt(Buffer.concat([fixture, Buffer.from("\n")])), /canonical/)
  assert.throws(
    () =>
      parseRunReceipt(
        fixture.toString("utf8").replace(document.receipt_digest, `sha256:${"0".repeat(64)}`),
      ),
    /digest/,
  )
})

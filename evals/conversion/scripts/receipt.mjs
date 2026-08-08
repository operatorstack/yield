import { readFile } from "node:fs/promises"
import { join } from "node:path"
import { evalRoot, sourceHash } from "./surface.mjs"

export const receiptPath = join(evalRoot, "results/latest-conversion.json")

export async function validateReceiptFile(path = receiptPath) {
  return validateReceipt(JSON.parse(await readFile(path, "utf8")))
}

export async function validateReceipt(receipt) {
  if (receipt === undefined) return validateReceiptFile()
  const fail = (message) => {
    throw new Error(message)
  }
  if (receipt.schema_version !== 1) fail("unsupported conversion receipt schema")
  if (receipt.methodology_version !== "semantic-disposition-v1")
    fail("unsupported conversion evaluation method")
  if (receipt.source_hash !== (await sourceHash()))
    fail("conversion receipt has a stale source hash")
  if (receipt.status !== "passed") fail("conversion receipt is not passing")
  if (
    receipt.model?.product !== "Codex CLI" ||
    receipt.model?.name !== "gpt-5.6-sol" ||
    receipt.model?.reasoning !== "medium"
  )
    fail("conversion receipt used the wrong model")
  if (receipt.sessions !== 2) fail("conversion evaluation must use exactly two fresh sessions")
  for (const key of [
    "input_tokens",
    "cached_input_tokens",
    "output_tokens",
    "reasoning_output_tokens",
  ]) {
    if (!Number.isInteger(receipt.token_usage?.[key]) || receipt.token_usage[key] < 0)
      fail(`invalid token usage: ${key}`)
  }
  const counts = receipt.clause_counts ?? {}
  if (
    counts.total !== 4 ||
    counts.control !== 1 ||
    counts.guidance !== 1 ||
    counts.both !== 1 ||
    counts.excluded !== 1
  )
    fail("conversion receipt does not cover each disposition once")
  if (receipt.candidate_verdict !== "accept") fail("generated candidate was not accepted")
  if (receipt.negative_control_verdict !== "reject") fail("negative control was not rejected")
  const defects = receipt.defect_detection ?? {}
  for (const key of [
    "missing",
    "contradictory",
    "incorrectly_duplicated",
    "excluded_without_reason",
    "unreachable",
  ]) {
    if (defects[key] !== true) fail(`judge did not detect ${key}`)
  }
  if (
    receipt.claim_boundary !==
    "Advisory evidence for this four-clause fixture. The contract is stable; model projections can differ."
  )
    fail("conversion receipt has the wrong advisory claim boundary")
  return receipt
}

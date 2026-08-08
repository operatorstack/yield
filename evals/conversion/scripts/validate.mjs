import { changedPaths, isSemanticPath } from "./surface.mjs"
import { validateReceipt } from "./receipt.mjs"

function valueAfter(flag) {
  const index = process.argv.indexOf(flag)
  return index === -1 ? undefined : process.argv[index + 1]
}

const base = valueAfter("--base") ?? process.env.EVAL_BASE_SHA
const head = valueAfter("--head") ?? process.env.EVAL_HEAD_SHA
const paths = process.argv.includes("--force") ? null : changedPaths(base, head)
const relevant = paths === null || paths.some(isSemanticPath)

if (!relevant) {
  console.log("conversion receipt skipped: no semantic-conversion source changed")
} else {
  const receipt = await validateReceipt()
  console.log(`validated conversion receipt ${receipt.source_hash.slice(0, 12)}`)
}

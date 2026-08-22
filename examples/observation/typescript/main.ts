import { readFileSync } from "node:fs"
import { argv, stderr, stdout } from "node:process"

import { parseRunReceipt } from "../../../sdk/typescript/src/index.ts"

const args = argv.slice(2)
if (args.length !== 1) {
  stderr.write("usage: node examples/observation/typescript/main.ts <canonical-receipt.json>\n")
  process.exit(1)
}

try {
  const receipt = parseRunReceipt(readFileSync(args[0]))
  const summary = {
    schema: receipt.schema,
    receipt_digest: receipt.receipt_digest,
    run_id: receipt.run.id,
    skill: receipt.skill.name,
    phase: receipt.outcome.phase,
    terminal_disposition: receipt.outcome.terminal_disposition ?? null,
    operation_summaries: receipt.operation_summaries,
  }
  stdout.write(`${JSON.stringify(summary)}\n`)
} catch (error) {
  const message = error instanceof Error ? error.message : "receipt verification failed"
  stderr.write(`receipt example: ${message}\n`)
  process.exit(1)
}

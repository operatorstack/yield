#!/usr/bin/env node
import { appendFile } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const [kind, id, raw = "null"] = process.argv.slice(2)
if (!["agent_task", "requirement", "terminal"].includes(kind)) throw new Error(`unsupported event kind: ${kind}`)
const event = { kind, id, result: JSON.parse(raw) }
await appendFile(join(root, ".eval/observed.jsonl"), JSON.stringify(event) + "\n")

#!/usr/bin/env node
import { appendFile, readFile } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const id = process.argv[2]
const testCase = JSON.parse(await readFile(join(root, ".eval/case.json"), "utf8"))
const command = testCase.commands[id]
if (!command) throw new Error(`unknown command step: ${id}`)
await appendFile(join(root, ".eval/observed.jsonl"), JSON.stringify({ kind: "run_command", id, exit_code: command.exit_code }) + "\n")
process.stdout.write(command.stdout)
process.stderr.write(command.stderr)
process.exitCode = command.exit_code

#!/usr/bin/env node
import { appendFile, readFile } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const id = process.argv[2]
const testCase = JSON.parse(await readFile(join(root, ".eval/case.json"), "utf8"))
const value = testCase.answers[id]
if (typeof value !== "string") throw new Error(`no user answer for: ${id}`)
await appendFile(join(root, ".eval/observed.jsonl"), JSON.stringify({ kind: "ask_user", id, result: { value } }) + "\n")
process.stdout.write(value + "\n")

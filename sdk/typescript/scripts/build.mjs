import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { stripTypeScriptTypes } from "node:module"
import { dirname, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const here = dirname(fileURLToPath(import.meta.url))
const root = resolve(here, "..")
const sourcePath = resolve(root, "src/index.ts")
const distPath = resolve(root, "dist")
const outputPath = resolve(distPath, "index.js")
const source = readFileSync(sourcePath, "utf8")
const runtime = stripTypeScriptTypes(source, { mode: "transform" })

rmSync(distPath, { recursive: true, force: true })
mkdirSync(dirname(outputPath), { recursive: true })
writeFileSync(
  outputPath,
  "// Generated from src/index.ts by scripts/build.mjs. Do not edit.\n" + runtime,
)

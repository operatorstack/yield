import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { stripTypeScriptTypes } from "node:module"
import { dirname, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const here = dirname(fileURLToPath(import.meta.url))
const root = resolve(here, "..")
const distPath = resolve(root, "dist")

rmSync(distPath, { recursive: true, force: true })
for (const name of ["index", "observation"]) {
  const sourcePath = resolve(root, `src/${name}.ts`)
  const outputPath = resolve(distPath, `${name}.js`)
  const source = readFileSync(sourcePath, "utf8")
  const runtime = stripTypeScriptTypes(source, { mode: "transform" })
  mkdirSync(dirname(outputPath), { recursive: true })
  writeFileSync(
    outputPath,
    `// Generated from src/${name}.ts by scripts/build.mjs. Do not edit.\n` + runtime,
  )
}

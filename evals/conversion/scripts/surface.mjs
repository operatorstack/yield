import { createHash } from "node:crypto"
import { execFileSync } from "node:child_process"
import { readFile, readdir, stat } from "node:fs/promises"
import { dirname, join, relative, resolve } from "node:path"
import { fileURLToPath } from "node:url"

export const conversionRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..")
export const evalRoot = resolve(conversionRoot, "..")
export const yieldRoot = resolve(evalRoot, "..")

export const semanticSurface = Object.freeze([
  "cmd/yskill/bootstrap_templates.go",
  "examples/convert-skill/",
  "evals/conversion/fixtures/",
  "evals/conversion/judge-schema.json",
  "evals/conversion/scripts/",
])
export const receiptSurface = "evals/results/latest-conversion.json"

export function isSemanticPath(path) {
  const normalized = path.replaceAll("\\", "/")
  return (
    normalized === receiptSurface ||
    semanticSurface.some((entry) =>
      entry.endsWith("/") ? normalized.startsWith(entry) : normalized === entry,
    )
  )
}

async function filesUnder(path) {
  if ((await stat(path)).isFile()) return [path]
  const files = []
  for (const entry of await readdir(path, { withFileTypes: true })) {
    const child = join(path, entry.name)
    if (entry.isDirectory()) files.push(...(await filesUnder(child)))
    else files.push(child)
  }
  return files
}

export async function semanticFiles() {
  const files = []
  for (const entry of semanticSurface) files.push(...(await filesUnder(join(yieldRoot, entry))))
  return [...new Set(files)].sort()
}

export async function sourceHash() {
  const hash = createHash("sha256")
  for (const path of await semanticFiles()) {
    hash.update(relative(yieldRoot, path).replaceAll("\\", "/"))
    hash.update("\0")
    hash.update(await readFile(path))
    hash.update("\0")
  }
  return hash.digest("hex")
}

export function changedPaths(base, head) {
  if (!base || !head) return null
  const effectiveBase = /^0+$/.test(base) ? `${head}^` : base
  return execFileSync("git", ["diff", "--name-only", effectiveBase, head], {
    cwd: yieldRoot,
    encoding: "utf8",
  })
    .split("\n")
    .map((path) => path.trim())
    .filter(Boolean)
}

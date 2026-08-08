import test from "node:test"
import assert from "node:assert/strict"
import { execFileSync } from "node:child_process"
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises"
import { join } from "node:path"
import { tmpdir } from "node:os"

test("npm initializer forwards bootstrap and user arguments to the matching CLI", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "create-yield-"))
  t.after(() => rm(root, { recursive: true, force: true }))
  const initializer = join(root, "node_modules/@operatorstack/create-yield")
  const sdk = join(root, "node_modules/@operatorstack/yield")
  await mkdir(join(initializer, "bin"), { recursive: true })
  await mkdir(join(sdk, "dist"), { recursive: true })
  await mkdir(join(sdk, "bin"), { recursive: true })
  await cp(
    join(import.meta.dirname, "create-yield/bin/create-yield.mjs"),
    join(initializer, "bin/create-yield.mjs"),
  )
  await writeFile(
    join(sdk, "package.json"),
    JSON.stringify({
      name: "@operatorstack/yield",
      type: "module",
      exports: { ".": "./dist/index.js" },
    }),
  )
  await writeFile(join(sdk, "dist/index.js"), "export {};\n")
  await writeFile(
    join(sdk, "bin/yskill.mjs"),
    "import { writeFileSync } from 'node:fs'; writeFileSync(process.env.RECEIPT, JSON.stringify(process.argv.slice(2)));\n",
  )
  const receipt = join(root, "receipt.json")
  execFileSync(
    process.execPath,
    [join(initializer, "bin/create-yield.mjs"), "--root", root, "--dry-run"],
    {
      cwd: root,
      env: { ...process.env, RECEIPT: receipt },
    },
  )
  assert.deepEqual(JSON.parse(await readFile(receipt, "utf8")), [
    "bootstrap",
    "--language",
    "typescript",
    "--root",
    root,
    "--dry-run",
  ])
})

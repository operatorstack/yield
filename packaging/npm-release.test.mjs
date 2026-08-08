import test from "node:test"
import assert from "node:assert/strict"
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { packReleaseUnit } from "./npm-release.mjs"

test("packs immutable archives and preserves runtime execute mode", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "yield-npm-release-"))
  t.after(() => rm(root, { recursive: true, force: true }))
  const source = join(root, "source")
  const output = join(root, "output")
  const runtime = join(source, "linux-arm64")
  const sdk = join(source, "yield")
  await mkdir(runtime, { recursive: true })
  await mkdir(sdk, { recursive: true })
  await writeFile(
    join(runtime, "package.json"),
    JSON.stringify({
      name: "@operatorstack/yield-linux-arm64",
      version: "1.2.3",
      files: ["yskill"],
    }),
  )
  await writeFile(join(runtime, "yskill"), "#!/bin/sh\necho ok\n", { mode: 0o755 })
  await writeFile(
    join(sdk, "package.json"),
    JSON.stringify({ name: "@operatorstack/yield", version: "1.2.3", files: ["index.js"] }),
  )
  await writeFile(join(sdk, "index.js"), "export {};\n")

  const result = await packReleaseUnit({ source, output, sourceSHA: "a".repeat(40) })
  assert.equal(result.version, "1.2.3")
  assert.equal(result.source_sha, "a".repeat(40))
  assert.deepEqual(
    result.archives.map((entry) => entry.name),
    ["@operatorstack/yield-linux-arm64", "@operatorstack/yield"],
  )
  assert.match(result.archives[0].sha256, /^[0-9a-f]{64}$/)
})

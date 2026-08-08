import test from "node:test"
import assert from "node:assert/strict"
import { createHash } from "node:crypto"
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { crateNames, indexPath, registryRecord, status, verifyRelease } from "./crates-release.mjs"

function response(statusCode, text = "") {
  return { status: statusCode, ok: statusCode >= 200 && statusCode < 300, text: async () => text }
}

test("maps public crate names to sparse index paths", () => {
  assert.equal(indexPath("yieldskill"), "yi/el/yieldskill")
  assert.equal(indexPath("yieldskill-runtime-linux-amd64"), "yi/el/yieldskill-runtime-linux-amd64")
})

test("distinguishes an absent version from an absent crate", async () => {
  assert.equal(await registryRecord("yieldskill", "1.2.3", async () => response(404)), null)
  assert.equal(
    await registryRecord("yieldskill", "1.2.3", async () => response(200, '{"vers":"1.2.2"}\n')),
    null,
  )
})

test("refuses an immutable version with a different checksum", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "yield-crate-status-"))
  t.after(() => rm(root, { recursive: true, force: true }))
  const archive = join(root, "yieldskill-1.2.3.crate")
  await writeFile(archive, "release-unit")
  await assert.rejects(
    status({
      name: "yieldskill",
      version: "1.2.3",
      archive,
      fetchImpl: async () =>
        response(200, `${JSON.stringify({ vers: "1.2.3", cksum: "0".repeat(64) })}\n`),
    }),
    /checksum does not match/,
  )
})

test("verifies the complete seven-crate release unit", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "yield-crates-verify-"))
  t.after(() => rm(root, { recursive: true, force: true }))
  await mkdir(root, { recursive: true })
  const checksums = new Map()
  for (const name of crateNames) {
    const body = `archive:${name}`
    await writeFile(join(root, `${name}-1.2.3.crate`), body)
    checksums.set(name, createHash("sha256").update(body).digest("hex"))
  }
  const verified = await verifyRelease({
    version: "1.2.3",
    archives: root,
    fetchImpl: async (url) => {
      const name = url.slice(url.lastIndexOf("/") + 1)
      return response(200, `${JSON.stringify({ vers: "1.2.3", cksum: checksums.get(name) })}\n`)
    },
  })
  assert.deepEqual(verified, crateNames)
})

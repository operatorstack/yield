import test from "node:test"
import assert from "node:assert/strict"
import { mkdtemp, rm, writeFile } from "node:fs/promises"
import { createHash } from "node:crypto"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { createPackageContract } from "./package-contract.mjs"

test("binds package and evaluation evidence to one release", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "yield-contract-"))
  t.after(() => rm(root, { recursive: true, force: true }))
  const npm = join(root, "npm.json")
  const evaluation = join(root, "evaluation.json")
  const archives = await Promise.all(
    Array.from({ length: 8 }, async (_, index) => {
      const file = `pkg-${index}.tgz`
      const bytes = Buffer.from(`archive-${index}`)
      await writeFile(join(root, file), bytes)
      return {
        name: `pkg-${index}`,
        file,
        sha256: createHash("sha256").update(bytes).digest("hex"),
      }
    }),
  )
  await writeFile(
    npm,
    JSON.stringify({ schema_version: 1, version: "1.2.3", source_sha: "c".repeat(40), archives }),
  )
  await writeFile(
    evaluation,
    JSON.stringify({ schema_version: 2, status: "passed", source_digest: "b".repeat(64) }),
  )
  const { contract, promotion } = await createPackageContract({
    version: "1.2.3",
    sourceSHA: "c".repeat(40),
    npmReleasePath: npm,
    evaluationPath: evaluation,
    releaseURL: "https://github.com/operatorstack/yield/releases/tag/v1.2.3",
  })
  assert.equal(contract.version, "1.2.3")
  assert.equal(contract.npm_archives.length, 8)
  assert.equal(promotion.contract_digest.length, 64)
  await assert.rejects(
    () =>
      createPackageContract({
        version: "1.2.3",
        sourceSHA: "c".repeat(40),
        npmReleasePath: npm,
        evaluationPath: evaluation,
        releaseURL: "https://example.com",
      }),
    /release URL/,
  )
  await writeFile(join(root, archives[0].file), "altered archive")
  await assert.rejects(
    () =>
      createPackageContract({
        version: "1.2.3",
        sourceSHA: "c".repeat(40),
        npmReleasePath: npm,
        evaluationPath: evaluation,
        releaseURL: "https://github.com/operatorstack/yield/releases/tag/v1.2.3",
      }),
    /digest/,
  )
})

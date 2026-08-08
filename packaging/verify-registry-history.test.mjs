import test from "node:test"
import assert from "node:assert/strict"
import { npmPackage, rustPackage, targets } from "./targets.mjs"
import { verifyRegistryHistory } from "./verify-registry-history.mjs"

const versions = ["0.1.23", "0.1.24"]

function response(body, status = 200) {
  return { ok: status >= 200 && status < 300, status, text: async () => body }
}

function registry({ omit = "" } = {}) {
  return async (url) => {
    if (url.includes("/npm/")) {
      const name = decodeURIComponent(url.split("/npm/")[1])
      const present = Object.fromEntries(
        versions.filter((version) => `${name}@${version}` !== omit).map((version) => [version, {}]),
      )
      return response(JSON.stringify({ name, versions: present }))
    }
    if (url.includes("/pip/simple/yieldskill/")) {
      const files = versions.flatMap((version) =>
        targets.map((target) => `yieldskill-${version}-py3-none-${target.pythonTag}.whl`),
      )
      return response(files.filter((file) => `Python ${file}` !== omit).join("\n"))
    }
    if (url.includes("/go/github.com/operatorstack/yield/@v/list")) {
      return response(
        versions
          .filter((version) => `Go github.com/operatorstack/yield@v${version}` !== omit)
          .map((version) => `v${version}`)
          .join("\n"),
      )
    }
    if (url.includes("/cargo/index/")) {
      const name = url.split("/").at(-1)
      const records = versions
        .filter((version) => `Cargo ${name}@${version}` !== omit)
        .map((vers) => JSON.stringify({ name, vers }))
      return response(records.join("\n") + "\n")
    }
    return response("not found", 404)
  }
}

test("verifies every SDK, platform package, wheel, and runtime crate", async () => {
  const result = await verifyRegistryHistory({
    versions,
    base: "https://registry.test",
    fetchImpl: registry(),
  })
  assert.deepEqual(result.versions, versions)
  assert.deepEqual(result.languages, ["typescript", "python", "go", "rust"])
  assert.equal(result.targets.length, 6)
})

test("reports a missing historical version instead of accepting latest", async () => {
  const missing = `Cargo ${rustPackage(targets[0])}@${versions[0]}`
  await assert.rejects(
    verifyRegistryHistory({
      versions,
      base: "https://registry.test",
      fetchImpl: registry({ omit: missing }),
    }),
    new RegExp(missing),
  )
})

test("reports missing platform packages, not only public package names", async () => {
  const missing = `${npmPackage(targets[1])}@${versions[1]}`
  await assert.rejects(
    verifyRegistryHistory({
      versions,
      base: "https://registry.test",
      fetchImpl: registry({ omit: missing }),
    }),
    new RegExp(missing.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")),
  )
})

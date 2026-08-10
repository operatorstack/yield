import test from "node:test"
import assert from "node:assert/strict"
import { classifyCollection, inspectRemote, validateMirrorIdentity } from "./private-mirror.mjs"

const expected = [
  { name: "one", sha256: "a".repeat(64) },
  { name: "two", sha256: "b".repeat(64) },
]

test("classifies a wholly missing or byte-identical private release", () => {
  assert.equal(classifyCollection(expected, [], "fixture"), "missing")
  assert.equal(classifyCollection(expected, expected, "fixture"), "matched")
})

test("reports partial releases and refuses checksum drift", () => {
  assert.equal(classifyCollection(expected, expected.slice(0, 1), "fixture"), "partial")
  assert.throws(
    () => classifyCollection(expected, [{ name: "one", sha256: "c".repeat(64) }], "fixture"),
    /checksum drift/,
  )
})

test("binds the mirror to one stable version and source revision", () => {
  const input = {
    version: "0.5.1",
    sourceSHA: "1".repeat(40),
    npmRelease: { schema_version: 1, version: "0.5.1", source_sha: "1".repeat(40) },
  }
  assert.doesNotThrow(() => validateMirrorIdentity(input))
  assert.throws(
    () => validateMirrorIdentity({ ...input, version: "0.5.0" }),
    /version does not match/,
  )
  assert.throws(
    () => validateMirrorIdentity({ ...input, sourceSHA: "2".repeat(40) }),
    /source SHA does not match/,
  )
})

test("treats a missing private crate payload as recoverable", async () => {
  const checksum = "a".repeat(64)
  const manifest = {
    version: "0.5.1",
    npm: [],
    python: [],
    go: { module: "github.com/operatorstack/yield", version: "v0.5.1" },
    rust: [{ name: "yieldskill", sha256: checksum }],
  }
  const fetchImpl = async (url) => {
    if (String(url).includes("/cargo/index/")) {
      return new Response(
        `${JSON.stringify({ name: "yieldskill", vers: "0.5.1", cksum: checksum })}\n`,
      )
    }
    return new Response("missing", { status: 404 })
  }

  const result = await inspectRemote(manifest, { base: "https://mirror.test", fetchImpl })
  assert.equal(result.states.rust, "missing")
  assert.deepEqual(result.missing.rust, manifest.rust)
})

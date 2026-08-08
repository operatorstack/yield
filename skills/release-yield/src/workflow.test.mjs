import test from "node:test"
import assert from "node:assert/strict"
import { runReleaseYield } from "./workflow.ts"

const sha = "a".repeat(40)

function successReceipts(overrides = {}) {
  return {
    preflight: { status: "ok", source_sha: sha },
    "dispatch-dry-run": {
      status: "ok",
      source_sha: sha,
      run_id: "10",
      run_url: "https://example.test/10",
    },
    "wait-dry-run": {
      status: "ok",
      source_sha: sha,
      run_id: "10",
      run_url: "https://example.test/10",
    },
    "resolve-plan": {
      status: "ok",
      source_sha: sha,
      version: "1.2.3",
      tag: "v1.2.3",
      changesets: [
        { path: ".changeset/example.md", bump: "patch", summary: "Improve release confirmation." },
      ],
    },
    "dispatch-release": {
      status: "ok",
      source_sha: sha,
      run_id: "11",
      run_url: "https://example.test/11",
      publisher_baseline: "1,2",
      finalizer_baseline: "3,4",
    },
    "wait-release-control": {
      status: "ok",
      run_id: "11",
      run_url: "https://example.test/11",
      publisher_run_id: "12",
      publisher_run_url: "https://example.test/12",
    },
    "wait-publishers": { status: "ok", run_id: "12", run_url: "https://example.test/12" },
    "wait-finalizer": { status: "ok", run_id: "13", run_url: "https://example.test/13" },
    "verify-public-release": {
      status: "ok",
      targets: {
        npm: 8,
        pypi: { project: "yieldskill", wheels: 6 },
        crates: 7,
        go: "github.com/operatorstack/yield",
      },
    },
    ...overrides,
  }
}

function context({
  mode = "release",
  bump = "patch",
  confirmation = "confirm",
  authorization = "release",
  receipts = successReceipts(),
} = {}) {
  const operations = []
  return {
    operations,
    askUser(id) {
      operations.push(id)
      if (id === "select-mode") return mode
      if (id === "select-bump") return bump
      if (id === "confirm-high-impact-bump") return confirmation
      return authorization
    },
    runCommand(id) {
      operations.push(id)
      const receipt = receipts[id]
      assert.ok(receipt, `missing receipt for ${id}`)
      return { exit_code: 0, stdout: JSON.stringify(receipt), stderr: "" }
    },
    require(ok, claim) {
      if (!ok) throw new Error(`requirement_failed: ${claim}`)
    },
    blocked(reason) {
      throw new Error(`blocked: ${reason}`)
    },
    refused(reason) {
      throw new Error(`refused: ${reason}`)
    },
  }
}

test("enforces dry run, immutable authorization, protected publication, and verification order", () => {
  const ctx = context()
  const result = runReleaseYield(ctx)
  assert.deepEqual(ctx.operations, [
    "select-mode",
    "select-bump",
    "preflight",
    "dispatch-dry-run",
    "wait-dry-run",
    "resolve-plan",
    "authorize-release",
    "dispatch-release",
    "wait-release-control",
    "wait-publishers",
    "wait-finalizer",
    "verify-public-release",
  ])
  assert.equal(result.version, "1.2.3")
  assert.equal(result.source_sha, sha)
  assert.equal(result.verified.npm, 8)
})

test("stops before live dispatch when authorization is declined", () => {
  const ctx = context({ authorization: "stop" })
  assert.throws(() => runReleaseYield(ctx), /refused: release of v1\.2\.3 was not authorized/)
  assert.equal(ctx.operations.includes("dispatch-release"), false)
})

test("dry-run-only completes after the verified plan without asking for release authorization", () => {
  const ctx = context({ mode: "dry-run" })
  const result = runReleaseYield(ctx)
  assert.equal(result.mode, "dry-run")
  assert.equal(result.version, "1.2.3")
  assert.equal(result.changesets.length, 1)
  assert.equal(ctx.operations.includes("authorize-release"), false)
  assert.equal(ctx.operations.includes("dispatch-release"), false)
})

test("auto and patch bumps do not ask for high-impact confirmation", () => {
  for (const bump of ["auto", "patch"]) {
    const ctx = context({ mode: "dry-run", bump })
    runReleaseYield(ctx)
    assert.equal(ctx.operations.includes("confirm-high-impact-bump"), false)
  }
})

test("minor and major bumps require confirmation before preflight", () => {
  for (const bump of ["minor", "major"]) {
    const ctx = context({ mode: "dry-run", bump })
    runReleaseYield(ctx)
    assert.deepEqual(ctx.operations.slice(0, 4), [
      "select-mode",
      "select-bump",
      "confirm-high-impact-bump",
      "preflight",
    ])
  }
})

test("cancelling a minor or major bump stops before any GitHub operation", () => {
  for (const bump of ["minor", "major"]) {
    const ctx = context({ bump, confirmation: "cancel" })
    assert.throws(
      () => runReleaseYield(ctx),
      new RegExp(`refused: ${bump} release intent was not confirmed`),
    )
    assert.deepEqual(ctx.operations, ["select-mode", "select-bump", "confirm-high-impact-bump"])
  }
})

test("reports a GitHub authority boundary as blocked", () => {
  const ctx = context({
    receipts: successReceipts({
      preflight: { status: "blocked", reason: "GitHub denied workflow dispatch" },
    }),
  })
  assert.throws(() => runReleaseYield(ctx), /blocked: GitHub denied workflow dispatch/)
  assert.deepEqual(ctx.operations, ["select-mode", "select-bump", "preflight"])
})

test("refuses plan drift before authorization", () => {
  const ctx = context({
    receipts: successReceipts({
      "resolve-plan": {
        status: "ok",
        source_sha: "b".repeat(40),
        version: "1.2.3",
        tag: "v1.2.3",
        changesets: [
          {
            path: ".changeset/example.md",
            bump: "patch",
            summary: "Improve release confirmation.",
          },
        ],
      },
    }),
  })
  assert.throws(() => runReleaseYield(ctx), /displayed plan uses the dry-run source SHA/)
  assert.equal(ctx.operations.includes("authorize-release"), false)
})

test("rejects malformed controller receipts", () => {
  const ctx = context()
  ctx.runCommand = (id) => {
    ctx.operations.push(id)
    return { exit_code: 0, stdout: "not-json", stderr: "" }
  }
  assert.throws(() => runReleaseYield(ctx), /controller returned invalid JSON/)
})

test("rejects a timed-out controller operation", () => {
  const ctx = context()
  ctx.runCommand = (id) => {
    ctx.operations.push(id)
    return { exit_code: 0, timed_out: true, stdout: "", stderr: "controller timeout" }
  }
  assert.throws(
    () => runReleaseYield(ctx),
    /requirement_failed: the protected main preflight passes/,
  )
  assert.deepEqual(ctx.operations, ["select-mode", "select-bump", "preflight"])
})

test("rejects a malformed Changeset plan before release authorization", () => {
  const ctx = context({
    receipts: successReceipts({
      "resolve-plan": {
        status: "ok",
        source_sha: sha,
        version: "1.2.3",
        tag: "v1.2.3",
        changesets: [],
      },
    }),
  })
  assert.throws(() => runReleaseYield(ctx), /release plan contains at least one Changeset/)
  assert.equal(ctx.operations.includes("authorize-release"), false)
})

import test from "node:test"
import assert from "node:assert/strict"
import { runReleaseYield } from "./workflow.ts"

const sha = "a".repeat(40)
const baseTag = "v1.2.2"
const commitRange = `${baseTag}..${sha}`
const changesets = [
  { path: ".changeset/example.md", bump: "patch", summary: "Improve release confirmation." },
]

function inspection(overrides = {}) {
  return {
    status: "ok",
    baseTag,
    sourceSha: sha,
    commitRange,
    targetContract: "full-train",
    commits: [{ sha, summary: "Improve release confirmation" }],
    changesets,
    ...overrides,
  }
}

function plan(overrides = {}) {
  return {
    status: "ok",
    source_sha: sha,
    version: "1.2.3",
    tag: "v1.2.3",
    changesets,
    release_basis: "changesets",
    notes_source: "changesets",
    base_tag: baseTag,
    commit_range: commitRange,
    commit_count: 1,
    target_contract: "full-train",
    ...overrides,
  }
}

function successReceipts(overrides = {}) {
  return {
    "inspect-release-input": inspection(),
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
    "resolve-plan": plan(),
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

function context({ mode = "release", choices = {}, receipts = successReceipts() } = {}) {
  const operations = []
  const commands = []
  const defaults = {
    "select-mode": mode,
    "select-release-scope": "full-train",
    "select-version-basis": "changesets",
    "select-note-source": "changesets",
    "authorize-release": "release",
  }
  return {
    operations,
    commands,
    askUser(id) {
      operations.push(id)
      const value = choices[id] ?? defaults[id]
      assert.ok(value, `missing choice for ${id}`)
      return value
    },
    runCommand(id, command) {
      operations.push(id)
      commands.push({ id, command })
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

test("enforces the full decision tree, immutable authorization, and registry verification order", () => {
  const ctx = context()
  const result = runReleaseYield(ctx)
  assert.deepEqual(ctx.operations, [
    "select-mode",
    "select-release-scope",
    "inspect-release-input",
    "select-version-basis",
    "select-note-source",
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
  assert.equal(result.release_basis, "changesets")
  assert.equal(result.verified.npm, 8)
  assert.match(
    ctx.commands.find(({ id }) => id === "dispatch-dry-run").command,
    /--notes-source changesets/,
  )
})

test("uses an explicit bump and Git history when no Changesets are pending", () => {
  const noChangesets = inspection({ changesets: [] })
  const explicitPlan = plan({
    changesets: [],
    release_basis: "explicit-bump",
    notes_source: "git-history",
  })
  const ctx = context({
    mode: "dry-run",
    choices: {
      "handle-no-changesets": "explicit",
      "select-explicit-bump": "patch",
      "confirm-git-history-notes": "git-history",
    },
    receipts: successReceipts({
      "inspect-release-input": noChangesets,
      "resolve-plan": explicitPlan,
    }),
  })
  const result = runReleaseYield(ctx)
  assert.equal(result.release_basis, "explicit-bump")
  assert.equal(result.note_source, "git-history")
  assert.match(
    ctx.commands.find(({ id }) => id === "dispatch-dry-run").command,
    /--notes-source git-history/,
  )
  assert.equal(ctx.operations.includes("authorize-release"), false)
  assert.equal(ctx.operations.includes("dispatch-release"), false)
})

test("confirms an explicit minor before preflight", () => {
  const ctx = context({
    mode: "dry-run",
    choices: {
      "select-version-basis": "explicit",
      "select-explicit-bump": "minor",
      "confirm-high-impact-bump": "confirm",
      "select-note-source": "git-history",
    },
    receipts: successReceipts({
      "resolve-plan": plan({ release_basis: "explicit-bump", notes_source: "git-history" }),
    }),
  })
  runReleaseYield(ctx)
  assert.deepEqual(ctx.operations.slice(0, 7), [
    "select-mode",
    "select-release-scope",
    "inspect-release-input",
    "select-version-basis",
    "select-explicit-bump",
    "select-note-source",
    "confirm-high-impact-bump",
  ])
})

test("stops before GitHub activity when the operator declines any early decision", () => {
  for (const [label, config] of [
    ["mode", { mode: "stop" }],
    ["scope", { choices: { "select-release-scope": "design-target-specific" } }],
    [
      "no Changesets",
      {
        choices: { "handle-no-changesets": "stop" },
        receipts: successReceipts({ "inspect-release-input": inspection({ changesets: [] }) }),
      },
    ],
    ["notes", { choices: { "select-note-source": "stop" } }],
  ]) {
    const ctx = context(config)
    assert.throws(() => runReleaseYield(ctx), /refused:/, label)
    assert.equal(ctx.operations.includes("preflight"), false, label)
    assert.equal(ctx.operations.includes("dispatch-dry-run"), false, label)
  }
})

test("dry-run returns the selected full-train plan without live authorization", () => {
  const ctx = context({ mode: "dry-run" })
  const result = runReleaseYield(ctx)
  assert.equal(result.mode, "dry-run")
  assert.equal(result.target_contract, "full-train")
  assert.equal(ctx.operations.includes("authorize-release"), false)
  assert.equal(ctx.operations.includes("dispatch-release"), false)
})

test("refuses plan drift before authorization", () => {
  const ctx = context({
    receipts: successReceipts({ "resolve-plan": plan({ source_sha: "b".repeat(40) }) }),
  })
  assert.throws(() => runReleaseYield(ctx), /displayed plan uses the dry-run source SHA/)
  assert.equal(ctx.operations.includes("authorize-release"), false)
})

test("refuses a non-full-train plan before authorization", () => {
  const ctx = context({
    receipts: successReceipts({ "resolve-plan": plan({ target_contract: "npm-only" }) }),
  })
  assert.throws(() => runReleaseYield(ctx), /valid target_contract/)
  assert.equal(ctx.operations.includes("authorize-release"), false)
})

test("refuses a Changeset-basis plan without Changesets", () => {
  const ctx = context({
    receipts: successReceipts({ "resolve-plan": plan({ changesets: [] }) }),
  })
  assert.throws(() => runReleaseYield(ctx), /Changeset-based plan contains pending Changesets/)
  assert.equal(ctx.operations.includes("authorize-release"), false)
})

test("reports a GitHub authority boundary as blocked", () => {
  const ctx = context({
    receipts: successReceipts({
      preflight: { status: "blocked", reason: "GitHub denied workflow dispatch" },
    }),
  })
  assert.throws(() => runReleaseYield(ctx), /blocked: GitHub denied workflow dispatch/)
})

test("rejects malformed controller receipts", () => {
  const ctx = context()
  ctx.runCommand = (id) => {
    ctx.operations.push(id)
    return { exit_code: 0, stdout: "not-json", stderr: "" }
  }
  assert.throws(() => runReleaseYield(ctx), /controller returned invalid JSON/)
})

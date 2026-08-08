import test from "node:test"
import assert from "node:assert/strict"
import { bumpVersion, parseChangeset, planRelease } from "./release-plan.mjs"

const changeset = (bump, summary = "Ship it") =>
  parseChangeset(`---\n"@operatorstack/yield": ${bump}\n---\n\n${summary}\n`)

test("aggregates the highest pending Changeset bump", () => {
  assert.equal(
    planRelease({ baseVersion: "0.1.29", changesets: [changeset("patch"), changeset("minor")] })
      .version,
    "0.2.0",
  )
})

test("allows an explicit bump to raise but not lower intent", () => {
  assert.equal(
    planRelease({ baseVersion: "0.1.29", changesets: [changeset("patch")], requestedBump: "major" })
      .version,
    "1.0.0",
  )
  assert.throws(
    () =>
      planRelease({
        baseVersion: "0.1.29",
        changesets: [changeset("major")],
        requestedBump: "minor",
      }),
    /cannot lower/,
  )
})

test("requires pending release intent", () => {
  assert.throws(() => planRelease({ baseVersion: "0.1.29", changesets: [] }), /at least one/)
})

test("rejects another package or malformed bump", () => {
  assert.throws(
    () => parseChangeset(`---\nother: patch\n---\n\nNo\n`),
    /only @operatorstack\/yield/,
  )
  assert.throws(
    () => parseChangeset(`---\n"@operatorstack/yield": huge\n---\n\nNo\n`),
    /patch, minor, or major/,
  )
})

test("applies ordinary semantic version increments", () => {
  assert.equal(bumpVersion("0.1.29", "patch"), "0.1.30")
  assert.equal(bumpVersion("0.1.29", "minor"), "0.2.0")
  assert.equal(bumpVersion("0.1.29", "major"), "1.0.0")
})

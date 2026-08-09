import test from "node:test"
import assert from "node:assert/strict"
import { bumpVersion, parseChangeset, planRelease, releaseNotes } from "./release-plan.mjs"

const commits = [{ sha: "a".repeat(40), summary: "Ship a deterministic release plan" }]
const changeset = (bump, summary = "Ship it") =>
  parseChangeset(`---\n"@operatorstack/yield": ${bump}\n---\n\n${summary}\n`)

test("aggregates the highest pending Changeset bump", () => {
  const plan = planRelease({
    baseVersion: "0.1.29",
    changesets: [changeset("patch"), changeset("minor")],
    commits,
  })
  assert.equal(plan.version, "0.2.0")
  assert.equal(plan.basis, "changesets")
  assert.equal(plan.notesSource, "changesets")
})

test("allows an explicit bump to raise but not lower pending Changeset intent", () => {
  assert.equal(
    planRelease({
      baseVersion: "0.1.29",
      changesets: [changeset("patch")],
      requestedBump: "major",
      commits,
    }).version,
    "1.0.0",
  )
  assert.throws(
    () =>
      planRelease({
        baseVersion: "0.1.29",
        changesets: [changeset("major")],
        requestedBump: "minor",
        commits,
      }),
    /cannot lower/,
  )
})

test("permits an explicit release without Changesets and uses Git history", () => {
  const plan = planRelease({
    baseVersion: "0.1.29",
    changesets: [],
    requestedBump: "minor",
    commits,
  })
  assert.equal(plan.version, "0.2.0")
  assert.equal(plan.basis, "explicit-bump")
  assert.equal(plan.notesSource, "git-history")
  assert.equal(
    releaseNotes(plan, "v0.1.29"),
    "# Yield 0.2.0\n\n## Changes since v0.1.29\n\n- Ship a deterministic release plan (aaaaaaaaaaaa)\n",
  )
})

test("automatic releases still require Changesets", () => {
  assert.throws(
    () => planRelease({ baseVersion: "0.1.29", changesets: [], commits }),
    /automatic releases require at least one pending Changeset/,
  )
})

test("refuses an explicit release with no new commit and Changeset notes without Changesets", () => {
  assert.throws(
    () =>
      planRelease({
        baseVersion: "0.1.29",
        changesets: [],
        requestedBump: "patch",
        commits: [],
      }),
    /at least one commit/,
  )
  assert.throws(
    () =>
      planRelease({
        baseVersion: "0.1.29",
        changesets: [],
        requestedBump: "patch",
        notesSource: "changesets",
        commits,
      }),
    /Changeset notes require/,
  )
})

test("applies ordinary semantic version increments", () => {
  assert.equal(bumpVersion("0.1.29", "patch"), "0.1.30")
  assert.equal(bumpVersion("0.1.29", "minor"), "0.2.0")
  assert.equal(bumpVersion("0.1.29", "major"), "1.0.0")
})

import type { CommandResult, Context } from "@operatorstack/yield"

export type ReleaseBump = "auto" | "patch" | "minor" | "major"
export type ReleaseMode = "dry-run" | "release" | "stop"
export type NoteSource = "changesets" | "git-history"

type Changeset = {
  bump: "patch" | "minor" | "major"
  path: string
  summary: string
}

type Receipt = {
  status: "ok" | "blocked" | "failed"
  reason?: string
  [key: string]: unknown
}

type ReleaseContext = Pick<Context, "askUser" | "runCommand" | "require" | "blocked" | "refused">

const controller = "node src/release-controller.mjs"
const fullTrain = "full-train"

function parseReceipt(ctx: ReleaseContext, claim: string, result: CommandResult): Receipt {
  ctx.require(result.exit_code === 0 && !result.timed_out, claim, result)
  let receipt: Receipt
  try {
    receipt = JSON.parse(result.stdout.trim()) as Receipt
  } catch {
    ctx.require(false, `${claim}: controller returned invalid JSON`, result)
    throw new Error("unreachable")
  }
  if (receipt.status === "blocked") ctx.blocked(receipt.reason ?? claim)
  ctx.require(receipt.status === "ok", receipt.reason ?? claim, receipt)
  return receipt
}

function command(
  ctx: ReleaseContext,
  id: string,
  args: string,
  claim: string,
  timeout = 600,
): Receipt {
  return parseReceipt(ctx, claim, ctx.runCommand(id, `${controller} ${args}`, timeout))
}

function stringField(ctx: ReleaseContext, receipt: Receipt, field: string): string {
  const value = receipt[field]
  ctx.require(
    typeof value === "string" && value.length > 0,
    `controller receipt contains ${field}`,
    receipt,
  )
  return value as string
}

function matchingField(
  ctx: ReleaseContext,
  receipt: Receipt,
  field: string,
  pattern: RegExp,
): string {
  const value = stringField(ctx, receipt, field)
  ctx.require(pattern.test(value), `controller receipt contains a valid ${field}`, receipt)
  return value
}

function countField(ctx: ReleaseContext, receipt: Receipt, field: string): number {
  const value = receipt[field]
  ctx.require(
    Number.isInteger(value) && (value as number) >= 0,
    `controller receipt contains a non-negative ${field}`,
    receipt,
  )
  return value as number
}

function changesetsField(ctx: ReleaseContext, receipt: Receipt): Changeset[] {
  const value = receipt.changesets
  ctx.require(Array.isArray(value), "controller receipt contains a Changeset list", receipt)
  for (const item of value as unknown[]) {
    const candidate = item as Partial<Changeset>
    ctx.require(
      typeof candidate?.path === "string" &&
        candidate.path.length > 0 &&
        typeof candidate.summary === "string" &&
        candidate.summary.length > 0 &&
        ["patch", "minor", "major"].includes(candidate.bump ?? ""),
      "every listed Changeset has a path, bump, and summary",
      receipt,
    )
  }
  return value as Changeset[]
}

function explicitBump(ctx: ReleaseContext): Exclude<ReleaseBump, "auto"> {
  const choice = ctx.askUser("select-explicit-bump", "Choose the explicit Yield release bump.", [
    { value: "patch", label: "Patch" },
    { value: "minor", label: "Minor" },
    { value: "major", label: "Major" },
    { value: "stop", label: "Stop" },
  ])
  if (choice === "stop") ctx.refused("release was stopped before preflight")
  ctx.require(["patch", "minor", "major"].includes(choice), "an explicit release bump is selected")
  return choice as Exclude<ReleaseBump, "auto">
}

function confirmHighImpactBump(ctx: ReleaseContext, bump: ReleaseBump) {
  if (bump !== "minor" && bump !== "major") return
  const confirmation = ctx.askUser(
    "confirm-high-impact-bump",
    `Confirm the ${bump} release intent before GitHub performs the protected dry run.`,
    [
      { value: "confirm", label: `Confirm ${bump}` },
      { value: "cancel", label: "Cancel" },
    ],
  )
  if (confirmation !== "confirm") ctx.refused(`${bump} release intent was not confirmed`)
}

export function runReleaseYield(ctx: ReleaseContext) {
  const mode = ctx.askUser("select-mode", "Choose how far this Yield release run may proceed.", [
    { value: "dry-run", label: "Dry run only" },
    { value: "release", label: "Prepare release" },
    { value: "stop", label: "Stop" },
  ]) as ReleaseMode
  if (mode === "stop") ctx.refused("release was not started")

  const scope = ctx.askUser(
    "select-release-scope",
    "Stable releases currently publish one verified Yield version to every public target.",
    [
      { value: fullTrain, label: "Release the full Yield train" },
      { value: "design-target-specific", label: "Stop and design target-specific releases" },
      { value: "stop", label: "Stop" },
    ],
  )
  if (scope === "design-target-specific")
    ctx.refused(
      "target-specific stable releases need a compatibility manifest and verification path",
    )
  if (scope === "stop") ctx.refused("release was stopped before input inspection")
  ctx.require(scope === fullTrain, "the selected stable release scope is the full Yield train")

  const input = command(
    ctx,
    "inspect-release-input",
    "inspect",
    "the release input is inspected before a decision",
  )
  const inputChangesets = changesetsField(ctx, input)

  let bump: ReleaseBump
  let notesSource: NoteSource
  if (inputChangesets.length) {
    const basis = ctx.askUser(
      "select-version-basis",
      `Found ${inputChangesets.length} pending Changeset(s). Choose how to set the release version.`,
      [
        { value: "changesets", label: "Use the Changeset bump" },
        { value: "explicit", label: "Select an explicit bump" },
        { value: "stop", label: "Stop" },
      ],
    )
    if (basis === "stop") ctx.refused("release was stopped before bump selection")
    bump = basis === "changesets" ? "auto" : explicitBump(ctx)
    const noteChoice = ctx.askUser(
      "select-note-source",
      "Choose the source for immutable GitHub release notes.",
      [
        { value: "changesets", label: "Use Changeset summaries" },
        { value: "git-history", label: "Use Git history" },
        { value: "stop", label: "Stop" },
      ],
    )
    if (noteChoice === "stop") ctx.refused("release was stopped before note selection")
    ctx.require(
      noteChoice === "changesets" || noteChoice === "git-history",
      "a release note source is selected",
    )
    notesSource = noteChoice as NoteSource
  } else {
    const noChangesets = ctx.askUser(
      "handle-no-changesets",
      "No pending Changesets were found. Select an explicit bump to release from the exact Git history, or stop.",
      [
        { value: "explicit", label: "Select an explicit bump" },
        { value: "stop", label: "Stop" },
      ],
    )
    if (noChangesets !== "explicit")
      ctx.refused("release was stopped because no Changeset was selected")
    bump = explicitBump(ctx)
    const historyNotes = ctx.askUser(
      "confirm-git-history-notes",
      "Generate immutable release notes from the exact base-tag-to-HEAD Git history?",
      [
        { value: "git-history", label: "Generate Git-history notes" },
        { value: "stop", label: "Stop" },
      ],
    )
    if (historyNotes !== "git-history")
      ctx.refused("release was stopped before Git-history notes were selected")
    notesSource = "git-history"
  }

  confirmHighImpactBump(ctx, bump)
  const preflight = command(
    ctx,
    "preflight",
    `preflight --bump ${bump}`,
    "the protected main preflight passes",
  )
  const sourceSha = matchingField(ctx, preflight, "source_sha", /^[0-9a-f]{40}$/)

  const dry = command(
    ctx,
    "dispatch-dry-run",
    `dispatch --bump ${bump} --notes-source ${notesSource} --dry-run true`,
    "the dry-run workflow is dispatched",
  )
  const dryRunID = matchingField(ctx, dry, "run_id", /^\d+$/)
  const dryResult = command(
    ctx,
    "wait-dry-run",
    `wait --run-id ${dryRunID}`,
    "the GitHub dry run succeeds",
    1800,
  )
  ctx.require(
    stringField(ctx, dryResult, "source_sha") === sourceSha,
    "the dry run uses the preflight source SHA",
    dryResult,
  )

  const plan = command(
    ctx,
    "resolve-plan",
    `plan --bump ${bump} --notes-source ${notesSource}`,
    "the local deterministic release plan resolves",
  )
  const version = matchingField(ctx, plan, "version", /^\d+\.\d+\.\d+$/)
  const tag = matchingField(ctx, plan, "tag", /^v\d+\.\d+\.\d+$/)
  const changesets = changesetsField(ctx, plan)
  const releaseBasis = matchingField(ctx, plan, "release_basis", /^(changesets|explicit-bump)$/)
  const baseTag = matchingField(ctx, plan, "base_tag", /^v\d+\.\d+\.\d+$/)
  const commitRange = stringField(ctx, plan, "commit_range")
  const commitCount = countField(ctx, plan, "commit_count")
  const planNotesSource = matchingField(ctx, plan, "notes_source", /^(changesets|git-history)$/)
  const targetContract = matchingField(ctx, plan, "target_contract", /^full-train$/)
  ctx.require(tag === `v${version}`, "the release tag matches the planned version", plan)
  ctx.require(
    stringField(ctx, plan, "source_sha") === sourceSha,
    "the displayed plan uses the dry-run source SHA",
    plan,
  )
  ctx.require(
    planNotesSource === notesSource,
    "the displayed plan uses the selected note source",
    plan,
  )
  ctx.require(targetContract === fullTrain, "the displayed plan keeps the full Yield train", plan)
  ctx.require(
    commitRange === `${baseTag}..${sourceSha}`,
    "the plan binds the exact Git history range",
    plan,
  )
  if (releaseBasis === "changesets")
    ctx.require(changesets.length > 0, "a Changeset-based plan contains pending Changesets", plan)
  if (releaseBasis === "explicit-bump")
    ctx.require(commitCount > 0, "an explicit release contains commits after the base tag", plan)

  const releaseSummary =
    planNotesSource === "changesets"
      ? `${changesets.length} Changeset(s)`
      : `Git history ${commitRange} (${commitCount} commits)`
  if (mode === "dry-run") {
    return {
      mode,
      bump,
      release_basis: releaseBasis,
      note_source: planNotesSource,
      base_tag: baseTag,
      commit_range: commitRange,
      target_contract: targetContract,
      version,
      tag,
      source_sha: sourceSha,
      changesets,
      dry_run: { id: dryRunID, url: dry.run_url },
    }
  }

  const authorization = ctx.askUser(
    "authorize-release",
    `Dry run passed for ${tag} from ${sourceSha}. Basis: ${releaseBasis}; notes: ${releaseSummary}; targets: full Yield train. Continue with the protected release?`,
    [
      { value: "release", label: `Release ${tag}` },
      { value: "stop", label: "Stop" },
    ],
  )
  if (authorization !== "release") ctx.refused(`release of ${tag} was not authorized`)

  const live = command(
    ctx,
    "dispatch-release",
    `dispatch --bump ${bump} --notes-source ${notesSource} --dry-run false`,
    "the protected release workflow is dispatched",
  )
  ctx.require(
    stringField(ctx, live, "source_sha") === sourceSha,
    "the live dispatch uses the authorized source SHA",
    live,
  )
  const releaseRunID = matchingField(ctx, live, "run_id", /^\d+$/)
  const publisherBaseline = matchingField(
    ctx,
    live,
    "publisher_baseline",
    /^(?:none|\d+(?:,\d+)*)$/,
  )
  const finalizerBaseline = matchingField(
    ctx,
    live,
    "finalizer_baseline",
    /^(?:none|\d+(?:,\d+)*)$/,
  )

  const release = command(
    ctx,
    "wait-release-control",
    `monitor-controller --run-id ${releaseRunID} --publisher-baseline ${publisherBaseline}`,
    "release-control approves and the controller dispatches the publisher",
    3600,
  )
  const publisherRunID = matchingField(ctx, release, "publisher_run_id", /^\d+$/)

  command(
    ctx,
    "wait-publishers",
    `monitor-publisher --run-id ${publisherRunID}`,
    "npm, PyPI, and crates.io publishers complete",
    3600,
  )

  const finalized = command(
    ctx,
    "wait-finalizer",
    `monitor-finalizer --baseline ${finalizerBaseline}`,
    "the release finalizer completes",
    1800,
  )
  const finalizerRunID = matchingField(ctx, finalized, "run_id", /^\d+$/)

  const verified = command(
    ctx,
    "verify-public-release",
    `verify --version ${version} --tag ${tag} --source-sha ${sourceSha}`,
    "every public release target matches the authorized release",
    1800,
  )

  return {
    mode: "release",
    bump,
    release_basis: releaseBasis,
    note_source: planNotesSource,
    base_tag: baseTag,
    commit_range: commitRange,
    target_contract: targetContract,
    version,
    tag,
    source_sha: sourceSha,
    changesets,
    dry_run: { id: dryRunID, url: dry.run_url },
    release_controller: { id: releaseRunID, url: live.run_url },
    publisher: { id: publisherRunID, url: release.publisher_run_url },
    finalizer: { id: finalizerRunID, url: finalized.run_url },
    verified: verified.targets,
  }
}

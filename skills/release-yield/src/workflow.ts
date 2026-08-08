import type { CommandResult, Context } from "@operatorstack/yield"

export type ReleaseBump = "auto" | "patch" | "minor" | "major"
export type ReleaseMode = "dry-run" | "release"

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

function changesetsField(ctx: ReleaseContext, receipt: Receipt): Changeset[] {
  const value = receipt.changesets
  ctx.require(
    Array.isArray(value) && value.length > 0,
    "the release plan contains at least one Changeset",
    receipt,
  )
  for (const item of value as unknown[]) {
    const candidate = item as Partial<Changeset>
    ctx.require(
      typeof candidate?.path === "string" &&
        candidate.path.length > 0 &&
        typeof candidate.summary === "string" &&
        candidate.summary.length > 0 &&
        ["patch", "minor", "major"].includes(candidate.bump ?? ""),
      "every planned Changeset has a path, bump, and summary",
      receipt,
    )
  }
  return value as Changeset[]
}

export function runReleaseYield(ctx: ReleaseContext) {
  const mode = ctx.askUser("select-mode", "Choose how far this Yield release run may proceed.", [
    { value: "dry-run", label: "Dry run only" },
    { value: "release", label: "Prepare release" },
  ]) as ReleaseMode

  const bump = ctx.askUser("select-bump", "Choose the Yield release bump.", [
    { value: "auto", label: "Use Changesets" },
    { value: "patch", label: "Patch" },
    { value: "minor", label: "Minor" },
    { value: "major", label: "Major" },
  ]) as ReleaseBump

  if (bump === "minor" || bump === "major") {
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
    `dispatch --bump ${bump} --dry-run true`,
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
    `plan --bump ${bump}`,
    "the local deterministic release plan resolves",
  )
  const version = matchingField(ctx, plan, "version", /^\d+\.\d+\.\d+$/)
  const tag = matchingField(ctx, plan, "tag", /^v\d+\.\d+\.\d+$/)
  const changesets = changesetsField(ctx, plan)
  ctx.require(tag === `v${version}`, "the release tag matches the planned version", plan)
  ctx.require(
    stringField(ctx, plan, "source_sha") === sourceSha,
    "the displayed plan uses the dry-run source SHA",
    plan,
  )

  if (mode === "dry-run") {
    return {
      mode,
      bump,
      version,
      tag,
      source_sha: sourceSha,
      changesets,
      dry_run: { id: dryRunID, url: dry.run_url },
    }
  }

  const changesetSummary = changesets
    .map((item) => `${item.path} (${item.bump}): ${item.summary}`)
    .join("; ")
  const authorization = ctx.askUser(
    "authorize-release",
    `Dry run passed for ${tag} from ${sourceSha}. Changesets: ${changesetSummary}. Continue with the protected release?`,
    [
      { value: "release", label: `Release ${tag}` },
      { value: "stop", label: "Stop" },
    ],
  )
  if (authorization !== "release") ctx.refused(`release of ${tag} was not authorized`)

  const live = command(
    ctx,
    "dispatch-release",
    `dispatch --bump ${bump} --dry-run false`,
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

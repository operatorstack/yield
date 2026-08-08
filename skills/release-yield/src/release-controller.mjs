#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { realpath } from "node:fs/promises";
import { resolve } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const repository = "operatorstack/yield";
const root = resolve(import.meta.dirname, "../../..");
const bumps = new Set(["auto", "patch", "minor", "major"]);
const active = new Set(["queued", "in_progress", "pending", "waiting", "requested"]);
const npmPackages = [
  "@operatorstack/yield",
  "@operatorstack/create-yield",
  "@operatorstack/yield-darwin-amd64",
  "@operatorstack/yield-darwin-arm64",
  "@operatorstack/yield-linux-amd64",
  "@operatorstack/yield-linux-arm64",
  "@operatorstack/yield-windows-amd64",
  "@operatorstack/yield-windows-arm64",
];
const crates = [
  "yieldskill",
  "yieldskill-runtime-darwin-amd64",
  "yieldskill-runtime-darwin-arm64",
  "yieldskill-runtime-linux-amd64",
  "yieldskill-runtime-linux-arm64",
  "yieldskill-runtime-windows-amd64",
  "yieldskill-runtime-windows-arm64",
];

export class Blocked extends Error {}
export class Failed extends Error {}

function parseArgs(argv) {
  const [action, ...rest] = argv;
  if (!action) throw new Failed("an action is required");
  const values = {};
  for (let index = 0; index < rest.length; index += 2) {
    if (!rest[index]?.startsWith("--") || rest[index + 1] === undefined) throw new Failed(`invalid argument ${rest[index] ?? ""}`);
    values[rest[index].slice(2)] = rest[index + 1];
  }
  return { action, values };
}

function run(file, args, options = {}) {
  return execFileSync(file, args, { cwd: root, encoding: "utf8", stdio: [options.input ? "pipe" : "ignore", "pipe", "pipe"], ...options }).trim();
}

const git = (...args) => run("git", args);
const gh = (...args) => run("gh", args);
const ghJSON = (...args) => JSON.parse(gh(...args));
const ghInput = (args, input) => run("gh", args, { input });
const sleep = (milliseconds) => new Promise((resolveSleep) => setTimeout(resolveSleep, milliseconds));

export function selectNewRun(runs, baseline, { event, sourceSha } = {}) {
  const previous = new Set(String(baseline || "").split(",").filter(Boolean));
  const candidates = runs.filter((run) => !previous.has(String(run.databaseId)))
    .filter((run) => !event || run.event === event)
    .filter((run) => !sourceSha || run.headSha === sourceSha);
  if (candidates.length > 1) throw new Blocked("multiple matching workflow runs appeared; refusing ambiguous correlation");
  return candidates[0] ?? null;
}

function listRuns(workflow) {
  return ghJSON("run", "list", "--repo", repository, "--workflow", workflow, "--limit", "50", "--json", "databaseId,status,conclusion,headSha,event,createdAt,url,displayTitle");
}

const baseline = (workflow) => listRuns(workflow).map((run) => run.databaseId).join(",") || "none";

async function discover(workflow, previous, options = {}) {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    const found = selectNewRun(listRuns(workflow), previous === "none" ? "" : previous, options);
    if (found) return found;
    await sleep(2000);
  }
  throw new Blocked(`no new ${workflow} run appeared`);
}

function requireBump(value) {
  if (!bumps.has(value)) throw new Failed(`invalid release bump ${value ?? "missing"}`);
  return value;
}

function normalizeRemote(value) {
  return value.replace(/^git@github\.com:/, "https://github.com/").replace(/\.git$/, "");
}

function pendingDeployments(runID) {
  return ghJSON("api", `repos/${repository}/actions/runs/${runID}/pending_deployments`);
}

function approve(runID, deployments, expected) {
  if (!deployments.length) return [];
  const names = deployments.map((item) => item.environment?.name);
  const unknown = names.filter((name) => !expected.has(name));
  if (unknown.length) throw new Failed(`unexpected protected environment: ${unknown.join(", ")}`);
  const ids = deployments.map((item) => item.environment?.id);
  try {
    ghInput(
      ["api", "--method", "POST", `repos/${repository}/actions/runs/${runID}/pending_deployments`, "--input", "-"],
      JSON.stringify({ environment_ids: ids, state: "approved", comment: "Authorized by the recorded release-yield decision." }),
    );
  } catch {
    throw new Blocked(`GitHub refused approval for ${names.join(", ")}; approve the environments at the workflow run`);
  }
  return names;
}

async function waitRun(runID, expectedEnvironments = new Set()) {
  const seen = new Set();
  for (let attempt = 0; attempt < 1800; attempt += 1) {
    for (const name of approve(runID, pendingDeployments(runID), expectedEnvironments)) seen.add(name);
    const runInfo = ghJSON("run", "view", String(runID), "--repo", repository, "--json", "databaseId,status,conclusion,headSha,createdAt,updatedAt,url");
    if (runInfo.status === "completed") {
      if (runInfo.conclusion !== "success") throw new Failed(`workflow run ${runID} concluded ${runInfo.conclusion}`);
      return { ...runInfo, environments: [...seen].sort() };
    }
    await sleep(2000);
  }
  throw new Blocked(`workflow run ${runID} did not complete before the timeout`);
}

async function preflight(values) {
  requireBump(values.bump);
  if (normalizeRemote(git("remote", "get-url", "origin")) !== `https://github.com/${repository}`) throw new Blocked(`origin is not ${repository}`);
  if (git("branch", "--show-current") !== "main") throw new Blocked("release-yield must run from the main branch");
  if (git("status", "--porcelain")) throw new Blocked("the worktree is not clean");
  git("fetch", "origin", "main");
  const sourceSha = git("rev-parse", "HEAD");
  if (git("rev-parse", "origin/main") !== sourceSha) throw new Blocked("main does not match origin/main");
  let protection;
  try {
    gh("auth", "status");
    protection = ghJSON("api", `repos/${repository}/branches/main/protection`);
  } catch {
    throw new Blocked("GitHub authentication cannot inspect the protected main branch");
  }
  if (!protection?.enforce_admins?.enabled || !protection?.required_status_checks?.strict) throw new Blocked("main is not protected with strict required checks");
  const conflicting = [...listRuns("release.yml"), ...listRuns("npm-publish.yml")].filter((item) => active.has(item.status));
  if (conflicting.length) throw new Blocked(`another release workflow is active: ${conflicting[0].url}`);
  return { source_sha: sourceSha, protected_main: true };
}

async function dispatch(values) {
  const bump = requireBump(values.bump);
  if (values["dry-run"] !== "true" && values["dry-run"] !== "false") throw new Failed("--dry-run must be true or false");
  const sourceSha = git("rev-parse", "HEAD");
  if (git("branch", "--show-current") !== "main" || git("status", "--porcelain")) throw new Blocked("main changed after preflight");
  if (git("rev-parse", "origin/main") !== sourceSha) throw new Blocked("source SHA changed after preflight");
  const conflicting = [...listRuns("release.yml"), ...listRuns("npm-publish.yml")].filter((item) => active.has(item.status));
  if (conflicting.length) throw new Blocked(`another release workflow is active: ${conflicting[0].url}`);
  const releaseBaseline = baseline("release.yml");
  const publisherBaseline = baseline("npm-publish.yml");
  const finalizerBaseline = baseline("release-finalize.yml");
  try {
    gh("workflow", "run", "release.yml", "--repo", repository, "--ref", "main", "-f", `bump=${bump}`, "-f", `dry_run=${values["dry-run"]}`);
  } catch {
    throw new Blocked("GitHub refused the release workflow dispatch");
  }
  const found = await discover("release.yml", releaseBaseline, { event: "workflow_dispatch", sourceSha });
  return {
    source_sha: sourceSha,
    run_id: String(found.databaseId),
    run_url: found.url,
    publisher_baseline: publisherBaseline,
    finalizer_baseline: finalizerBaseline,
  };
}

async function plan(values) {
  const bump = requireBump(values.bump);
  const value = JSON.parse(run("node", ["scripts/release-plan.mjs", "--bump", bump]));
  return { version: value.version, tag: `v${value.version}`, source_sha: value.sourceSha, changesets: value.changesets };
}

async function monitorController(values) {
  const info = await waitRun(values["run-id"], new Set(["release-control"]));
  const publisher = await discover("npm-publish.yml", values["publisher-baseline"] === "none" ? "" : values["publisher-baseline"], { event: "workflow_dispatch" });
  return { run_id: String(info.databaseId), run_url: info.url, publisher_run_id: String(publisher.databaseId), publisher_run_url: publisher.url, environments: info.environments };
}

async function monitorPublisher(values) {
  const info = await waitRun(values["run-id"], new Set(["npm-production", "pypi-production", "crates-production"]));
  return { run_id: String(info.databaseId), run_url: info.url, environments: info.environments };
}

async function monitorFinalizer(values) {
  const found = await discover("release-finalize.yml", values.baseline === "none" ? "" : values.baseline, { event: "workflow_run" });
  const info = await waitRun(String(found.databaseId));
  return { run_id: String(info.databaseId), run_url: info.url };
}

async function verify(values) {
  const { version, tag } = values;
  const sourceSha = values["source-sha"];
  if (!/^\d+\.\d+\.\d+$/.test(version ?? "") || tag !== `v${version}` || !/^[0-9a-f]{40}$/.test(sourceSha ?? "")) throw new Failed("invalid release identity");
  for (const name of npmPackages) {
    const found = JSON.parse(run("npm", ["view", `${name}@${version}`, "version", "--json"]));
    if (found !== version) throw new Failed(`${name}@${version} is missing from npm`);
  }
  const python = await (await fetch(`https://pypi.org/pypi/yieldskill/${version}/json`)).json();
  if (python?.info?.version !== version || !Array.isArray(python.urls) || python.urls.length !== 6 || python.urls.some((file) => !file?.digests?.sha256)) throw new Failed(`yieldskill ${version} is incomplete on PyPI`);
  for (const name of crates) {
    const response = await fetch(`https://crates.io/api/v1/crates/${name}/${version}`);
    const body = await response.json();
    if (!response.ok || body?.version?.num !== version) throw new Failed(`${name}@${version} is missing from crates.io`);
  }
  run("node", ["packaging/go-release.mjs", "--version", version, "--source-sha", sourceSha, "--attempts", "3", "--delay-ms", "10000"]);
  const remoteTag = git("ls-remote", "--tags", "origin", `refs/tags/${tag}`).split(/\s+/)[0];
  if (remoteTag !== sourceSha) throw new Failed(`${tag} does not point to the authorized source SHA`);
  const release = ghJSON("release", "view", tag, "--repo", repository, "--json", "isDraft,tagName,url,targetCommitish");
  if (release.isDraft || release.tagName !== tag) throw new Failed(`${tag} is not a finalized GitHub release`);
  return {
    targets: {
      npm: npmPackages.length,
      pypi: { project: "yieldskill", wheels: python.urls.length },
      crates: crates.length,
      go: "github.com/operatorstack/yield",
      github: release.url,
    },
  };
}

export async function execute(action, values) {
  if (action === "preflight") return preflight(values);
  if (action === "dispatch") return dispatch(values);
  if (action === "wait") {
    const info = await waitRun(values["run-id"]);
    return { run_id: String(info.databaseId), run_url: info.url, source_sha: info.headSha };
  }
  if (action === "plan") return plan(values);
  if (action === "monitor-controller") return monitorController(values);
  if (action === "monitor-publisher") return monitorPublisher(values);
  if (action === "monitor-finalizer") return monitorFinalizer(values);
  if (action === "verify") return verify(values);
  throw new Failed(`unknown action ${action}`);
}

async function main() {
  try {
    const { action, values } = parseArgs(process.argv.slice(2));
    process.stdout.write(`${JSON.stringify({ status: "ok", ...(await execute(action, values)) })}\n`);
  } catch (error) {
    const status = error instanceof Blocked ? "blocked" : "failed";
    process.stdout.write(`${JSON.stringify({ status, reason: error instanceof Error ? error.message : String(error) })}\n`);
  }
}

if (process.argv[1] && await realpath(process.argv[1]) === await realpath(fileURLToPath(import.meta.url))) await main();

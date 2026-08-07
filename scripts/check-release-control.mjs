#!/usr/bin/env node
import { readdir, readFile, stat } from "node:fs/promises";
import { resolve } from "node:path";
import process from "node:process";
import { pathToFileURL } from "node:url";
import { parse } from "yaml";

const SHA_REF = /^[^\s@]+@[0-9a-f]{40}$/;

function expect(condition, message) {
  if (!condition) throw new Error(message);
}

function usesIn(value, found = []) {
  if (Array.isArray(value)) {
    for (const item of value) usesIn(item, found);
  } else if (value && typeof value === "object") {
    for (const [key, item] of Object.entries(value)) {
      if (key === "uses" && typeof item === "string") found.push(item);
      else usesIn(item, found);
    }
  }
  return found;
}

async function exists(path) {
  return stat(path).then(() => true, () => false);
}

export async function checkReleaseControl(root = resolve(import.meta.dirname, "..")) {
  const workflowDir = resolve(root, ".github/workflows");
  const names = (await readdir(workflowDir)).filter((name) => name.endsWith(".yml") || name.endsWith(".yaml"));
  const workflows = {};
  const raw = {};
  for (const name of names) {
    raw[name] = await readFile(resolve(workflowDir, name), "utf8");
    workflows[name] = parse(raw[name]);
    for (const action of usesIn(workflows[name])) {
      expect(action.startsWith("./") || SHA_REF.test(action), `${name}: action is not pinned to a full commit SHA: ${action}`);
    }
  }

  expect(!(await exists(resolve(root, "UPSTREAM.json"))), "UPSTREAM.json must be removed after graduation");
  expect(!names.includes("sync-upstream.yml"), "projection sync workflow must be removed after graduation");

  const verify = workflows["verify.yml"];
  const validationJobs = ["go", "release", "typescript", "python", "rust", "conformance", "examples"];
  expect(verify, "verify.yml is required");
  expect(verify.on?.pull_request !== undefined, "verification must run on every pull request");
  expect(validationJobs.every((name) => verify.jobs?.[name]), "verification must expose every SDK and package boundary");
  expect(
    JSON.stringify([...(verify.jobs?.validate?.needs ?? [])].sort()) === JSON.stringify([...validationJobs].sort()),
    "the final validation gate must depend on every visible validation job",
  );
  expect(verify.jobs?.validate?.name === "Release authority and full validation", "the protected validation context must remain stable");

  const release = workflows["release.yml"];
  expect(release, "release.yml is required");
  expect(JSON.stringify(Object.keys(release.on ?? {}).sort()) === JSON.stringify(["workflow_dispatch"]), "stable release must be dispatch-only");
  expect(release.permissions?.contents === "read", "release planning must be read-only");
  expect(release.jobs?.release?.permissions?.contents === "write", "tag creation alone needs contents:write");
  expect(release.jobs?.release?.environment === "release-control", "release authorization must use the protected release-control environment");
  expect(raw["release.yml"].includes('repos/$GITHUB_REPOSITORY/git/refs'), "release controller must create tags with its scoped GitHub token");
  expect(!raw["release.yml"].includes('git push origin "refs/tags/$TAG"'), "release controller must not push tags without explicit authentication");
  expect(raw["release.yml"].includes("--draft"), "release controller must create a draft release");
  expect(!raw["release.yml"].includes("--draft=false"), "release controller must not finalize its own release");

  const npm = workflows["npm-publish.yml"];
  expect(npm, "npm-publish.yml is required");
  expect(npm.permissions?.contents === "read" && npm.permissions?.["id-token"] === "write", "npm publisher must use read-only source plus OIDC");
  expect(npm.on?.push?.branches?.includes("main"), "npm canary must follow public main");
  expect(npm.on?.workflow_run?.workflows?.includes("Release Yield"), "stable npm must consume the release controller receipt");
  expect(raw["npm-publish.yml"].indexOf("Publish platform runtimes") < raw["npm-publish.yml"].indexOf("Publish SDK and CLI"), "runtime packages must publish before the SDK package");

  const privateRegistry = workflows["private-registry.yml"];
  expect(privateRegistry && !privateRegistry.on?.push, "private stable publishing must not accept direct tag pushes");
  expect(privateRegistry.jobs?.publish?.environment === "private-production", "private publishing must use its protected environment");

  const finalizer = workflows["release-finalize.yml"];
  expect(finalizer?.permissions?.actions === "read" && finalizer.permissions?.contents === "read", "finalizer preflight must be read-only");
  expect(finalizer.jobs?.finalize?.permissions?.contents === "write", "receipt-complete finalization alone needs contents:write");
  expect(finalizer.jobs?.finalize?.needs === "resolve", "finalization must follow read-only tag resolution");
  expect(raw["release-finalize.yml"].includes("--draft=false"), "only the receipt finalizer may publish the GitHub release");
  expect(!raw["release-finalize.yml"].includes("private-registry.yml"), "the private mirror must not block public release finalization");

  for (const [name, text] of Object.entries(raw)) {
    expect(!/NPM_TOKEN|NODE_AUTH_TOKEN|secrets\.npm/i.test(text), `${name}: long-lived npm credentials are forbidden`);
  }

  return { workflows: names.length, externalActionsPinned: names.flatMap((name) => usesIn(workflows[name])).filter((ref) => !ref.startsWith("./")).length };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  checkReleaseControl()
    .then((result) => console.log(`release-control: ${result.workflows} workflows and ${result.externalActionsPinned} pinned action references verified`))
    .catch((error) => { console.error(`release-control: ${error.message}`); process.exit(1); });
}

#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import process from "node:process";
import { pathToFileURL } from "node:url";
import { parse as parseYaml } from "yaml";

const PACKAGE = "@operatorstack/yield";
const levels = { patch: 0, minor: 1, major: 2 };

function parseArgs(argv) {
  const result = {};
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    if (!key?.startsWith("--") || argv[index + 1] === undefined) throw new Error(`invalid argument ${key ?? ""}`);
    result[key.slice(2)] = argv[index + 1];
  }
  return result;
}

function git(args) {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

export function parseChangeset(text, path = "changeset") {
  const match = /^---\r?\n([\s\S]*?)\r?\n---\r?\n([\s\S]+)$/.exec(text);
  if (!match) throw new Error(`${path}: expected YAML frontmatter and a summary`);
  const releases = parseYaml(match[1]);
  if (!releases || typeof releases !== "object" || Array.isArray(releases)) throw new Error(`${path}: frontmatter must be a package map`);
  const entries = Object.entries(releases);
  if (entries.length !== 1 || entries[0][0] !== PACKAGE) throw new Error(`${path}: only ${PACKAGE} may declare release intent`);
  const bump = entries[0][1];
  if (!(bump in levels)) throw new Error(`${path}: bump must be patch, minor, or major`);
  const summary = match[2].trim();
  if (!summary) throw new Error(`${path}: summary must not be empty`);
  return { bump, summary, path };
}

export function bumpVersion(version, bump) {
  if (!/^\d+\.\d+\.\d+$/.test(version)) throw new Error(`invalid base version ${version}`);
  const [major, minor, patch] = version.split(".").map(Number);
  if (bump === "major") return `${major + 1}.0.0`;
  if (bump === "minor") return `${major}.${minor + 1}.0`;
  if (bump === "patch") return `${major}.${minor}.${patch + 1}`;
  throw new Error(`invalid bump ${bump}`);
}

export function planRelease({ baseVersion, changesets, requestedBump = "auto" }) {
  if (!changesets.length) throw new Error("stable releases require at least one pending Changeset");
  if (requestedBump !== "auto" && !(requestedBump in levels)) throw new Error(`invalid requested bump ${requestedBump}`);
  const declaredBump = changesets.map(({ bump }) => bump).sort((a, b) => levels[b] - levels[a])[0];
  if (requestedBump !== "auto" && levels[requestedBump] < levels[declaredBump]) {
    throw new Error(`requested ${requestedBump} cannot lower declared ${declaredBump}`);
  }
  const bump = requestedBump === "auto" ? declaredBump : requestedBump;
  return { baseVersion, bump, version: bumpVersion(baseVersion, bump), changesets };
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const requestedBump = args.bump ?? "auto";
  const baseTag = args.base ?? git(["tag", "--list", "v[0-9]*", "--sort=-v:refname"]).split("\n")[0];
  if (!/^v\d+\.\d+\.\d+$/.test(baseTag)) throw new Error("no valid stable base tag found");
  git(["rev-parse", "--verify", `refs/tags/${baseTag}`]);
  const paths = git(["diff", "--name-only", "--diff-filter=A", `${baseTag}..HEAD`, "--", ".changeset/*.md"])
    .split("\n")
    .filter((path) => path && path !== ".changeset/README.md");
  const changesets = [];
  for (const path of paths) changesets.push(parseChangeset(await readFile(path, "utf8"), path));
  const plan = planRelease({ baseVersion: baseTag.slice(1), changesets, requestedBump });
  const sourceSha = git(["rev-parse", "HEAD"]);
  const notes = [`# Yield ${plan.version}`, "", ...plan.changesets.flatMap(({ summary }) => [`- ${summary}`, ""])].join("\n").trimEnd() + "\n";
  if (args.notes) await writeFile(args.notes, notes);
  if (args.output) {
    await writeFile(args.output, [
      `base_tag=${baseTag}`,
      `bump=${plan.bump}`,
      `version=${plan.version}`,
      `tag=v${plan.version}`,
      `source_sha=${sourceSha}`,
      `changeset_count=${plan.changesets.length}`,
      "",
    ].join("\n"), { flag: "a" });
  }
  process.stdout.write(`${JSON.stringify({ ...plan, baseTag, sourceSha }, null, 2)}\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => { console.error(`release-plan: ${error.message}`); process.exit(1); });
}

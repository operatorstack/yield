#!/usr/bin/env node
import process from "node:process";
import { npmPackage, rustPackage, targets } from "./targets.mjs";

function parseArgs(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 2) {
    values[argv[index]?.replace(/^--/, "")] = argv[index + 1];
  }
  const versions = (values.versions ?? "").split(",").filter(Boolean);
  if (!versions.length || versions.some((version) => !/^\d+\.\d+\.\d+$/.test(version))) {
    throw new Error("--versions must be a comma-separated semver list");
  }
  return {
    versions: [...new Set(versions)],
    base: (values.base ?? "https://get.operatorstack.systems").replace(/\/$/, ""),
  };
}

async function fetchText(fetchImpl, url) {
  const response = await fetchImpl(url);
  if (!response.ok) throw new Error(`${url}: HTTP ${response.status}`);
  return response.text();
}

async function fetchJSON(fetchImpl, url) {
  return JSON.parse(await fetchText(fetchImpl, url));
}

function npmPath(name) {
  return name.replace("/", "%2F");
}

function cargoPath(name) {
  const lower = name.toLowerCase();
  if (lower.length === 1) return `1/${lower}`;
  if (lower.length === 2) return `2/${lower}`;
  if (lower.length === 3) return `3/${lower[0]}/${lower}`;
  return `${lower.slice(0, 2)}/${lower.slice(2, 4)}/${lower}`;
}

function cargoVersions(text, expectedName) {
  const versions = new Set();
  for (const line of text.split("\n").filter(Boolean)) {
    const record = JSON.parse(line);
    if (record.name !== expectedName) throw new Error(`Cargo index for ${expectedName} contains ${record.name}`);
    if (/^\d+\.\d+\.\d+$/.test(record.vers)) versions.add(record.vers);
  }
  return versions;
}

export async function verifyRegistryHistory({ versions, base, fetchImpl = fetch }) {
  const missing = [];
  const npmNames = ["@operatorstack/yield", ...targets.map(npmPackage)];
  for (const name of npmNames) {
    const packument = await fetchJSON(fetchImpl, `${base}/npm/${npmPath(name)}`);
    const available = new Set(Object.keys(packument.versions ?? {}));
    for (const version of versions) {
      if (!available.has(version)) missing.push(`npm ${name}@${version}`);
    }
  }

  const pythonIndex = await fetchText(fetchImpl, `${base}/pip/simple/yieldskill/`);
  for (const version of versions) {
    for (const target of targets) {
      const filename = `yieldskill-${version}-py3-none-${target.pythonTag}.whl`;
      if (!pythonIndex.includes(filename)) missing.push(`Python ${filename}`);
    }
  }

  const goVersions = new Set((await fetchText(fetchImpl, `${base}/go/github.com/operatorstack/yield/@v/list`)).split(/\s+/).filter(Boolean));
  for (const version of versions) {
    if (!goVersions.has(`v${version}`)) missing.push(`Go github.com/operatorstack/yield@v${version}`);
  }

  const rustNames = ["yieldskill", ...targets.map(rustPackage)];
  for (const name of rustNames) {
    const available = cargoVersions(await fetchText(fetchImpl, `${base}/cargo/index/${cargoPath(name)}`), name);
    for (const version of versions) {
      if (!available.has(version)) missing.push(`Cargo ${name}@${version}`);
    }
  }

  if (missing.length) {
    throw new Error(`registry history is incomplete:\n  - ${missing.join("\n  - ")}`);
  }
  return { versions, languages: ["typescript", "python", "go", "rust"], targets: targets.map((target) => target.id) };
}

if (process.argv[1] && import.meta.filename === process.argv[1]) {
  verifyRegistryHistory({ ...parseArgs(process.argv.slice(2)) })
    .then((result) => console.log(`verified ${result.versions.length} release versions across 4 SDKs and ${result.targets.length} targets`))
    .catch((error) => { console.error(`verify-registry-history: ${error.message}`); process.exit(1); });
}

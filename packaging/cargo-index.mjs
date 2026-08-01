#!/usr/bin/env node
import { readFile, writeFile } from "node:fs/promises";
import process from "node:process";
import { resolve } from "node:path";
import { rustPackage, targets } from "./targets.mjs";

const cratesIo = "https://github.com/rust-lang/crates.io-index";
const operatorstack = "sparse+https://get.operatorstack.systems/cargo/index/";

function dependency(name, req, { features = [], target = null, registry = cratesIo } = {}) {
  return { name, req, features, optional: false, default_features: true, target, kind: "normal", registry };
}

export function record(name, version, checksum) {
  const deps = name === "yieldskill" ? [
    dependency("hex", "^0.4"),
    dependency("serde", "^1", { features: ["derive"] }),
    dependency("serde_json", "^1"),
    dependency("sha2", "^0.10"),
    ...targets.map((target) => dependency(rustPackage(target), `=${version}`, {
      target: `cfg(all(target_os = "${target.rustOs}", target_arch = "${target.rustArch}"))`,
      registry: operatorstack,
    })),
  ] : [];
  return { name, vers: version, deps, cksum: checksum, features: {}, yanked: false, links: null };
}

function args(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 2) values[argv[index]?.replace(/^--/, "")] = argv[index + 1];
  if (!values.name || !values.version || !/^[a-f0-9]{64}$/.test(values.checksum ?? "") || !values.output) {
    throw new Error("--name, --version, --checksum, and --output are required");
  }
  return values;
}

async function main() {
  const values = args(process.argv.slice(2));
  const output = resolve(values.output);
  const current = await readFile(output, "utf8").catch(() => "");
  const lines = current.split("\n").filter(Boolean).filter((line) => JSON.parse(line).vers !== values.version);
  lines.push(JSON.stringify(record(values.name, values.version, values.checksum)));
  await writeFile(output, `${lines.join("\n")}\n`);
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(import.meta.filename)) {
  main().catch((error) => { console.error(`cargo-index: ${error.message}`); process.exit(1); });
}

#!/usr/bin/env node
import { createHash } from "node:crypto"
import { readdir, readFile } from "node:fs/promises"
import { join, resolve } from "node:path"
import process from "node:process"
import { rustPackage, targets } from "./targets.mjs"

export const crateNames = [...targets.map(rustPackage), "yieldskill"]

export function indexPath(name) {
  const normalized = name.toLowerCase()
  if (normalized.length === 1) return `1/${normalized}`
  if (normalized.length === 2) return `2/${normalized}`
  if (normalized.length === 3) return `3/${normalized[0]}/${normalized}`
  return `${normalized.slice(0, 2)}/${normalized.slice(2, 4)}/${normalized}`
}

export async function registryRecord(name, version, fetchImpl = fetch) {
  const response = await fetchImpl(`https://index.crates.io/${indexPath(name)}`, {
    headers: { "User-Agent": "operatorstack-yield-release/1" },
  })
  if (response.status === 404) return null
  if (!response.ok) throw new Error(`crates.io index returned HTTP ${response.status} for ${name}`)
  for (const line of (await response.text()).split("\n").filter(Boolean)) {
    const record = JSON.parse(line)
    if (record.vers === version) return record
  }
  return null
}

function parseArgs(argv) {
  const values = {}
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index]
    if (!key?.startsWith("--") || argv[index + 1] === undefined)
      throw new Error(`invalid argument ${key ?? ""}`)
    values[key.slice(2)] = argv[index + 1]
  }
  if (!/^\d+\.\d+\.\d+$/.test(values.version ?? ""))
    throw new Error("--version must be stable semver")
  return values
}

function field(manifest, name) {
  return manifest.match(new RegExp(`^${name}\\s*=\\s*"([^"]+)"`, "m"))?.[1] ?? ""
}

export async function inspectRelease({ version, rust }) {
  const root = resolve(rust)
  const manifests = []
  for (const target of targets) manifests.push(join(root, "runtime", target.id, "Cargo.toml"))
  manifests.push(join(root, "yieldskill", "Cargo.toml"))
  const seen = []
  for (const manifestPath of manifests) {
    const manifest = await readFile(manifestPath, "utf8")
    const name = field(manifest, "name")
    if (!crateNames.includes(name)) throw new Error(`${manifestPath}: unexpected crate ${name}`)
    if (field(manifest, "version") !== version)
      throw new Error(`${name}: expected version ${version}`)
    if (field(manifest, "license") !== "MIT")
      throw new Error(`${name}: MIT license metadata is required`)
    if (field(manifest, "repository") !== "https://github.com/operatorstack/yield")
      throw new Error(`${name}: canonical repository is required`)
    if (/registry\s*=|get\.operatorstack\.systems/.test(manifest))
      throw new Error(`${name}: private registry configuration is forbidden`)
    if (name === "yieldskill") {
      for (const runtime of targets.map(rustPackage)) {
        const escaped = runtime.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
        const dependency = new RegExp(
          `^${escaped}\\s*=\\s*\\{\\s*version\\s*=\\s*"=${version.replace(/\./g, "\\.")}"\\s*\\}$`,
          "m",
        )
        if (!dependency.test(manifest))
          throw new Error(`${name}: ${runtime} must be pinned to ${version} on crates.io`)
      }
    }
    const readme = field(manifest, "readme")
    if (readme) await readFile(join(resolve(manifestPath, ".."), readme), "utf8")
    seen.push(name)
  }
  if (new Set(seen).size !== crateNames.length)
    throw new Error("Rust release unit has duplicate or missing crates")
  return seen
}

async function sha256(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex")
}

function archiveName(name, version) {
  return `${name}-${version}.crate`
}

async function localArchives(directory, version) {
  const names = new Set(await readdir(directory))
  const result = new Map()
  for (const name of crateNames) {
    const file = archiveName(name, version)
    if (!names.has(file)) throw new Error(`missing crate archive ${file}`)
    result.set(name, resolve(directory, file))
  }
  if (names.size !== crateNames.length)
    throw new Error("crate archive directory contains unexpected files")
  return result
}

export async function status({ name, version, archive, fetchImpl = fetch }) {
  if (!crateNames.includes(name)) throw new Error(`unexpected crate ${name}`)
  const record = await registryRecord(name, version, fetchImpl)
  if (!record) return "missing"
  const expected = await sha256(archive)
  if (record.cksum !== expected)
    throw new Error(`${name}@${version}: published checksum does not match the release unit`)
  return "matched"
}

export async function verifyRelease({
  version,
  archives,
  attempts = 1,
  delayMs = 0,
  fetchImpl = fetch,
}) {
  const local = await localArchives(resolve(archives), version)
  let missing = []
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    missing = []
    for (const [name, archive] of local) {
      if ((await status({ name, version, archive, fetchImpl })) === "missing") missing.push(name)
    }
    if (!missing.length) return crateNames
    if (attempt < attempts) await new Promise((resolveDelay) => setTimeout(resolveDelay, delayMs))
  }
  throw new Error(`crates.io is missing ${missing.join(", ")} at ${version}`)
}

async function main() {
  const [command, ...argv] = process.argv.slice(2)
  const values = parseArgs(argv)
  if (command === "inspect") {
    if (!values.rust) throw new Error("inspect requires --rust")
    const names = await inspectRelease({ version: values.version, rust: values.rust })
    console.log(`crates-release: inspected ${names.length} crates at ${values.version}`)
    return
  }
  if (command === "status") {
    if (!values.name || !values.archive) throw new Error("status requires --name and --archive")
    console.log(
      await status({
        name: values.name,
        version: values.version,
        archive: resolve(values.archive),
      }),
    )
    return
  }
  if (command === "verify") {
    if (!values.archives) throw new Error("verify requires --archives")
    const names = await verifyRelease({
      version: values.version,
      archives: values.archives,
      attempts: Number(values.attempts ?? 1),
      delayMs: Number(values["delay-ms"] ?? 0),
    })
    console.log(`crates-release: verified ${names.length} crates at ${values.version}`)
    return
  }
  throw new Error(`unknown command ${command ?? ""}`)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(import.meta.filename)) {
  main().catch((error) => {
    console.error(`crates-release: ${error.message}`)
    process.exit(1)
  })
}

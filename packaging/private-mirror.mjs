#!/usr/bin/env node
import { createHash } from "node:crypto"
import { copyFile, mkdir, readFile, readdir, writeFile } from "node:fs/promises"
import { basename, join, resolve } from "node:path"
import process from "node:process"
import { pathToFileURL } from "node:url"
import { crateNames, indexPath, registryRecord } from "./crates-release.mjs"
import { inspectLocalRelease } from "./pypi-release.mjs"
import { targets } from "./targets.mjs"

const stableVersion = /^\d+\.\d+\.\d+$/
const fullSHA = /^[0-9a-f]{40}$/
const defaultBase = "https://get.operatorstack.systems"

function expect(condition, message) {
  if (!condition) throw new Error(message)
}

async function sha256(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex")
}

async function responseSHA256(response) {
  expect(response.ok, `download returned HTTP ${response.status}`)
  return createHash("sha256")
    .update(Buffer.from(await response.arrayBuffer()))
    .digest("hex")
}

export function classifyCollection(expected, remote, label) {
  const expectedByName = new Map(expected.map((item) => [item.name, item.sha256]))
  expect(expectedByName.size === expected.length, `${label}: expected identities are not unique`)
  const remoteByName = new Map()
  for (const item of remote) {
    expect(!remoteByName.has(item.name), `${label}: duplicate remote artifact ${item.name}`)
    expect(expectedByName.has(item.name), `${label}: unexpected remote artifact ${item.name}`)
    expect(
      expectedByName.get(item.name) === item.sha256,
      `${label}: checksum drift for ${item.name}`,
    )
    remoteByName.set(item.name, item.sha256)
  }
  if (remoteByName.size === 0) return "missing"
  return remoteByName.size === expectedByName.size ? "matched" : "partial"
}

function missingArtifacts(expected, remote) {
  const present = new Set(remote.map((item) => item.name))
  return expected.filter((item) => !present.has(item.name))
}

export function validateMirrorIdentity({ version, sourceSHA, npmRelease }) {
  expect(stableVersion.test(version), "version must be stable semver")
  expect(fullSHA.test(sourceSHA), "source SHA must be a full commit SHA")
  expect(npmRelease?.schema_version === 1, "npm release schema must be 1")
  expect(npmRelease.version === version, "npm release version does not match")
  expect(npmRelease.source_sha === sourceSHA, "npm release source SHA does not match")
}

async function localCrates(directory, version) {
  const names = new Set(await readdir(directory))
  const crates = []
  for (const name of crateNames) {
    const file = `${name}-${version}.crate`
    expect(names.has(file), `missing crate archive ${file}`)
    crates.push({ name, file, sha256: await sha256(join(directory, file)) })
  }
  expect(
    [...names].filter((name) => name.endsWith(".crate")).length === crateNames.length,
    "crate archive directory contains unexpected files",
  )
  return crates
}

export async function inspectMirrorUnit({ version, sourceSHA, releaseUnit, crates }) {
  const npmRoot = join(releaseUnit, "npm")
  const npmRelease = JSON.parse(await readFile(join(npmRoot, "npm-release.json"), "utf8"))
  validateMirrorIdentity({ version, sourceSHA, npmRelease })
  expect(npmRelease.archives.length === 8, "private mirror requires eight npm archives")
  const npm = []
  for (const archive of npmRelease.archives) {
    expect(archive.version === version, `${archive.name}: npm version does not match`)
    const digest = await sha256(join(npmRoot, archive.file))
    expect(digest === archive.sha256, `${archive.name}: npm archive checksum drift`)
    npm.push({ name: archive.name, file: archive.file, sha256: digest })
  }
  const python = (await inspectLocalRelease(join(releaseUnit, "pypi"), version)).map((file) => ({
    name: file.filename,
    file: file.filename,
    sha256: file.sha256,
  }))
  const rust = await localCrates(crates, version)
  return {
    schema_version: 1,
    version,
    source_sha: sourceSHA,
    targets: targets.map((target) => target.id),
    npm,
    python,
    rust,
    go: { module: "github.com/operatorstack/yield", version: `v${version}` },
  }
}

async function remoteNPM(manifest, base, fetchImpl) {
  const remote = []
  for (const expected of manifest.npm) {
    const path = expected.name.replace("/", "%2F")
    const response = await fetchImpl(`${base}/npm/${path}`, { cache: "no-store" })
    if (response.status === 404) continue
    expect(response.ok, `${expected.name}: private npm returned HTTP ${response.status}`)
    const packument = await response.json()
    const record = packument.versions?.[manifest.version]
    if (!record) continue
    expect(record.dist?.tarball, `${expected.name}: private npm tarball URL is missing`)
    remote.push({
      name: expected.name,
      sha256: await responseSHA256(await fetchImpl(record.dist.tarball)),
    })
  }
  return remote
}

async function remotePython(manifest, base, fetchImpl) {
  const response = await fetchImpl(`${base}/pip/simple/yieldskill/`, { cache: "no-store" })
  if (response.status === 404) return []
  expect(response.ok, `private Python index returned HTTP ${response.status}`)
  const expected = new Set(manifest.python.map((file) => file.name))
  const remote = []
  for (const match of (await response.text()).matchAll(/href=["']([^"']+)["']/gi)) {
    const url = new URL(match[1], `${base}/pip/simple/yieldskill/`)
    const name = decodeURIComponent(basename(url.pathname))
    if (!expected.has(name)) continue
    const digest = url.hash.match(/^#sha256=([0-9a-f]{64})$/)?.[1]
    remote.push({ name, sha256: digest ?? (await responseSHA256(await fetchImpl(url))) })
  }
  return remote
}

async function remoteRust(manifest, base, fetchImpl) {
  const remote = []
  for (const expected of manifest.rust) {
    const response = await fetchImpl(`${base}/cargo/index/${indexPath(expected.name)}`, {
      headers: { "User-Agent": "operatorstack-yield-private-mirror/1" },
      cache: "no-store",
    })
    if (response.status === 404) continue
    expect(response.ok, `${expected.name}: private Cargo index returned HTTP ${response.status}`)
    const record = (await response.text())
      .split("\n")
      .filter(Boolean)
      .map((line) => JSON.parse(line))
      .find((item) => item.vers === manifest.version)
    if (!record) continue
    expect(record.cksum === expected.sha256, `${expected.name}: private Cargo index checksum drift`)
    const download = `${base}/cargo/crates/${expected.name}/${manifest.version}/download`
    const archive = await fetchImpl(download)
    if (archive.status === 404) continue
    expect(archive.ok, `${expected.name}: private Cargo download returned HTTP ${archive.status}`)
    remote.push({ name: expected.name, sha256: await responseSHA256(archive) })
  }
  return remote
}

async function remoteGo(manifest, base, fetchImpl) {
  const privateZip = `${base}/go/github.com/operatorstack/yield/@v/v${manifest.version}.zip`
  const response = await fetchImpl(privateZip, { cache: "no-store" })
  if (response.status === 404) return []
  expect(response.ok, `private Go proxy returned HTTP ${response.status}`)
  const privateDigest = await responseSHA256(response)
  const modulePath = `github.com/operatorstack/yield/@v/v${manifest.version}.mod`
  const [privateModule, publicModule] = await Promise.all([
    fetchImpl(`${base}/go/${modulePath}`, { cache: "no-store" }),
    fetchImpl(`https://proxy.golang.org/${modulePath}`, { cache: "no-store" }),
  ])
  expect(privateModule.ok, `private Go module metadata returned HTTP ${privateModule.status}`)
  expect(publicModule.ok, `public Go module metadata returned HTTP ${publicModule.status}`)
  const [privateModuleBytes, publicModuleBytes] = await Promise.all([
    privateModule.arrayBuffer(),
    publicModule.arrayBuffer(),
  ])
  expect(
    Buffer.from(privateModuleBytes).equals(Buffer.from(publicModuleBytes)),
    "private Go module metadata differs from the public module",
  )
  return [{ name: manifest.go.module, sha256: privateDigest }]
}

export async function inspectRemote(manifest, { base = defaultBase, fetchImpl = fetch } = {}) {
  const [npm, python, rust, go] = await Promise.all([
    remoteNPM(manifest, base, fetchImpl),
    remotePython(manifest, base, fetchImpl),
    remoteRust(manifest, base, fetchImpl),
    remoteGo(manifest, base, fetchImpl),
  ])
  const goExpected = go.length ? go : []
  return {
    states: {
      npm: classifyCollection(manifest.npm, npm, "npm"),
      python: classifyCollection(manifest.python, python, "Python"),
      rust: classifyCollection(manifest.rust, rust, "Cargo"),
      go: goExpected.length === 0 ? "missing" : "matched",
    },
    remote: { npm, python, rust, go },
    missing: {
      npm: missingArtifacts(manifest.npm, npm),
      python: missingArtifacts(manifest.python, python),
      rust: missingArtifacts(manifest.rust, rust),
      go: go.length ? [] : [manifest.go],
    },
  }
}

export async function prepareCargoMirror(
  manifest,
  { crates, output, names = crateNames, fetchImpl = fetch },
) {
  await mkdir(output, { recursive: true })
  const selected = new Set(names)
  for (const item of manifest.rust.filter((candidate) => selected.has(candidate.name))) {
    const record = await registryRecord(item.name, manifest.version, fetchImpl)
    expect(record, `${item.name}@${manifest.version}: crates.io index record is missing`)
    expect(
      record.cksum === item.sha256,
      `${item.name}: crates.io checksum differs from release unit`,
    )
    await copyFile(join(crates, item.file), join(output, item.file))
    await writeFile(join(output, `${item.name}-index.json`), `${JSON.stringify(record)}\n`)
  }
}

function receipt(manifest, remote, base) {
  return {
    schema_version: 1,
    version: manifest.version,
    source_sha: manifest.source_sha,
    registry: base,
    targets: manifest.targets,
    endpoints: {
      npm: `${base}/npm/`,
      python: `${base}/pip/simple/yieldskill/`,
      go: `${base}/go/${manifest.go.module}/@v/v${manifest.version}.zip`,
      rust_index: `${base}/cargo/index/`,
      rust_download: `${base}/cargo/crates/{crate}/{version}/download`,
    },
    packages: {
      npm: remote.remote.npm,
      python: remote.remote.python,
      go: remote.remote.go,
      rust: remote.remote.rust,
    },
    states: remote.states,
  }
}

function parseArgs(argv) {
  const [command, ...rest] = argv
  const values = {}
  for (let index = 0; index < rest.length; index += 2) {
    const key = rest[index]
    expect(key?.startsWith("--") && rest[index + 1] !== undefined, `invalid argument ${key ?? ""}`)
    values[key.slice(2)] = rest[index + 1]
  }
  expect(["inspect", "status", "prepare-cargo", "verify"].includes(command), "unknown command")
  return { command, ...values }
}

async function appendOutput(path, states) {
  if (!path) return
  await writeFile(
    path,
    Object.entries(states)
      .map(([key, value]) => `${key}_state=${value}\n`)
      .join(""),
    {
      flag: "a",
    },
  )
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  if (options.command === "inspect") {
    expect(
      options.version && options["source-sha"] && options["release-unit"] && options.crates,
      "inspect inputs are required",
    )
    const manifest = await inspectMirrorUnit({
      version: options.version,
      sourceSHA: options["source-sha"],
      releaseUnit: resolve(options["release-unit"]),
      crates: resolve(options.crates),
    })
    expect(options.manifest, "--manifest is required")
    await writeFile(options.manifest, `${JSON.stringify(manifest, null, 2)}\n`)
    return
  }
  expect(options.manifest, "--manifest is required")
  const manifest = JSON.parse(await readFile(options.manifest, "utf8"))
  if (options.command === "prepare-cargo") {
    expect(options.crates && options.output, "prepare-cargo requires --crates and --output")
    const status = options.status ? JSON.parse(await readFile(options.status, "utf8")) : null
    await prepareCargoMirror(manifest, {
      crates: resolve(options.crates),
      output: resolve(options.output),
      names: status?.missing?.rust?.map((item) => item.name) ?? crateNames,
    })
    return
  }
  const remote = await inspectRemote(manifest, { base: options.base ?? defaultBase })
  if (options.command === "status") {
    await appendOutput(options.output, remote.states)
    if (options.status) await writeFile(options.status, `${JSON.stringify(remote, null, 2)}\n`)
    process.stdout.write(`${JSON.stringify(remote.states, null, 2)}\n`)
    return
  }
  expect(
    Object.values(remote.states).every((state) => state === "matched"),
    "private mirror is incomplete",
  )
  expect(options.receipt, "verify requires --receipt")
  await writeFile(
    options.receipt,
    `${JSON.stringify(receipt(manifest, remote, options.base ?? defaultBase), null, 2)}\n`,
  )
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(`private-mirror: ${error.message}`)
    process.exit(1)
  })
}

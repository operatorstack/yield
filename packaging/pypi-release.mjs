#!/usr/bin/env node
import { createHash } from "node:crypto"
import { copyFile, mkdir, readdir, readFile, rm } from "node:fs/promises"
import { join, resolve } from "node:path"
import process from "node:process"
import { pathToFileURL } from "node:url"
import { targets } from "./targets.mjs"

const stableVersion = /^\d+\.\d+\.\d+$/

function expect(condition, message) {
  if (!condition) throw new Error(message)
}

function parseArgs(argv) {
  const [command, ...rest] = argv
  const values = {}
  for (let index = 0; index < rest.length; index += 2) {
    const key = rest[index]
    if (!key?.startsWith("--") || rest[index + 1] === undefined)
      throw new Error(`invalid argument ${key ?? ""}`)
    values[key.slice(2)] = rest[index + 1]
  }
  expect(
    ["inspect", "prepare", "verify"].includes(command),
    "command must be inspect, prepare, or verify",
  )
  expect(stableVersion.test(values.version ?? ""), "--version must be stable semver")
  expect(values.dist, "--dist is required")
  if (command === "prepare") expect(values.upload, "--upload is required for prepare")
  return { command, ...values }
}

export function expectedWheelNames(version) {
  expect(stableVersion.test(version), `invalid stable version ${version}`)
  return targets.map((target) => `yieldskill-${version}-py3-none-${target.pythonTag}.whl`).sort()
}

async function sha256(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex")
}

export async function inspectLocalRelease(directory, version) {
  const expected = expectedWheelNames(version)
  const actual = (await readdir(directory)).filter((name) => name.endsWith(".whl")).sort()
  expect(
    JSON.stringify(actual) === JSON.stringify(expected),
    `wheel set mismatch: expected ${expected.join(", ")}; got ${actual.join(", ")}`,
  )
  return Promise.all(
    actual.map(async (filename) => ({ filename, sha256: await sha256(join(directory, filename)) })),
  )
}

export function compareRelease(local, remote) {
  const localByName = new Map(local.map((file) => [file.filename, file.sha256]))
  expect(localByName.size === local.length, "local wheel filenames must be unique")
  const remoteByName = new Map()
  for (const file of remote) {
    expect(!remoteByName.has(file.filename), `duplicate remote file ${file.filename}`)
    expect(localByName.has(file.filename), `unexpected remote file ${file.filename}`)
    remoteByName.set(file.filename, file.sha256)
    expect(
      localByName.get(file.filename) === file.sha256,
      `remote hash mismatch for ${file.filename}`,
    )
  }
  return local.filter((file) => !remoteByName.has(file.filename))
}

export async function fetchPyPIRelease(version, fetchImpl = fetch) {
  const response = await fetchImpl(`https://pypi.org/pypi/yieldskill/${version}/json`, {
    headers: { Accept: "application/json" },
    cache: "no-store",
  })
  if (response.status === 404) return []
  expect(response.ok, `PyPI returned HTTP ${response.status}`)
  const payload = await response.json()
  return (payload.urls ?? []).map((file) => ({
    filename: file.filename,
    sha256: file.digests?.sha256 ?? "",
  }))
}

export async function prepareUpload({ dist, upload, version, remote }) {
  const local = await inspectLocalRelease(dist, version)
  const missing = compareRelease(local, remote)
  await rm(upload, { recursive: true, force: true })
  await mkdir(upload, { recursive: true })
  for (const file of missing) await copyFile(join(dist, file.filename), join(upload, file.filename))
  return { local, missing }
}

async function appendOutput(path, values) {
  if (!path) return
  const { appendFile } = await import("node:fs/promises")
  await appendFile(
    path,
    Object.entries(values)
      .map(([key, value]) => `${key}=${value}\n`)
      .join(""),
  )
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  const dist = resolve(options.dist)
  const version = options.version
  if (options.command === "inspect") {
    const local = await inspectLocalRelease(dist, version)
    process.stdout.write(`${JSON.stringify({ version, files: local }, null, 2)}\n`)
    return
  }
  if (options.command === "prepare") {
    const remote = await fetchPyPIRelease(version)
    const result = await prepareUpload({ dist, upload: resolve(options.upload), version, remote })
    await appendOutput(options.output, {
      publish: result.missing.length > 0,
      missing_count: result.missing.length,
    })
    process.stdout.write(
      `${JSON.stringify({ version, missing: result.missing.map(({ filename }) => filename) }, null, 2)}\n`,
    )
    return
  }

  const attempts = Number(options.attempts ?? "1")
  const delayMs = Number(options["delay-ms"] ?? "0")
  expect(Number.isInteger(attempts) && attempts > 0, "--attempts must be a positive integer")
  const local = await inspectLocalRelease(dist, version)
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    const missing = compareRelease(local, await fetchPyPIRelease(version))
    if (missing.length === 0) {
      process.stdout.write(`${JSON.stringify({ version, verified: true, files: local.length })}\n`)
      return
    }
    if (attempt === attempts)
      throw new Error(
        `PyPI release is incomplete: ${missing.map(({ filename }) => filename).join(", ")}`,
      )
    await new Promise((resolveDelay) => setTimeout(resolveDelay, delayMs))
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(`pypi-release: ${error.message}`)
    process.exit(1)
  })
}

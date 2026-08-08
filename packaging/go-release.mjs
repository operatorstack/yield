#!/usr/bin/env node
import { execFile as execFileCallback } from "node:child_process"
import { mkdtemp, rm } from "node:fs/promises"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import process from "node:process"
import { promisify } from "node:util"
import { pathToFileURL } from "node:url"

export const modulePath = "github.com/operatorstack/yield"
const stableVersion = /^\d+\.\d+\.\d+$/
const commit = /^[0-9a-f]{40}$/
const execFile = promisify(execFileCallback)

function goPlatform() {
  const operatingSystem = process.platform === "win32" ? "windows" : process.platform
  const architecture = process.arch === "x64" ? "amd64" : process.arch
  return `${operatingSystem}/${architecture}`
}

function expect(condition, message) {
  if (!condition) throw new Error(message)
}

export function validateModuleReceipt(receipt, { version, sourceSha }) {
  expect(stableVersion.test(version), `invalid stable version ${version}`)
  expect(commit.test(sourceSha), `invalid source SHA ${sourceSha}`)
  expect(receipt?.Path === modulePath, `unexpected Go module ${receipt?.Path ?? "missing"}`)
  expect(
    receipt?.Version === `v${version}`,
    `unexpected Go module version ${receipt?.Version ?? "missing"}`,
  )
  expect(receipt?.Origin?.VCS === "git", "Go module origin must use git")
  expect(
    receipt?.Origin?.URL === `https://${modulePath}`,
    `unexpected Go module origin ${receipt?.Origin?.URL ?? "missing"}`,
  )
  expect(receipt?.Origin?.Hash === sourceSha, "Go module source does not match the release tag")
  return receipt
}

export async function verifyGoRelease({
  version,
  sourceSha,
  attempts = 1,
  delayMs = 0,
  execImpl = execFile,
  delay = (milliseconds) => new Promise((resolveDelay) => setTimeout(resolveDelay, milliseconds)),
}) {
  expect(Number.isInteger(attempts) && attempts > 0, "attempts must be a positive integer")
  const bin = await mkdtemp(join(tmpdir(), "yield-go-release-"))
  const environment = {
    ...process.env,
    GOBIN: bin,
    GOSUMDB: "sum.golang.org",
    GOPROXY: "https://proxy.golang.org",
    GOWORK: "off",
  }
  let lastError
  try {
    for (let attempt = 1; attempt <= attempts; attempt += 1) {
      try {
        const listed = await execImpl("go", ["list", "-m", "-json", `${modulePath}@v${version}`], {
          env: environment,
        })
        validateModuleReceipt(JSON.parse(listed.stdout), { version, sourceSha })
        await execImpl("go", ["install", `${modulePath}/cmd/yskill@v${version}`], {
          env: environment,
        })
        const executable = join(bin, process.platform === "win32" ? "yskill.exe" : "yskill")
        const installed = await execImpl(executable, ["--version"], { env: environment })
        expect(
          installed.stdout.trim() === `yskill ${version} ${goPlatform()}`,
          `unexpected yskill version: ${installed.stdout.trim()}`,
        )
        return { module: modulePath, version, sourceSha }
      } catch (error) {
        lastError = error
        if (attempt < attempts) await delay(delayMs)
      }
    }
  } finally {
    await rm(bin, { recursive: true, force: true })
  }
  throw lastError
}

function parseArgs(argv) {
  const values = {}
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index]
    if (!key?.startsWith("--") || argv[index + 1] === undefined)
      throw new Error(`invalid argument ${key ?? ""}`)
    values[key.slice(2)] = argv[index + 1]
  }
  expect(stableVersion.test(values.version ?? ""), "--version must be stable semver")
  expect(commit.test(values["source-sha"] ?? ""), "--source-sha must be a full commit SHA")
  return values
}

async function main() {
  const values = parseArgs(process.argv.slice(2))
  const result = await verifyGoRelease({
    version: values.version,
    sourceSha: values["source-sha"],
    attempts: Number(values.attempts ?? "1"),
    delayMs: Number(values["delay-ms"] ?? "0"),
  })
  process.stdout.write(`${JSON.stringify({ ...result, verified: true })}\n`)
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(`go-release: ${error.message}`)
    process.exit(1)
  })
}

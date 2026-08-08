#!/usr/bin/env node
import { createHash } from "node:crypto"
import { readFile, writeFile } from "node:fs/promises"
import { dirname, join, resolve } from "node:path"

const languages = ["typescript", "python", "go", "rust"]
const targets = [
  "linux-amd64",
  "linux-arm64",
  "darwin-amd64",
  "darwin-arm64",
  "windows-amd64",
  "windows-arm64",
]

function expect(value, message) {
  if (!value) throw new Error(message)
}

async function json(path) {
  return JSON.parse(await readFile(path, "utf8"))
}

async function digest(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex")
}

export async function createPackageContract({
  version,
  sourceSHA,
  npmReleasePath,
  evaluationPath,
  releaseURL,
}) {
  expect(/^\d+\.\d+\.\d+$/.test(version), "version must be stable semver")
  expect(/^[0-9a-f]{40}$/.test(sourceSHA), "source SHA must be a full commit SHA")
  expect(
    releaseURL === `https://github.com/operatorstack/yield/releases/tag/v${version}`,
    "release URL must identify the matching Yield release",
  )
  const npm = await json(npmReleasePath)
  expect(
    npm.schema_version === 1 && npm.version === version && npm.source_sha === sourceSHA,
    "npm release manifest does not match the release version and source",
  )
  expect(
    npm.archives?.length === 8,
    "npm release manifest must contain the SDK, initializer, and six runtimes",
  )
  const names = new Set()
  for (const archive of npm.archives) {
    expect(
      typeof archive.name === "string" &&
        typeof archive.file === "string" &&
        /^[0-9a-f]{64}$/.test(archive.sha256 ?? ""),
      "npm release manifest contains an invalid archive",
    )
    expect(!names.has(archive.name), "npm release manifest contains a duplicate package")
    names.add(archive.name)
    expect(
      (await digest(join(dirname(npmReleasePath), archive.file))) === archive.sha256,
      `npm archive digest does not match ${archive.file}`,
    )
  }
  const evaluation = await json(evaluationPath)
  expect(
    evaluation.schema_version === 2 && evaluation.status === "passed",
    "evaluation result is not passing required evidence",
  )
  expect(
    /^[0-9a-f]{64}$/.test(evaluation.source_digest ?? ""),
    "evaluation source digest is invalid",
  )
  const contract = {
    schema_version: 1,
    version,
    source_sha: sourceSHA,
    languages,
    targets,
    current_and_previous_install_journeys: true,
    package_history_complete: true,
    npm_archives: npm.archives,
    evidence_digest: createHash("sha256")
      .update(await readFile(evaluationPath))
      .digest("hex"),
    evaluation_source_digest: evaluation.source_digest,
    release_url: releaseURL,
  }
  const contractDigest = createHash("sha256").update(JSON.stringify(contract)).digest("hex")
  return {
    contract,
    promotion: {
      schema_version: 1,
      version,
      source_sha: sourceSHA,
      contract_digest: contractDigest,
      evidence_digest: contract.evidence_digest,
      release_url: releaseURL,
    },
  }
}

function args(argv) {
  const values = {}
  for (let index = 0; index < argv.length; index += 2)
    values[argv[index]?.replace(/^--/, "")] = argv[index + 1]
  for (const name of [
    "version",
    "source-sha",
    "npm-release",
    "evaluation",
    "release-url",
    "output",
  ])
    expect(values[name], `--${name} is required`)
  return values
}

if (process.argv[1] && import.meta.filename === process.argv[1]) {
  const values = args(process.argv.slice(2))
  createPackageContract({
    version: values.version,
    sourceSHA: values["source-sha"],
    npmReleasePath: resolve(values["npm-release"]),
    evaluationPath: resolve(values.evaluation),
    releaseURL: values["release-url"],
  })
    .then(async ({ contract, promotion }) => {
      const output = resolve(values.output)
      await writeFile(
        `${output}/yield-package-contract.json`,
        `${JSON.stringify(contract, null, 2)}\n`,
      )
      await writeFile(
        `${output}/yield-website-promotion.json`,
        `${JSON.stringify(promotion, null, 2)}\n`,
      )
    })
    .catch((error) => {
      console.error(`package-contract: ${error.message}`)
      process.exit(1)
    })
}

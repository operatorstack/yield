#!/usr/bin/env node
import { createHash } from "node:crypto"
import { execFile } from "node:child_process"
import { mkdir, readFile, readdir, stat, writeFile } from "node:fs/promises"
import { join, resolve } from "node:path"
import { promisify } from "node:util"

const run = promisify(execFile)

function expect(value, message) {
  if (!value) throw new Error(message)
}

async function digest(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex")
}

async function packageJSON(path) {
  return JSON.parse(await readFile(join(path, "package.json"), "utf8"))
}

async function tarListing(path) {
  const { stdout } = await run("tar", ["-tzvf", path], { encoding: "utf8" })
  return stdout
}

export async function verifyArchive(path, expected) {
  const listing = await tarListing(path)
  expect(listing.includes("package/package.json"), `${path}: archive is missing package.json`)
  if (!expected.runtime) return
  const runtime = `package/${expected.runtime}`
  const line = listing.split("\n").find((value) => value.endsWith(` ${runtime}`))
  expect(line, `${path}: archive is missing ${runtime}`)
  if (!expected.windows)
    expect(/^-rwxr-xr-x\s/.test(line), `${path}: ${runtime} is not executable in the tar header`)
}

export async function packReleaseUnit({ source, output, sourceSHA = "" }) {
  expect(/^[0-9a-f]{40}$/.test(sourceSHA), "source SHA must be a full commit SHA")
  await mkdir(output, { recursive: true })
  const directories = (await readdir(source, { withFileTypes: true }))
    .filter((entry) => entry.isDirectory())
    .map((entry) => join(source, entry.name))
    .sort()
  const archives = []
  for (const directory of directories) {
    const manifest = await packageJSON(directory)
    const { stdout } = await run("npm", ["pack", "--json", "--pack-destination", output], {
      cwd: directory,
      encoding: "utf8",
    })
    const [packed] = JSON.parse(stdout)
    expect(
      packed?.name === manifest.name && packed?.version === manifest.version,
      `${directory}: npm pack identity drift`,
    )
    const archive = join(output, packed.filename)
    const details = await stat(archive)
    expect(
      details.isFile() && details.size > 0,
      `${directory}: npm pack did not create ${packed.filename}`,
    )
    const runtime = manifest.files?.find((file) => file === "yskill" || file === "yskill.exe")
    await verifyArchive(archive, { runtime, windows: runtime === "yskill.exe" })
    archives.push({
      name: manifest.name,
      version: manifest.version,
      file: packed.filename,
      sha256: await digest(archive),
    })
  }
  const versions = new Set(archives.map((entry) => entry.version))
  expect(versions.size === 1, "npm release unit must contain exactly one version")
  const release = {
    schema_version: 1,
    version: archives[0]?.version ?? "",
    source_sha: sourceSHA,
    archives,
  }
  await writeFile(join(output, "npm-release.json"), `${JSON.stringify(release, null, 2)}\n`)
  return release
}

function parseArgs(argv) {
  const values = {}
  for (let index = 0; index < argv.length; index += 2)
    values[argv[index]?.replace(/^--/, "")] = argv[index + 1]
  if (!values.source || !values.output || !values["source-sha"])
    throw new Error("--source, --output, and --source-sha are required")
  return {
    source: resolve(values.source),
    output: resolve(values.output),
    sourceSHA: values["source-sha"],
  }
}

if (process.argv[1] && import.meta.filename === process.argv[1]) {
  packReleaseUnit(parseArgs(process.argv.slice(2)))
    .then((result) =>
      console.log(`packed ${result.archives.length} immutable npm archives for ${result.version}`),
    )
    .catch((error) => {
      console.error(`npm-release: ${error.message}`)
      process.exit(1)
    })
}

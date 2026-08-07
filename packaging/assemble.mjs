#!/usr/bin/env node
import { chmod, cp, mkdir, readFile, realpath, rm, stat, writeFile } from "node:fs/promises";
import { basename, join, resolve } from "node:path";
import { createHash } from "node:crypto";
import process from "node:process";
import { binaryName, npmPackage, rustPackage, targets } from "./targets.mjs";

const root = resolve(import.meta.dirname, "..");

function parseArgs(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 2) values[argv[index]?.replace(/^--/, "")] = argv[index + 1];
  if (!/^\d+\.\d+\.\d+$/.test(values.version ?? "")) throw new Error("--version must be semver without a leading v");
  if (!values.binaries || !values.output) throw new Error("--binaries and --output are required");
  return { version: values.version, binaries: resolve(values.binaries), output: resolve(values.output) };
}

async function json(path) {
  return JSON.parse(await readFile(path, "utf8"));
}

async function sha256(path) {
  return createHash("sha256").update(await readFile(path)).digest("hex");
}

async function copyBinary(source, destination, executable = true) {
  await mkdir(resolve(destination, ".."), { recursive: true });
  await cp(source, destination);
  if (executable && !destination.endsWith(".exe")) await chmod(destination, 0o755);
}

async function validateBinaries(directory) {
  const records = [];
  for (const target of targets) {
    const path = join(directory, binaryName(target));
    const details = await stat(path).catch(() => null);
    if (!details?.isFile() || details.size === 0) throw new Error(`missing runtime ${path}`);
    records.push({ target: target.id, file: basename(path), bytes: details.size, sha256: await sha256(path) });
  }
  return records;
}

async function assembleNpm({ version, binaries, output }) {
  const npm = join(output, "npm");
  const main = join(npm, "yield");
  await cp(join(root, "sdk/typescript"), main, { recursive: true, filter: (source) => !source.includes("node_modules") && !source.includes("/dist") });
  await Promise.all([
    cp(join(root, "README.md"), join(main, "README.md")),
    cp(join(root, "LICENSE"), join(main, "LICENSE")),
  ]);
  const packageJson = await json(join(main, "package.json"));
  packageJson.version = version;
  packageJson.publishConfig = {
    access: "public",
    provenance: true,
    registry: "https://registry.npmjs.org/",
  };
  packageJson.optionalDependencies = Object.fromEntries(targets.map((target) => [npmPackage(target), version]));
  await writeFile(join(main, "package.json"), `${JSON.stringify(packageJson, null, 2)}\n`);

  for (const target of targets) {
    const directory = join(npm, target.id);
    const runtime = target.goos === "windows" ? "yskill.exe" : "yskill";
    await mkdir(directory, { recursive: true });
    await copyBinary(join(binaries, binaryName(target)), join(directory, runtime));
    await cp(join(root, "LICENSE"), join(directory, "LICENSE"));
    await writeFile(join(directory, "package.json"), `${JSON.stringify({
      name: npmPackage(target), version, description: `Yield runtime for ${target.id}`,
      license: "MIT", os: [target.nodeOs], cpu: [target.nodeCpu], main: `./${runtime}`,
      files: [runtime, "LICENSE"], repository: { type: "git", url: "git+https://github.com/operatorstack/yield.git" },
      homepage: "https://github.com/operatorstack/yield#readme",
      bugs: { url: "https://github.com/operatorstack/yield/issues" },
      publishConfig: { access: "public", provenance: true, registry: "https://registry.npmjs.org/" },
    }, null, 2)}\n`);
  }
}

async function assemblePython({ version, binaries, output }) {
  const python = join(output, "python");
  for (const target of targets) {
    const directory = join(python, target.id);
    await cp(join(root, "sdk/python"), directory, { recursive: true, filter: (source) => !source.includes("__pycache__") && !source.includes("/dist") && !source.includes("/build") });
    const pyproject = (await readFile(join(directory, "pyproject.toml"), "utf8")).replace(/^version = ".*"/m, `version = "${version}"`);
    await writeFile(join(directory, "pyproject.toml"), pyproject);
    const runtime = target.goos === "windows" ? "yskill.exe" : "yskill";
    await copyBinary(join(binaries, binaryName(target)), join(directory, "yieldskill/_runtime", runtime));
    await writeFile(join(directory, "setup.py"), `from wheel.bdist_wheel import bdist_wheel\nfrom setuptools import setup\n\nclass PlatformWheel(bdist_wheel):\n    def finalize_options(self):\n        super().finalize_options()\n        self.root_is_pure = False\n    def get_tag(self):\n        return ("py3", "none", "${target.pythonTag}")\n\nsetup(cmdclass={"bdist_wheel": PlatformWheel})\n`);
  }
}

function rustDependency(target, version) {
  return `[target.'cfg(all(target_os = "${target.rustOs}", target_arch = "${target.rustArch}"))'.dependencies]\n${rustPackage(target)} = { version = "=${version}", registry = "operatorstack" }\n`;
}

async function assembleRust({ version, binaries, output }, records) {
  const rust = join(output, "rust");
  const runtimeByTarget = new Map(records.map((record) => [record.target, record]));
  for (const target of targets) {
    const name = rustPackage(target);
    const directory = join(rust, "runtime", target.id);
    const runtime = target.goos === "windows" ? "yskill.exe" : "yskill";
    await mkdir(join(directory, "src"), { recursive: true });
    await copyBinary(join(binaries, binaryName(target)), join(directory, "runtime", runtime), false);
    await writeFile(join(directory, "Cargo.toml"), `[package]\nname = "${name}"\nversion = "${version}"\nedition = "2021"\nlicense = "MIT"\ndescription = "Internal Yield runtime for ${target.id}"\nrepository = "https://github.com/operatorstack/yield"\ninclude = ["src/lib.rs", "runtime/${runtime}"]\n\n[lib]\npath = "src/lib.rs"\n`);
    await writeFile(join(directory, "src/lib.rs"), `pub const BYTES: &[u8] = include_bytes!("../runtime/${runtime}");\npub const SHA256: &str = "${runtimeByTarget.get(target.id).sha256}";\n`);
  }

  const main = join(rust, "yieldskill");
  await cp(join(root, "sdk/rust"), main, { recursive: true, filter: (source) => !source.includes("/target") });
  let cargo = (await readFile(join(main, "Cargo.toml"), "utf8")).replace(/^version = ".*"/m, `version = "${version}"`);
  cargo += `\n[[bin]]\nname = "yskill"\npath = "src/bin/yskill.rs"\n\n${targets.map((target) => rustDependency(target, version)).join("\n")}`;
  await writeFile(join(main, "Cargo.toml"), cargo);
  await mkdir(join(main, "src/bin"), { recursive: true });
  await cp(join(root, "packaging/rust-launcher.rs"), join(main, "src/bin/yskill.rs"));
}

export async function assemble(options) {
  await rm(options.output, { recursive: true, force: true });
  await mkdir(options.output, { recursive: true });
  const records = await validateBinaries(options.binaries);
  await Promise.all([assembleNpm(options), assemblePython(options), assembleRust(options, records)]);
  await writeFile(join(options.output, "SHA256SUMS.json"), `${JSON.stringify({ version: options.version, artifacts: records }, null, 2)}\n`);
}

if (process.argv[1]) {
  const [entrypoint, modulePath] = await Promise.all([
    realpath(resolve(process.argv[1])),
    realpath(import.meta.filename),
  ]);
  if (entrypoint === modulePath) {
    assemble(parseArgs(process.argv.slice(2))).catch((error) => { console.error(`assemble: ${error.message}`); process.exit(1); });
  }
}

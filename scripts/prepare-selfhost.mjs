#!/usr/bin/env node
import { chmod, realpath, stat } from "node:fs/promises";
import { createRequire } from "node:module";
import { resolve, sep } from "node:path";

const root = resolve(import.meta.dirname, "..");
const require = createRequire(import.meta.url);
const platforms = new Map([
  ["darwin:x64", "@operatorstack/yield-darwin-amd64"],
  ["darwin:arm64", "@operatorstack/yield-darwin-arm64"],
  ["linux:x64", "@operatorstack/yield-linux-amd64"],
  ["linux:arm64", "@operatorstack/yield-linux-arm64"],
  ["win32:x64", "@operatorstack/yield-windows-amd64"],
  ["win32:arm64", "@operatorstack/yield-windows-arm64"],
]);

const packageName = platforms.get(`${process.platform}:${process.arch}`);
if (!packageName) throw new Error(`unsupported self-host platform ${process.platform}/${process.arch}`);
const runtime = await realpath(require.resolve(packageName));
const modules = await realpath(resolve(root, "node_modules"));
if (runtime !== modules && !runtime.startsWith(`${modules}${sep}`)) throw new Error("refusing to chmod a runtime outside this repository's node_modules");
const details = await stat(runtime);
if (!details.isFile() || details.size === 0) throw new Error("published Yield runtime is missing or empty");
if (process.platform !== "win32" && (details.mode & 0o111) === 0) await chmod(runtime, details.mode | 0o755);
process.stdout.write(`prepared ${packageName}\n`);

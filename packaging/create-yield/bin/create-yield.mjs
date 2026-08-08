#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";
import process from "node:process";

const require = createRequire(import.meta.url);
const sdkEntry = require.resolve("@operatorstack/yield");
const cli = resolve(dirname(sdkEntry), "../bin/yskill.mjs");
const result = spawnSync(process.execPath, [cli, "bootstrap", "--language", "typescript", ...process.argv.slice(2)], {
  stdio: "inherit",
});

if (result.error) {
  console.error(`create-yield: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status ?? 1);

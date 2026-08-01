#!/usr/bin/env node
import { spawn } from "node:child_process";
import process from "node:process";
import { resolveRuntime } from "./runtime.mjs";

let binary;
try {
  binary = resolveRuntime();
} catch (error) {
  console.error(`yskill: ${error.message}`);
  process.exit(1);
}

const child = spawn(binary, process.argv.slice(2), {
  stdio: "inherit",
  env: { ...process.env, YIELD_LANGUAGE: process.env.YIELD_LANGUAGE ?? "typescript" },
});
for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(signal, () => {
    if (!child.killed) child.kill(signal);
  });
}
child.on("error", (error) => {
  console.error(`yskill: could not start the packaged runtime: ${error.message}`);
  process.exit(1);
});
child.on("exit", (code, signal) => {
  if (signal && process.platform !== "win32") {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});

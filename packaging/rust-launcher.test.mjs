import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("Rust wrapper passes its repository-local path to the runtime", async () => {
  const source = await readFile(new URL("./rust-launcher.rs", import.meta.url), "utf8");
  assert.match(source, /current_exe\(\)/);
  assert.match(source, /YIELD_LAUNCHER_PATH/);
});

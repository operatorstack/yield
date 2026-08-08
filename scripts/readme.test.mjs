import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");

async function text(path) {
  return readFile(resolve(root, path), "utf8");
}

test("README release example matches the tested TypeScript source", async () => {
  const [readme, source] = await Promise.all([
    text("README.md"),
    text("examples/release-checklist/main.ts"),
  ]);

  const readmeMatch = readme.match(
    /<!-- release-example:start -->\s*```typescript\n([\s\S]*?)\n```\s*<!-- release-example:end -->/,
  );
  assert.ok(readmeMatch, "README release example markers are missing");

  const sourceMatch = source.match(
    /\/\/ README_EXAMPLE_START\n([\s\S]*?)\n\/\/ README_EXAMPLE_END/,
  );
  assert.ok(sourceMatch, "TypeScript release example markers are missing");

  const readmeProgram = readmeMatch[1]
    .replace(/^import \{ defineSkill \} from "@operatorstack\/yield";\n+/, "")
    .trim();
  assert.equal(readmeProgram, sourceMatch[1].trim());
});

test("Python README example matches the tested environment doctor", async () => {
  const [readme, source] = await Promise.all([
    text("sdk/python/README.md"),
    text("examples/env-doctor/main.py"),
  ]);

  const readmeMatch = readme.match(
    /<!-- python-example:start -->\s*```python\n([\s\S]*?)\n```\s*<!-- python-example:end -->/,
  );
  assert.ok(readmeMatch, "Python README example markers are missing");

  const sourceMatch = source.match(
    /# README_EXAMPLE_START\n([\s\S]*?)\n# README_EXAMPLE_END/,
  );
  assert.ok(sourceMatch, "Python source example markers are missing");

  const readmeProgram = readmeMatch[1]
    .replace(/^from yieldskill import define_skill\n+/, "")
    .trim();
  assert.equal(readmeProgram, sourceMatch[1].trim());
});

test("Rust README example matches the tested data migration", async () => {
  const [readme, source, fixture] = await Promise.all([
    text("sdk/rust/README.md"),
    text("examples/data-migration/src/main.rs"),
    text("examples/data-migration/fixtures/responses.json"),
  ]);

  const readmeMatch = readme.match(
    /<!-- rust-example:start -->\s*```rust\n([\s\S]*?)\n```\s*<!-- rust-example:end -->/,
  );
  assert.ok(readmeMatch, "Rust README example markers are missing");

  const sourceMatch = source.match(
    /\/\/ README_EXAMPLE_START\n([\s\S]*?)\n\/\/ README_EXAMPLE_END/,
  );
  assert.ok(sourceMatch, "Rust source example markers are missing");
  assert.equal(readmeMatch[1].trim(), sourceMatch[1].trim());

  const fixtureMatch = readme.match(
    /<!-- rust-fixture:start -->\s*```json\n([\s\S]*?)\n```\s*<!-- rust-fixture:end -->/,
  );
  assert.ok(fixtureMatch, "Rust README fixture markers are missing");
  assert.deepEqual(JSON.parse(fixtureMatch[1]), JSON.parse(fixture));
});

test("Go README example matches the tested investigation workflow", async () => {
  const [readme, source, fixture] = await Promise.all([
    text("sdk/yield/README.md"),
    text("examples/investigate/main.go"),
    text("examples/investigate/fixtures/responses.json"),
  ]);

  const readmeMatch = readme.match(
    /<!-- go-example:start -->\s*```go\n([\s\S]*?)\n```\s*<!-- go-example:end -->/,
  );
  assert.ok(readmeMatch, "Go README example markers are missing");
  const sourceMatch = source.match(/^(package main[\s\S]*)$/m);
  assert.ok(sourceMatch, "Go source program is missing");
  assert.equal(readmeMatch[1].trim(), sourceMatch[1].trim());

  const fixtureMatch = readme.match(
    /<!-- go-fixture:start -->\s*```json\n([\s\S]*?)\n```\s*<!-- go-fixture:end -->/,
  );
  assert.ok(fixtureMatch, "Go README fixture markers are missing");
  assert.deepEqual(JSON.parse(fixtureMatch[1]), JSON.parse(fixture));
});

test("Go README presents a public five-step workflow", async () => {
  const readme = await text("sdk/yield/README.md");
  const headings = [
    "### 1. Install Yield",
    "### 2. Create the workflow",
    "### 3. Test the workflow",
    "### 4. Register the skill",
    "### 5. Run the skill",
  ];
  let previous = -1;
  for (const heading of headings) {
    const current = readme.indexOf(heading);
    assert.ok(current > previous, `${heading} is missing or out of order`);
    previous = current;
  }
  assert.match(readme, /go install github\.com\/operatorstack\/yield\/cmd\/yskill@latest/);
  assert.match(readme, /yskill init skills\/investigate/);
  assert.match(readme, /yskill doctor skills\/investigate --test/);
  assert.match(readme, /yskill register skills\/investigate/);
  assert.match(readme, /^\/investigate$/m);
  assert.match(readme, /https:\/\/pkg\.go\.dev\/github\.com\/operatorstack\/yield\/sdk\/yield/);
  assert.match(readme, /https:\/\/proxy\.golang\.org/);
  assert.doesNotMatch(readme, /npmjs\.com|pypi\.org|crates\.io/);
  assert.doesNotMatch(readme, /(?:href|src)="(?!https:\/\/)/);
});

test("Rust README presents a public five-step workflow", async () => {
  const readme = await text("sdk/rust/README.md");
  const headings = [
    "### 1. Install Yield",
    "### 2. Create the workflow",
    "### 3. Test the workflow",
    "### 4. Register the skill",
    "### 5. Run the skill",
  ];

  let previous = -1;
  for (const heading of headings) {
    const current = readme.indexOf(heading);
    assert.ok(current > previous, `${heading} is missing or out of order`);
    previous = current;
  }

  assert.match(readme, /cargo install yieldskill --locked/);
  assert.match(readme, /yskill init skills\/data-migration/);
  assert.match(readme, /yskill doctor skills\/data-migration --test/);
  assert.match(readme, /yskill register skills\/data-migration/);
  assert.match(readme, /^\/data-migration$/m);
  assert.match(readme, /https:\/\/crates\.io\/crates\/yieldskill/);
  assert.match(readme, /https:\/\/docs\.rs\/yieldskill/);
  assert.doesNotMatch(readme, /get\.operatorstack\.systems\/cargo|npmjs\.com|pypi\.org/);
  assert.doesNotMatch(readme, /(?:href|src)="(?!https:\/\/)/);
});

test("Python README presents a public five-step workflow", async () => {
  const readme = await text("sdk/python/README.md");
  const headings = [
    "### 1. Install Yield",
    "### 2. Create the workflow",
    "### 3. Test the workflow",
    "### 4. Register the skill",
    "### 5. Run the skill",
  ];

  let previous = -1;
  for (const heading of headings) {
    const current = readme.indexOf(heading);
    assert.ok(current > previous, `${heading} is missing or out of order`);
    previous = current;
  }

  assert.match(readme, /python -m pip install yieldskill/);
  assert.match(readme, /python -m yieldskill init skills\/env-doctor/);
  assert.match(readme, /python -m yieldskill doctor skills\/env-doctor --test/);
  assert.match(readme, /python -m yieldskill register skills\/env-doctor/);
  assert.match(readme, /^\/env-doctor$/m);
  assert.match(readme, /https:\/\/github\.com\/operatorstack\/yield\/blob\/main\/docs\/reference\/cli\.md/);
  assert.doesNotMatch(readme, /get\.operatorstack\.systems\/pip/);
  assert.doesNotMatch(readme, /npmjs\.com|npm version/);
  assert.doesNotMatch(readme, /(?:href|src)="(?!https:\/\/)/);
});

test("README agent claims match the pinned registry", async () => {
  const [readme, registryText] = await Promise.all([
    text("README.md"),
    text("cmd/yskill/registry/agents.json"),
  ]);
  const registry = JSON.parse(registryText);
  const verified = registry.agents.filter((agent) => agent.tier === "verified");
  const registryBacked = registry.agents.filter((agent) => agent.tier === "registry");
  const normalized = readme.replace(/\s+/g, " ");

  assert.deepEqual(
    verified.map((agent) => agent.id).sort(),
    ["claude-code", "codex", "cursor"],
  );
  assert.match(normalized, /Verified with Cursor, Codex, and Claude Code\./);
  assert.match(
    normalized,
    new RegExp(`Registry-backed project paths are available for ${registryBacked.length} more coding agents\\.`),
  );
  assert.doesNotMatch(readme, /Agent Plugins and Yield/);
});

test("README presents the workflow as five ordered steps", async () => {
  const readme = await text("README.md");
  const headings = [
    "### 1. Install Yield",
    "### 2. Create the workflow",
    "### 3. Test the workflow",
    "### 4. Register the skill",
    "### 5. Run the skill",
  ];

  let previous = -1;
  for (const heading of headings) {
    const current = readme.indexOf(heading);
    assert.ok(current > previous, `${heading} is missing or out of order`);
    previous = current;
  }

  assert.match(readme, /npm exec -- yskill doctor skills\/release --test/);
  assert.match(readme, /npm exec -- yskill register skills\/release/);
  assert.match(readme, /Registration is the discovery step\./);
  assert.match(readme, /^\/release$/m);
  assert.match(readme, /Use the release skill to publish this package\./);
});

test("README adapter paths match every verified agent", async () => {
  const [readme, registryText] = await Promise.all([
    text("README.md"),
    text("cmd/yskill/registry/agents.json"),
  ]);
  const registry = JSON.parse(registryText);

  for (const agent of registry.agents.filter((entry) => entry.tier === "verified")) {
    const adapter = `${agent.project_dir}/release/SKILL.md`;
    assert.match(readme, new RegExp(adapter.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }

  assert.match(readme, /--agent cursor,codex,claude-code/);
  assert.match(readme, /If all three are selected/);
});

test("README uses the borderless Yield mark", async () => {
  const [readme, mark] = await Promise.all([
    text("README.md"),
    text("assets/yield-mark.svg"),
  ]);
  assert.match(
    readme,
    /<img src="https:\/\/raw\.githubusercontent\.com\/operatorstack\/yield\/main\/assets\/yield-mark\.svg" width="96" alt="Yield" \/>/,
  );
  assert.doesNotMatch(readme, /yield-mark\.svg" width="96" height=/);
  assert.doesNotMatch(readme, /apple-touch-icon\.png/);
  assert.match(mark, /width="96" height="96" viewBox="0 0 60 60"/);
  assert.match(mark, /<path d="M18 1h24C53\.4 1 59 6\.6 59 18v24c0 11\.4-5\.6 17-17 17H18C6\.6 59 1 53\.4 1 42V18C1 6\.6 6\.6 1 18 1Z" fill="#fff" stroke="#e5e6e3" stroke-width="\.75"\/>/);
  assert.doesNotMatch(mark, /<rect\b/);
  assert.doesNotMatch(mark, /stroke="#d7d7d1"/);
  assert.match(mark, /<path d="M22 12h17l2 7v22H24l-2-7Z"/);
});

test("README and quickstart use the public documentation and package registries", async () => {
  const [readme, pythonReadme, rustReadme, goReadme, docsIndex, quickstart, agentSetup] = await Promise.all([
    text("README.md"),
    text("sdk/python/README.md"),
    text("sdk/rust/README.md"),
    text("sdk/yield/README.md"),
    text("docs/README.md"),
    text("docs/quickstart.md"),
    text("docs/agent-setup.md"),
  ]);

  assert.match(readme, /href="https:\/\/yield\.operatorstack\.systems\/docs\/">Documentation<\/a>/);
  assert.match(readme, /href="https:\/\/pypi\.org\/project\/yieldskill\/">PyPI<\/a>/);
  assert.match(readme, /href="https:\/\/crates\.io\/crates\/yieldskill">crates\.io<\/a>/);
  assert.match(readme, /href="https:\/\/pkg\.go\.dev\/github\.com\/operatorstack\/yield\/sdk\/yield">pkg\.go\.dev<\/a>/);
  assert.match(pythonReadme, /python -m pip install yieldskill/);
  assert.match(pythonReadme, /https:\/\/pypi\.org\/project\/yieldskill\//);
  assert.doesNotMatch(pythonReadme, /get\.operatorstack\.systems\/pip/);
  assert.match(rustReadme, /cargo install yieldskill --locked/);
  assert.doesNotMatch(rustReadme, /get\.operatorstack\.systems\/cargo/);
  assert.match(goReadme, /go install github\.com\/operatorstack\/yield\/cmd\/yskill@latest/);
  assert.match(docsIndex, /\[public documentation\]\(https:\/\/yield\.operatorstack\.systems\/docs\/\)/);
  const commands = [
    "npm create @operatorstack/yield@latest",
    "uvx --from yieldskill yskill bootstrap --language python",
    "yskill bootstrap --language rust",
    "go run github.com/operatorstack/yield/cmd/yskill@latest bootstrap --language go",
  ];
  for (const command of commands) {
    assert.ok(readme.includes(command), `README is missing ${command}`);
    assert.ok(quickstart.includes(command), `quickstart is missing ${command}`);
    assert.ok(agentSetup.includes(command), `agent setup is missing ${command}`);
  }
  assert.doesNotMatch(quickstart, /get\.operatorstack\.systems\/npm|@operatorstack\/yield@0\./);
  assert.match(quickstart, /Use Yield to turn my release skill into a tested workflow\./);
  assert.match(quickstart, /^## Advanced: build manually$/m);
  assert.match(agentSetup, /^## Run the registered skill$/m);
});

test("root README links survive npm package rendering", async () => {
  const readme = await text("README.md");
  assert.match(readme, /https:\/\/github\.com\/operatorstack\/yield\/blob\/main\/docs\/skill-workflows\.md/);
  assert.match(readme, /https:\/\/github\.com\/operatorstack\/yield\/blob\/main\/evals\/README\.md/);
  assert.match(readme, /https:\/\/github\.com\/operatorstack\/yield\/tree\/main\/examples\/library\//);
  assert.doesNotMatch(readme, /\]\((?!https?:\/\/|#|mailto:)[^)]+\)/);
  assert.doesNotMatch(readme, /href="(?!https?:\/\/|#|mailto:)[^"]+"/);
});

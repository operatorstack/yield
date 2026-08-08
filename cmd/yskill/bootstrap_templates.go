package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func renderBootstrapSkill(language string, profile bootstrapProfile) (map[string]string, string, error) {
	version := profile.YieldVersion
	agents := strings.Join(profile.Agents, ",")
	launcher := map[string]string{
		"typescript": "npm --prefix 'skills/yield-workflow-builder' exec -- yskill",
		"python":     fmt.Sprintf("uvx --from 'yieldskill==%s' yskill", shellQuoteValue(version)),
		"go":         ".yield/bin/yskill",
		"rust":       ".yield/bin/yskill",
	}[language]
	config, err := json.MarshalIndent(map[string]any{
		"version": 1, "yield_version": version, "language": language,
		"launcher": launcher, "agents": profile.Agents,
	}, "", "  ")
	if err != nil {
		return nil, "", err
	}
	files := map[string]string{
		"SKILL.md": fmt.Sprintf(`---
name: yield-workflow-builder
description: Create a tested Yield skill workflow from a description, or convert an existing SKILL.md into one.
---

<!-- generated-by: yskill-bootstrap; version: %s -->

Run from the repository root:

    %s run 'skills/yield-workflow-builder'

Follow each returned operation exactly. Answer each operation directly:

    %s respond <run-id> --value <answer> --skill 'skills/yield-workflow-builder'

Use --result-json for structured agent work. When an operation asks you to
write or repair files, edit the repository before returning the JSON result.
Do not skip an operation or invent its response.

For convert mode, the builder first applies a semantic-disposition projection:

    C = clauses(S)
    Pi(S) = {(c, d, T, r) | c in C}

Each source clause has exactly one disposition: control, guidance, both, or
excluded. Control must reach code. Guidance must remain model-facing in the
canonical SKILL.md or a relevant agent_task. Both must reach both places.
Excluded clauses have no destination and need a reason. Every destination must
remain reachable by the coding agent. The projection stays in the Yield run
log. It is not a generated destination file.
`, version, launcher, launcher),
		"builder.json":            string(config) + "\n",
		"fixtures/responses.json": bootstrapFixtureResponses,
		"fixtures/test.json":      bootstrapFixtureConfig,
	}
	dependency := ""
	switch language {
	case "typescript":
		files["main.ts"] = bootstrapTypeScript
		files["package.json"] = fmt.Sprintf("{\n  \"private\": true,\n  \"type\": \"module\",\n  \"dependencies\": { \"@operatorstack/yield\": \"%s\" }\n}\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"typescript\",\"run\":[\"node\",\"main.ts\"]}\n", version)
		dependency = "npm install --ignore-scripts --no-audit --no-fund (inside skills/yield-workflow-builder)"
	case "python":
		files["main.py"] = bootstrapPython
		files["requirements.txt"] = fmt.Sprintf("yieldskill==%s\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"python\",\"run\":[\"python\",\"main.py\"]}\n", version)
	case "go":
		files["main.go"] = bootstrapGo
		files["go.mod"] = fmt.Sprintf("module yield-workflow-builder\n\ngo 1.26.5\n\nrequire github.com/operatorstack/yield v%s\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"go\",\"run\":[\"go\",\"run\",\"-mod=readonly\",\".\"]}\n", version)
		dependency = "go mod tidy (inside skills/yield-workflow-builder)"
	case "rust":
		files["src/main.rs"] = bootstrapRust
		files["Cargo.toml"] = fmt.Sprintf("[package]\nname = \"yield-workflow-builder\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nyieldskill = { version = \"=%s\" }\nserde_json = \"1\"\n", version)
		files["skill.json"] = fmt.Sprintf("{\"version\":1,\"yield_version\":%q,\"language\":\"rust\",\"run\":[\"cargo\",\"run\",\"--quiet\"]}\n", version)
	default:
		return nil, "", fmt.Errorf("unsupported language %q", language)
	}
	for path, content := range files {
		files[path] = strings.ReplaceAll(content, "{{AGENTS}}", agents)
	}
	return files, dependency, nil
}

const bootstrapFixtureResponses = `{
  "select-mode": {"value": "create"},
  "collect-specification": {
    "name": "yield-workflow-builder-fixture",
    "description": "Create a harmless fixture workflow.",
    "language": "go",
    "destination": "skills/yield-workflow-builder-fixture",
    "source_path": ""
  },
  "extract-flow": {
    "summary": "Ask for confirmation and complete.",
    "steps": [{"id":"confirm","kind":"ask_user","description":"Ask for confirmation."}]
  },
  "write-workflow": {"files":["skills/yield-workflow-builder-fixture/SKILL.md","skills/yield-workflow-builder-fixture/main.go"]}
}
`

const bootstrapFixtureConfig = `{
  "version": 1,
  "setup": [],
  "after_response": {},
  "teardown": []
}
`

const bootstrapTypeScript = `import { existsSync, readFileSync, realpathSync } from "node:fs";
import { dirname, isAbsolute, normalize, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { defineSkill } from "@operatorstack/yield";

type Specification = { name: string; description: string; language: "typescript"|"python"|"go"|"rust"; destination: string; source_path?: string };
type Files = { files: string[] };
type Projection = { ready: boolean; unresolved: string[]; clauses: unknown[] };
const specSchema = {type:"object",required:["name","description","language","destination","source_path"],additionalProperties:false,properties:{name:{type:"string",pattern:"^[a-z0-9]+(?:-[a-z0-9]+)*$"},description:{type:"string",minLength:1},language:{enum:["typescript","python","go","rust"]},destination:{type:"string",minLength:1},source_path:{type:"string"}}};
const projectionSchema = {type:"object",required:["clauses","ready","unresolved"],additionalProperties:false,properties:{clauses:{type:"array",minItems:1,items:{type:"object",required:["source_clause","disposition","destinations","reason"],additionalProperties:false,properties:{source_clause:{type:"string",minLength:1},disposition:{enum:["control","guidance","both","excluded"]},destinations:{type:"array",items:{type:"object",required:["kind","target"],additionalProperties:false,properties:{kind:{enum:["code","skill","agent_task"]},target:{type:"string",minLength:1}}}},reason:{type:"string"}}}},ready:{type:"boolean"},unresolved:{type:"array",items:{type:"string",minLength:1}}}};
const flowSchema = {type:"object",required:["summary","steps"],additionalProperties:false,properties:{summary:{type:"string",minLength:1},steps:{type:"array",minItems:1,items:{type:"object",required:["id","kind","description"],additionalProperties:false,properties:{id:{type:"string",minLength:1},kind:{enum:["ask_user","agent_task","run_command","branch","require"]},description:{type:"string",minLength:1}}}}}};
const filesSchema = {type:"object",required:["files"],additionalProperties:false,properties:{files:{type:"array",minItems:2,items:{type:"string",minLength:1}}}};
const config = JSON.parse(readFileSync(new URL("./builder.json", import.meta.url), "utf8")) as {launcher:string;agents:string[]};
const root = realpathSync(fileURLToPath(new URL("../../", import.meta.url)));
const quote = (v:string) => "'" + v.replaceAll("'", "'\\''") + "'";
const inside = (p:string) => p === root || p.startsWith(root+sep);
const safe = (p:string, destination=false) => !isAbsolute(p) && !normalize(p).split(sep).includes("..") && normalize(p).startsWith("skills"+sep) && inside(realpathSync(destination ? dirname(resolve(root,p)) : resolve(root,p)));

defineSkill((ctx) => {
  const mode = ctx.askUser("select-mode", "Create a workflow from a description, or convert an existing SKILL.md?", [{value:"create",label:"Create"},{value:"convert",label:"Convert"}]);
  const spec = ctx.agentTask<Specification>("collect-specification", "Use the user's current request. Return a safe kebab-case name, a short description, the target language, a new destination under skills/, and source_path for convert mode. Do not write files.", {mode}, specSchema);
  if (!safe(spec.destination, true) || spec.destination === "skills/yield-workflow-builder") ctx.blocked("the destination must be a new path under skills/");
  const available = ctx.runCommand("check-destination", "cd ../.. && test ! -e "+quote(spec.destination), 30);
  if (available.exit_code !== 0) ctx.blocked("the destination must be a new path under skills/");
  let source = "";
  let projection: Projection|null = null;
  if (mode === "convert") {
    if (!spec.source_path || !safe(spec.source_path)) ctx.blocked("the source skill must be inside the repository");
    const sourceFile = resolve(root,spec.source_path,"SKILL.md");
    if (!existsSync(sourceFile)) ctx.blocked("the source SKILL.md does not exist");
    source = readFileSync(sourceFile, "utf8");
    projection = ctx.agentTask<Projection>("project-semantics", "Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.", {source,spec}, projectionSchema);
    if (!projection.ready || projection.unresolved.length > 0) ctx.blocked("the semantic projection has unresolved source clauses");
  }
  const flow = ctx.agentTask("extract-flow", "Extract or design the minimal workflow control flow. Follow the semantic projection. Keep model judgment in agent_task operations. Put order, branches, commands, approvals, evidence requirements, and finish rules in code.", {mode,spec,source,projection}, flowSchema);
  let written = ctx.agentTask<Files>("write-workflow", "Create the complete Yield skill workflow at the destination. Follow every semantic disposition. Write the language program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. A thin SKILL.md removes duplicated sequencing, not useful guidance. Do not edit files outside the destination. Return every file written.", {mode,spec,source,projection,flow}, filesSchema);
  const fixture = spec.destination === "skills/yield-workflow-builder-fixture";
  const base = fixture ? "printf fixture-ok" : "cd ../.. && "+config.launcher+" doctor "+quote(spec.destination)+" --test";
  let checked = ctx.runCommand("verify-generated", base, 600);
  for (let attempt=1; checked.exit_code!==0 && attempt<=2; attempt++) {
    written = ctx.agentTask<Files>("repair-generated-"+attempt, "The generated workflow failed verification. Fix only the destination files. Preserve the source semantics and projection. Return every changed file.", {spec,source,projection,stdout:checked.stdout,stderr:checked.stderr}, filesSchema);
    checked = ctx.runCommand("verify-generated-retry-"+attempt, base, 600);
  }
  if (checked.exit_code!==0) ctx.blocked("the generated workflow still fails after two repair attempts");
  ctx.require(checked.exit_code===0, "the generated workflow passes its fixture run", {exit_code:checked.exit_code});
  const agentFlag = config.agents.join(",");
  const register = fixture ? "printf fixture-register-ok" : "cd ../.. && "+config.launcher+" register "+quote(spec.destination)+" --agent "+quote(agentFlag);
  const registered = ctx.runCommand("register-generated", register, 120);
  ctx.require(registered.exit_code===0, "the generated workflow is registered for the selected coding agents", {exit_code:registered.exit_code});
  const verifyAdapters = fixture ? "printf fixture-adapters-ok" : "cd ../.. && "+config.launcher+" doctor "+quote(spec.destination)+" --agent "+quote(agentFlag)+" --test";
  const adapters = ctx.runCommand("verify-adapters", verifyAdapters, 600);
  ctx.require(adapters.exit_code===0, "the generated coding-agent adapters pass verification", {exit_code:adapters.exit_code});
  return {mode,language:spec.language,destination:spec.destination,files:written.files,verified:true};
});
`

const bootstrapPython = `from pathlib import Path
import json
import os
from yieldskill import define_skill

ROOT = Path(__file__).resolve().parents[2].resolve()
CONFIG = json.loads((Path(__file__).parent / "builder.json").read_text())
SPEC_SCHEMA = {"type":"object","required":["name","description","language","destination","source_path"],"additionalProperties":False,"properties":{"name":{"type":"string","pattern":"^[a-z0-9]+(?:-[a-z0-9]+)*$"},"description":{"type":"string","minLength":1},"language":{"enum":["typescript","python","go","rust"]},"destination":{"type":"string","minLength":1},"source_path":{"type":"string"}}}
PROJECTION_SCHEMA = {"type":"object","required":["clauses","ready","unresolved"],"additionalProperties":False,"properties":{"clauses":{"type":"array","minItems":1,"items":{"type":"object","required":["source_clause","disposition","destinations","reason"],"additionalProperties":False,"properties":{"source_clause":{"type":"string","minLength":1},"disposition":{"enum":["control","guidance","both","excluded"]},"destinations":{"type":"array","items":{"type":"object","required":["kind","target"],"additionalProperties":False,"properties":{"kind":{"enum":["code","skill","agent_task"]},"target":{"type":"string","minLength":1}}}},"reason":{"type":"string"}}}},"ready":{"type":"boolean"},"unresolved":{"type":"array","items":{"type":"string","minLength":1}}}}
FLOW_SCHEMA = {"type":"object","required":["summary","steps"],"additionalProperties":False,"properties":{"summary":{"type":"string","minLength":1},"steps":{"type":"array","minItems":1,"items":{"type":"object","required":["id","kind","description"],"additionalProperties":False,"properties":{"id":{"type":"string","minLength":1},"kind":{"enum":["ask_user","agent_task","run_command","branch","require"]},"description":{"type":"string","minLength":1}}}}}}
FILES_SCHEMA = {"type":"object","required":["files"],"additionalProperties":False,"properties":{"files":{"type":"array","minItems":2,"items":{"type":"string","minLength":1}}}}

def safe(path, destination=False):
    candidate = Path(path)
    if candidate.is_absolute() or ".." in candidate.parts or len(candidate.parts) <= 1 or candidate.parts[0] != "skills": return False
    resolved = ((ROOT / candidate).parent if destination else (ROOT / candidate)).resolve()
    return resolved == ROOT or ROOT in resolved.parents

def quote(value):
    return "'" + value.replace("'", "'\\''") + "'"

def program(ctx):
    mode = ctx.ask_user("select-mode", "Create a workflow from a description, or convert an existing SKILL.md?", options=[{"value":"create","label":"Create"},{"value":"convert","label":"Convert"}])
    spec = ctx.agent_task("collect-specification", "Use the user's current request. Return a safe kebab-case name, a short description, the target language, a new destination under skills/, and source_path for convert mode. Do not write files.", context={"mode":mode}, schema=SPEC_SCHEMA)
    destination = ROOT / spec["destination"]
    if not safe(spec["destination"], True) or spec["destination"] == "skills/yield-workflow-builder":
        ctx.blocked("the destination must be a new path under skills/")
    available = ctx.run_command("check-destination", "cd ../.. && test ! -e " + quote(spec["destination"]), timeout_seconds=30)
    if available.exit_code != 0: ctx.blocked("the destination must be a new path under skills/")
    source = ""
    projection = None
    if mode == "convert":
        if not spec.get("source_path") or not safe(spec["source_path"]): ctx.blocked("the source skill must be inside the repository")
        source_file = ROOT / spec["source_path"] / "SKILL.md"
        if not source_file.is_file(): ctx.blocked("the source SKILL.md does not exist")
        source = source_file.read_text()
        projection = ctx.agent_task("project-semantics", "Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.", context={"source":source,"spec":spec}, schema=PROJECTION_SCHEMA)
        if not projection["ready"] or projection["unresolved"]: ctx.blocked("the semantic projection has unresolved source clauses")
    flow = ctx.agent_task("extract-flow", "Extract or design the minimal workflow control flow. Follow the semantic projection. Keep model judgment in agent_task operations. Put order, branches, commands, approvals, evidence requirements, and finish rules in code.", context={"mode":mode,"spec":spec,"source":source,"projection":projection}, schema=FLOW_SCHEMA)
    written = ctx.agent_task("write-workflow", "Create the complete Yield skill workflow at the destination. Follow every semantic disposition. Write the language program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. A thin SKILL.md removes duplicated sequencing, not useful guidance. Do not edit files outside the destination. Return every file written.", context={"mode":mode,"spec":spec,"source":source,"projection":projection,"flow":flow}, schema=FILES_SCHEMA)
    fixture = spec["destination"] == "skills/yield-workflow-builder-fixture"
    base = "printf fixture-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " doctor " + quote(spec["destination"]) + " --test"
    checked = ctx.run_command("verify-generated", base, timeout_seconds=600)
    for attempt in range(1, 3):
        if checked.exit_code == 0: break
        written = ctx.agent_task(f"repair-generated-{attempt}", "The generated workflow failed verification. Fix only the destination files. Preserve the source semantics and projection. Return every changed file.", context={"spec":spec,"source":source,"projection":projection,"stdout":checked.stdout,"stderr":checked.stderr}, schema=FILES_SCHEMA)
        checked = ctx.run_command(f"verify-generated-retry-{attempt}", base, timeout_seconds=600)
    if checked.exit_code != 0: ctx.blocked("the generated workflow still fails after two repair attempts")
    ctx.require(checked.exit_code == 0, "the generated workflow passes its fixture run", {"exit_code":checked.exit_code})
    agent_flag = ",".join(CONFIG["agents"])
    register = "printf fixture-register-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " register " + quote(spec["destination"]) + " --agent " + quote(agent_flag)
    registered = ctx.run_command("register-generated", register, timeout_seconds=120)
    ctx.require(registered.exit_code == 0, "the generated workflow is registered for the selected coding agents", {"exit_code":registered.exit_code})
    verify = "printf fixture-adapters-ok" if fixture else "cd ../.. && " + CONFIG["launcher"] + " doctor " + quote(spec["destination"]) + " --agent " + quote(agent_flag) + " --test"
    adapters = ctx.run_command("verify-adapters", verify, timeout_seconds=600)
    ctx.require(adapters.exit_code == 0, "the generated coding-agent adapters pass verification", {"exit_code":adapters.exit_code})
    return {"mode":mode,"language":spec["language"],"destination":spec["destination"],"files":written["files"],"verified":True}

define_skill(program)
`

const bootstrapGo = `package main

import (
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"
  "strings"
  "github.com/operatorstack/yield/sdk/yield"
)
type specification struct { Name string ` + "`json:\"name\"`" + `; Description string ` + "`json:\"description\"`" + `; Language string ` + "`json:\"language\"`" + `; Destination string ` + "`json:\"destination\"`" + `; SourcePath string ` + "`json:\"source_path\"`" + ` }
type fileResult struct { Files []string ` + "`json:\"files\"`" + ` }
type projectionResult struct { Ready bool ` + "`json:\"ready\"`" + `; Unresolved []string ` + "`json:\"unresolved\"`" + ` }
type config struct { Launcher string ` + "`json:\"launcher\"`" + `; Agents []string ` + "`json:\"agents\"`" + ` }
const specSchema = ` + "`" + `{"type":"object","required":["name","description","language","destination","source_path"],"additionalProperties":false,"properties":{"name":{"type":"string","pattern":"^[a-z0-9]+(?:-[a-z0-9]+)*$"},"description":{"type":"string","minLength":1},"language":{"enum":["typescript","python","go","rust"]},"destination":{"type":"string","minLength":1},"source_path":{"type":"string"}}}` + "`" + `
const projectionSchema = ` + "`" + `{"type":"object","required":["clauses","ready","unresolved"],"additionalProperties":false,"properties":{"clauses":{"type":"array","minItems":1,"items":{"type":"object","required":["source_clause","disposition","destinations","reason"],"additionalProperties":false,"properties":{"source_clause":{"type":"string","minLength":1},"disposition":{"enum":["control","guidance","both","excluded"]},"destinations":{"type":"array","items":{"type":"object","required":["kind","target"],"additionalProperties":false,"properties":{"kind":{"enum":["code","skill","agent_task"]},"target":{"type":"string","minLength":1}}}},"reason":{"type":"string"}}}},"ready":{"type":"boolean"},"unresolved":{"type":"array","items":{"type":"string","minLength":1}}}}` + "`" + `
const flowSchema = ` + "`" + `{"type":"object","required":["summary","steps"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"steps":{"type":"array","minItems":1,"items":{"type":"object","required":["id","kind","description"],"additionalProperties":false,"properties":{"id":{"type":"string","minLength":1},"kind":{"enum":["ask_user","agent_task","run_command","branch","require"]},"description":{"type":"string","minLength":1}}}}}}` + "`" + `
const filesSchema = ` + "`" + `{"type":"object","required":["files"],"additionalProperties":false,"properties":{"files":{"type":"array","minItems":2,"items":{"type":"string","minLength":1}}}}` + "`" + `
func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func safe(root,value string,destination bool) bool { if filepath.IsAbs(value) { return false }; clean:=filepath.Clean(value); parts:=strings.Split(filepath.ToSlash(clean), "/"); if len(parts)<=1||parts[0]!="skills"||strings.Contains(filepath.ToSlash(clean), "../"){return false};probe:=filepath.Join(root,clean);if destination{probe=filepath.Dir(probe)};canonical,err:=filepath.EvalSymlinks(probe);return err==nil&&strings.HasPrefix(canonical+string(filepath.Separator),root+string(filepath.Separator)) }
func main() { yield.Main(func(ctx *yield.Context) (yield.Outcome,error) {
  rawConfig,err:=os.ReadFile("builder.json"); if err!=nil{return yield.Outcome{},err}; var cfg config; if err=json.Unmarshal(rawConfig,&cfg);err!=nil{return yield.Outcome{},err}
  mode:=ctx.AskUser("select-mode","Create a workflow from a description, or convert an existing SKILL.md?",yield.Option{Value:"create",Label:"Create"},yield.Option{Value:"convert",Label:"Convert"})
  raw:=ctx.AgentTask("collect-specification","Use the user's current request. Return a safe kebab-case name, a short description, the target language, a new destination under skills/, and source_path for convert mode. Do not write files.",map[string]any{"mode":mode},json.RawMessage(specSchema)); var spec specification; if err=json.Unmarshal(raw,&spec);err!=nil{return yield.Outcome{},err}
  root,err:=filepath.Abs(filepath.Join("..","..")); if err!=nil{return yield.Outcome{},err};root,err=filepath.EvalSymlinks(root);if err!=nil{return yield.Outcome{},err}; if !safe(root,spec.Destination,true)||spec.Destination=="skills/yield-workflow-builder" { return yield.Outcome{},ctx.Blocked("the destination must be a new path under skills/") };available:=ctx.RunCommand("check-destination","cd ../.. && test ! -e "+quote(spec.Destination),30);if available.ExitCode!=0{return yield.Outcome{},ctx.Blocked("the destination must be a new path under skills/")}
  source:=""; var projection json.RawMessage; if mode=="convert" { if spec.SourcePath==""||!safe(root,spec.SourcePath,false){return yield.Outcome{},ctx.Blocked("the source skill must be inside the repository")}; b,readErr:=os.ReadFile(filepath.Join(root,spec.SourcePath,"SKILL.md"));if readErr!=nil{return yield.Outcome{},ctx.Blocked("the source SKILL.md does not exist")};source=string(b); projection=ctx.AgentTask("project-semantics","Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.",map[string]any{"source":source,"spec":spec},json.RawMessage(projectionSchema));var projected projectionResult;if err=json.Unmarshal(projection,&projected);err!=nil{return yield.Outcome{},err};if !projected.Ready||len(projected.Unresolved)>0{return yield.Outcome{},ctx.Blocked("the semantic projection has unresolved source clauses")} }
  flow:=ctx.AgentTask("extract-flow","Extract or design the minimal workflow control flow. Follow the semantic projection. Keep model judgment in agent_task operations. Put order, branches, commands, approvals, evidence requirements, and finish rules in code.",map[string]any{"mode":mode,"spec":spec,"source":source,"projection":projection},json.RawMessage(flowSchema))
  writtenRaw:=ctx.AgentTask("write-workflow","Create the complete Yield skill workflow at the destination. Follow every semantic disposition. Write the language program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. A thin SKILL.md removes duplicated sequencing, not useful guidance. Do not edit files outside the destination. Return every file written.",map[string]any{"mode":mode,"spec":spec,"source":source,"projection":projection,"flow":json.RawMessage(flow)},json.RawMessage(filesSchema));var written fileResult;_ = json.Unmarshal(writtenRaw,&written)
  fixture:=spec.Destination=="skills/yield-workflow-builder-fixture"; command:="printf fixture-ok";if !fixture{command="cd ../.. && "+cfg.Launcher+" doctor "+quote(spec.Destination)+" --test"};checked:=ctx.RunCommand("verify-generated",command,600)
  for attempt:=1;checked.ExitCode!=0&&attempt<=2;attempt++{ repair:=ctx.AgentTask(fmt.Sprintf("repair-generated-%d",attempt),"The generated workflow failed verification. Fix only the destination files. Preserve the source semantics and projection. Return every changed file.",map[string]any{"spec":spec,"source":source,"projection":projection,"stdout":checked.Stdout,"stderr":checked.Stderr},json.RawMessage(filesSchema));_ = json.Unmarshal(repair,&written);checked=ctx.RunCommand(fmt.Sprintf("verify-generated-retry-%d",attempt),command,600) }
  if checked.ExitCode!=0{return yield.Outcome{},ctx.Blocked("the generated workflow still fails after two repair attempts")};ctx.Require(checked.ExitCode==0,"the generated workflow passes its fixture run",map[string]any{"exit_code":checked.ExitCode})
  agentFlag:=strings.Join(cfg.Agents,",");register:="printf fixture-register-ok";if !fixture{register="cd ../.. && "+cfg.Launcher+" register "+quote(spec.Destination)+" --agent "+quote(agentFlag)};registered:=ctx.RunCommand("register-generated",register,120);ctx.Require(registered.ExitCode==0,"the generated workflow is registered for the selected coding agents",map[string]any{"exit_code":registered.ExitCode})
  verify:="printf fixture-adapters-ok";if !fixture{verify="cd ../.. && "+cfg.Launcher+" doctor "+quote(spec.Destination)+" --agent "+quote(agentFlag)+" --test"};adapters:=ctx.RunCommand("verify-adapters",verify,600);ctx.Require(adapters.ExitCode==0,"the generated coding-agent adapters pass verification",map[string]any{"exit_code":adapters.ExitCode})
  return ctx.Complete(map[string]any{"mode":mode,"language":spec.Language,"destination":spec.Destination,"files":written.Files,"verified":true})
}) }
`

const bootstrapRust = `use serde_json::{json, Value};
use std::{fs, path::{Path, PathBuf}};
use yieldskill::{define_skill, Context, SkillResult};
const SPEC_SCHEMA:&str=r#"{"type":"object","required":["name","description","language","destination","source_path"],"additionalProperties":false,"properties":{"name":{"type":"string","pattern":"^[a-z0-9]+(?:-[a-z0-9]+)*$"},"description":{"type":"string","minLength":1},"language":{"enum":["typescript","python","go","rust"]},"destination":{"type":"string","minLength":1},"source_path":{"type":"string"}}}"#;
const PROJECTION_SCHEMA:&str=r#"{"type":"object","required":["clauses","ready","unresolved"],"additionalProperties":false,"properties":{"clauses":{"type":"array","minItems":1,"items":{"type":"object","required":["source_clause","disposition","destinations","reason"],"additionalProperties":false,"properties":{"source_clause":{"type":"string","minLength":1},"disposition":{"enum":["control","guidance","both","excluded"]},"destinations":{"type":"array","items":{"type":"object","required":["kind","target"],"additionalProperties":false,"properties":{"kind":{"enum":["code","skill","agent_task"]},"target":{"type":"string","minLength":1}}}},"reason":{"type":"string"}}}},"ready":{"type":"boolean"},"unresolved":{"type":"array","items":{"type":"string","minLength":1}}}}"#;
const FLOW_SCHEMA:&str=r#"{"type":"object","required":["summary","steps"],"additionalProperties":false,"properties":{"summary":{"type":"string","minLength":1},"steps":{"type":"array","minItems":1,"items":{"type":"object","required":["id","kind","description"],"additionalProperties":false,"properties":{"id":{"type":"string","minLength":1},"kind":{"enum":["ask_user","agent_task","run_command","branch","require"]},"description":{"type":"string","minLength":1}}}}}}"#;
const FILES_SCHEMA:&str=r#"{"type":"object","required":["files"],"additionalProperties":false,"properties":{"files":{"type":"array","minItems":2,"items":{"type":"string","minLength":1}}}}"#;
fn safe(root:&Path,value:&str,destination:bool)->bool{let p=Path::new(value);if p.is_absolute()||p.components().any(|c|matches!(c,std::path::Component::ParentDir))||p.components().next().map(|c|c.as_os_str()!="skills").unwrap_or(true)||p.components().count()<=1{return false}let joined=root.join(p);let probe=if destination{joined.parent().unwrap_or(root)}else{joined.as_path()};probe.canonicalize().map(|resolved|resolved.starts_with(root)).unwrap_or(false)}
fn quote(value:&str)->String{format!("'{}'",value.replace('\'',"'\\''"))}
fn program(ctx:&mut Context)->SkillResult{
 let cfg:Value=serde_json::from_slice(&fs::read("builder.json").expect("builder.json must be readable")).expect("builder.json must be valid JSON");
 let mode=ctx.ask_user("select-mode","Create a workflow from a description, or convert an existing SKILL.md?",&[("create","Create"),("convert","Convert")]);
 let spec=ctx.agent_task("collect-specification","Use the user's current request. Return a safe kebab-case name, a short description, the target language, a new destination under skills/, and source_path for convert mode. Do not write files.",Some(json!({"mode":mode})),Some(serde_json::from_str(SPEC_SCHEMA).unwrap()));
 let destination=spec["destination"].as_str().unwrap_or("");let root=PathBuf::from("../..").canonicalize().expect("repository root must be readable");if !safe(&root,destination,true)||destination=="skills/yield-workflow-builder"{return Err(ctx.blocked("the destination must be a new path under skills/"))}let available=ctx.run_command("check-destination",&format!("cd ../.. && test ! -e {}",quote(destination)),30);if available.exit_code!=0{return Err(ctx.blocked("the destination must be a new path under skills/"))}
 let mut source=String::new();let mut projection=Value::Null;if mode=="convert"{let source_path=spec["source_path"].as_str().unwrap_or("");if !safe(&root,source_path,false){return Err(ctx.blocked("the source skill must be inside the repository"))};source=fs::read_to_string(root.join(source_path).join("SKILL.md")).map_err(|_|ctx.blocked("the source SKILL.md does not exist"))?;projection=ctx.agent_task("project-semantics","Map every source clause exactly once using Pi(S)={(c,d,T,r)|c in clauses(S)}. YAML frontmatter is metadata, not a clause. Each top-level bullet is exactly one clause, including a compound sentence. For prose without bullets, treat each paragraph as one clause. Use disposition control, guidance, both, or excluded. Control needs a reachable code destination. Guidance needs a reachable skill or agent_task destination. Both needs both kinds. Excluded needs no destination and a non-empty reason. Name concrete destinations that the writer can create. Use the target language's canonical entrypoint for code: main.ts, main.py, main.go, or src/main.rs. Report uncertainty in unresolved and set ready false. Do not write files.",Some(json!({"source":source,"spec":spec})),Some(serde_json::from_str(PROJECTION_SCHEMA).unwrap()));if !projection["ready"].as_bool().unwrap_or(false)||projection["unresolved"].as_array().map(|v|!v.is_empty()).unwrap_or(true){return Err(ctx.blocked("the semantic projection has unresolved source clauses"))}}
 let flow=ctx.agent_task("extract-flow","Extract or design the minimal workflow control flow. Follow the semantic projection. Keep model judgment in agent_task operations. Put order, branches, commands, approvals, evidence requirements, and finish rules in code.",Some(json!({"mode":mode,"spec":spec,"source":source,"projection":projection})),Some(serde_json::from_str(FLOW_SCHEMA).unwrap()));
 let mut written=ctx.agent_task("write-workflow","Create the complete Yield skill workflow at the destination. Follow every semantic disposition. Write the language program, SKILL.md, exact-version dependencies, skill.json, and self-contained fixtures. A thin SKILL.md removes duplicated sequencing, not useful guidance. Do not edit files outside the destination. Return every file written.",Some(json!({"mode":mode,"spec":spec,"source":source,"projection":projection,"flow":flow})),Some(serde_json::from_str(FILES_SCHEMA).unwrap()));
 let fixture=destination=="skills/yield-workflow-builder-fixture";let launcher=cfg["launcher"].as_str().unwrap();let command=if fixture{"printf fixture-ok".to_string()}else{format!("cd ../.. && {} doctor {} --test",launcher,quote(destination))};let mut checked=ctx.run_command("verify-generated",&command,600);
 for attempt in 1..=2{if checked.exit_code==0{break}written=ctx.agent_task(&format!("repair-generated-{attempt}"),"The generated workflow failed verification. Fix only the destination files. Preserve the source semantics and projection. Return every changed file.",Some(json!({"spec":spec,"source":source,"projection":projection,"stdout":checked.stdout,"stderr":checked.stderr})),Some(serde_json::from_str(FILES_SCHEMA).unwrap()));checked=ctx.run_command(&format!("verify-generated-retry-{attempt}"),&command,600)}
 if checked.exit_code!=0{return Err(ctx.blocked("the generated workflow still fails after two repair attempts"))}ctx.require(true,"the generated workflow passes its fixture run",Some(&json!({"exit_code":checked.exit_code})));
 let agents=cfg["agents"].as_array().unwrap().iter().filter_map(|v|v.as_str()).collect::<Vec<_>>().join(",");let register=if fixture{"printf fixture-register-ok".to_string()}else{format!("cd ../.. && {} register {} --agent {}",launcher,quote(destination),quote(&agents))};let registered=ctx.run_command("register-generated",&register,120);ctx.require(registered.exit_code==0,"the generated workflow is registered for the selected coding agents",Some(&json!({"exit_code":registered.exit_code})));
 let verify=if fixture{"printf fixture-adapters-ok".to_string()}else{format!("cd ../.. && {} doctor {} --agent {} --test",launcher,quote(destination),quote(&agents))};let adapters=ctx.run_command("verify-adapters",&verify,600);ctx.require(adapters.exit_code==0,"the generated coding-agent adapters pass verification",Some(&json!({"exit_code":adapters.exit_code})));
 Ok(json!({"mode":mode,"language":spec["language"],"destination":destination,"files":written["files"],"verified":true}))
}
fn main(){define_skill(program);}
`

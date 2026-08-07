package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

var releaseVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
var tidyGoModule = func(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("prepare Go scaffold dependencies: %w", err)
	}
	return nil
}

func defaultLanguage() string {
	if language := strings.TrimSpace(os.Getenv("YIELD_LANGUAGE")); language != "" {
		return language
	}
	return "go"
}

func packageVersion() string {
	v := runtimeVersion()
	if releaseVersionPattern.MatchString(v) {
		return v
	}
	return "0.0.0"
}

func scaffoldCommand(language, dir string) (launcher, workflow string) {
	launcher = map[string]string{
		"typescript": "npm exec -- yskill",
		"python":     "python -m yieldskill",
		"go":         "yskill",
		"rust":       "yskill",
	}[language]
	workflow = shellQuoteForPlatform(dir, runtime.GOOS)
	if language != "go" && language != "rust" {
		return launcher, workflow
	}
	root, ok := repositoryRootFromLocalRuntime()
	if !ok {
		var err error
		root, err = findRepoRoot(dir, "")
		if err != nil {
			return launcher, workflow
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return launcher, workflow
		}
		if verifyLocalRuntime(localRuntimePath(root), runtimeVersion(), language) != nil {
			return launcher, workflow
		}
	}
	absDir, err := filepath.Abs(dir)
	if err != nil || !within(root, absDir) {
		return launcher, workflow
	}
	runtimeRel, err := filepath.Rel(root, localRuntimePath(root))
	if err != nil {
		return launcher, workflow
	}
	skillRel, err := filepath.Rel(root, absDir)
	if err != nil {
		return launcher, workflow
	}
	return repositoryRuntimeLauncher(runtimeRel, runtime.GOOS), shellQuoteForPlatform(skillRel, runtime.GOOS)
}

func shellQuoteForPlatform(value, goos string) string {
	if goos == "windows" {
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	}
	return shellQuote(filepath.ToSlash(value))
}

func scaffoldSkill(dir, language, sdkPath, description string) error {
	language = strings.ToLower(strings.TrimSpace(language))
	if language != "typescript" && language != "python" && language != "go" && language != "rust" {
		return fmt.Errorf("unsupported language %q; choose typescript, python, go, or rust", language)
	}
	if sdkPath != "" && language != "go" {
		return fmt.Errorf("--sdk is only valid with --language go")
	}
	name := filepath.Base(filepath.Clean(dir))
	if !portableSkillName.MatchString(name) || len(name) > 64 {
		return fmt.Errorf("skill directory name must match [a-z0-9]+(-[a-z0-9]+)* and be at most 64 characters")
	}
	skillPath := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) && strings.TrimSpace(description) == "" {
		return fmt.Errorf("--description is required for a new skill; describe what it does and when an agent should use it")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if language == "go" || language == "rust" {
		if err := pinCurrentRuntime(dir); err != nil {
			return err
		}
	}
	writeIfAbsent := func(rel, content string) error {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("init: %s exists, preserved\n", rel)
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(content), 0o644)
	}

	launcher, workflow := scaffoldCommand(language, dir)
	files := scaffoldFiles(name, language, sdkPath)
	files["SKILL.md"] = fmt.Sprintf(skillMD, name, yamlString(strings.TrimSpace(description)), launcher, workflow, launcher, workflow)
	files["fixtures/responses.json"] = "{\n  \"confirm-start\": {\"value\": \"yes\"}\n}\n"
	files["fixtures/test.json"] = "{\n  \"version\": 1,\n  \"setup\": [],\n  \"after_response\": {},\n  \"teardown\": []\n}\n"
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, rel := range keys {
		if err := writeIfAbsent(rel, files[rel]); err != nil {
			return err
		}
	}
	if language == "go" {
		if err := tidyGoModule(dir); err != nil {
			return err
		}
	}
	if _, err := readSkillMetadata(dir); err != nil {
		return fmt.Errorf("validate SKILL.md: %w", err)
	}
	fmt.Printf("init: %s skill %q scaffolded in %s\n", language, name, dir)
	fmt.Println("next: replace the starter program and fixtures with the described workflow")
	fmt.Printf("test: %s doctor %s --test\n", launcher, workflow)
	fmt.Printf("then: %s register %s\n", launcher, workflow)
	return nil
}

func pinCurrentRuntime(dir string) error {
	root, err := findRepoRoot(dir, "")
	if err != nil {
		return nil
	}
	source := strings.TrimSpace(os.Getenv("YIELD_LAUNCHER_PATH"))
	if source == "" {
		source, err = currentExecutable()
		if err != nil {
			return fmt.Errorf("locate current Yield runtime: %w", err)
		}
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve current Yield runtime: %w", err)
	}
	name := strings.ToLower(filepath.Base(source))
	if name != "yskill" && name != "yskill.exe" {
		return nil
	}
	destination := localRuntimePath(root)
	if filepath.Clean(source) == filepath.Clean(destination) {
		return ensureLocalStateIgnored(root)
	}
	got, err := inspectRuntimeVersion(source)
	if err != nil {
		return fmt.Errorf("verify current Yield runtime: %w", err)
	}
	if expected := runtimeVersion(); got != expected {
		return fmt.Errorf("current Yield launcher version is %s, but the runtime is %s", got, expected)
	}
	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read current Yield runtime: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".yskill-pin-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("pin repository-local Yield runtime: %w", err)
	}
	if err := ensureLocalStateIgnored(root); err != nil {
		return err
	}
	fmt.Printf("init: pinned Yield %s at %s\n", got, filepath.ToSlash(destination))
	return nil
}

func scaffoldFiles(name, language, sdkPath string) map[string]string {
	v := packageVersion()
	switch language {
	case "typescript":
		return map[string]string{
			"main.ts": mainTypeScript,
			"package.json": fmt.Sprintf(`{
  "private": true,
  "type": "module",
  "dependencies": { "@operatorstack/yield": "%s" }
}
`, v),
			"skill.json": "{\"version\":1,\"language\":\"typescript\",\"run\":[\"node\",\"main.ts\"]}\n",
		}
	case "python":
		return map[string]string{
			"main.py":          mainPython,
			"requirements.txt": fmt.Sprintf("yieldskill==%s\n", v),
			"skill.json":       "{\"version\":1,\"language\":\"python\",\"run\":[\"python\",\"main.py\"]}\n",
		}
	case "rust":
		return map[string]string{
			"Cargo.toml":  fmt.Sprintf("[package]\nname = %q\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nyieldskill = { version = \"=%s\" }\nserde_json = \"1\"\n", name, v),
			"src/main.rs": mainRust,
			"skill.json":  fmt.Sprintf("{\"version\":1,\"language\":\"rust\",\"run\":[\"cargo\",\"run\",\"--quiet\",\"--bin\",%q]}\n", name),
		}
	default:
		gomod := fmt.Sprintf("module %s\n\ngo 1.26.5\n\nrequire github.com/operatorstack/yield v%s\n", name, v)
		if sdkPath != "" {
			gomod += fmt.Sprintf("\nreplace github.com/operatorstack/yield => %s\n", sdkPath)
		}
		return map[string]string{
			"main.go":    mainGo,
			"go.mod":     gomod,
			"skill.json": "{\"version\":1,\"language\":\"go\",\"run\":[\"go\",\"run\",\"-mod=readonly\",\".\"]}\n",
		}
	}
}

const skillMD = `---
name: %s
description: %s
---

Run:

    %s run %s

Follow each returned operation exactly. Answer it directly:

	%s respond <run-id> --value <answer> --skill %s

Do not skip an operation or invent a response.
`

const mainGo = `package main

import "github.com/operatorstack/yield/sdk/yield"

func main() {
	yield.Main(func(ctx *yield.Context) (yield.Outcome, error) {
		answer := ctx.AskUser("confirm-start", "Ready to start?")
		if answer != "yes" {
			return yield.Outcome{}, ctx.Refused("user declined to start")
		}
		return yield.Outcome{}, ctx.Blocked("replace the starter workflow and fixture before testing")
	})
}
`

const mainTypeScript = `import { defineSkill } from "@operatorstack/yield"

defineSkill((ctx) => {
  const answer = ctx.askUser("confirm-start", "Ready to start?", [
    { value: "yes" }, { value: "no" }
  ])
  if (answer !== "yes") ctx.refused("user declined to start")
  ctx.blocked("replace the starter workflow and fixture before testing")
})
`

const mainPython = `from yieldskill import define_skill

def program(ctx):
    answer = ctx.ask_user("confirm-start", "Ready to start?", options=[{"value": "yes"}, {"value": "no"}])
    if answer != "yes":
        ctx.refused("user declined to start")
    ctx.blocked("replace the starter workflow and fixture before testing")

define_skill(program)
`

const mainRust = `use serde_json::json;
use yieldskill::{define_skill, Context, SkillResult};

fn program(ctx: &mut Context) -> SkillResult {
    let answer = ctx.ask_user("confirm-start", "Ready to start?", &[("yes", "Yes"), ("no", "No")]);
    if answer != "yes" {
        return Err(ctx.refused("user declined to start"));
    }
    Err(ctx.blocked("replace the starter workflow and fixture before testing"))
}

fn main() { define_skill(program); }
`

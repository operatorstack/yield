package main

import (
	"flag"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestRuntimeVersionUsesGoModuleVersion(t *testing.T) {
	previousVersion := version
	previousReadBuildInfo := readBuildInfo
	version = "dev"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true
	}
	t.Cleanup(func() {
		version = previousVersion
		readBuildInfo = previousReadBuildInfo
	})
	if got := runtimeVersion(); got != "1.2.3" {
		t.Fatalf("runtimeVersion = %q, want 1.2.3", got)
	}
}

func TestParseOnePositionalAllowsDocumentedFlagOrder(t *testing.T) {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	response := fs.String("response", "", "response file")
	skill := fs.String("skill", ".", "skill directory")
	if err := parseOnePositional(fs, []string{"run_123", "--response", "response.json", "--skill", "skills/release"}); err != nil {
		t.Fatal(err)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "run_123" {
		t.Fatalf("positional args = %q, want run_123", fs.Args())
	}
	if *response != "response.json" || *skill != "skills/release" {
		t.Fatalf("flags = response %q skill %q", *response, *skill)
	}
}

func TestParseOnePositionalKeepsFlagFirstOrder(t *testing.T) {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	response := fs.String("response", "", "response file")
	if err := parseOnePositional(fs, []string{"--response", "response.json", "run_123"}); err != nil {
		t.Fatal(err)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "run_123" || *response != "response.json" {
		t.Fatalf("args = %q response = %q", fs.Args(), *response)
	}
}

func TestScaffoldSkillWritesLanguageSpecificEntrypoints(t *testing.T) {
	previousVersion := version
	previousTidyGoModule := tidyGoModule
	version = "0.1.9"
	tidyCalls := 0
	tidyGoModule = func(string) error {
		tidyCalls++
		return nil
	}
	t.Cleanup(func() {
		version = previousVersion
		tidyGoModule = previousTidyGoModule
	})

	tests := []struct {
		language string
		files    []string
		command  string
		pin      string
	}{
		{"typescript", []string{"main.ts", "package.json", "skill.json"}, "npm exec -- yskill run .", `"@operatorstack/yield": "0.1.9"`},
		{"python", []string{"main.py", "requirements.txt", "skill.json"}, "python -m yieldskill run .", "yieldskill==0.1.9"},
		{"go", []string{"main.go", "go.mod", "skill.json"}, "yskill run .", "github.com/operatorstack/yield v0.1.9"},
		{"rust", []string{"src/main.rs", "Cargo.toml", ".cargo/config.toml", "skill.json"}, "yskill run .", `version = "=0.1.9"`},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "My Skill")
			if err := scaffoldSkill(dir, tt.language, ""); err != nil {
				t.Fatal(err)
			}
			for _, rel := range append(tt.files, "SKILL.md", "fixtures/responses.json") {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
					t.Fatalf("%s: %v", rel, err)
				}
			}
			skill := readTestFile(t, filepath.Join(dir, "SKILL.md"))
			if !strings.Contains(skill, tt.command) {
				t.Fatalf("SKILL.md does not contain %q:\n%s", tt.command, skill)
			}
			var manifest string
			switch tt.language {
			case "typescript":
				manifest = readTestFile(t, filepath.Join(dir, "package.json"))
			case "python":
				manifest = readTestFile(t, filepath.Join(dir, "requirements.txt"))
			case "go":
				manifest = readTestFile(t, filepath.Join(dir, "go.mod"))
			case "rust":
				manifest = readTestFile(t, filepath.Join(dir, "Cargo.toml"))
			}
			if !strings.Contains(manifest, tt.pin) {
				t.Fatalf("manifest does not contain %q:\n%s", tt.pin, manifest)
			}
		})
	}
	if tidyCalls != 1 {
		t.Fatalf("go mod tidy calls = %d, want 1", tidyCalls)
	}
}

func TestGoScaffoldCanResolveItsPinnedModuleOnFirstRun(t *testing.T) {
	previousVersion := version
	previousTidyGoModule := tidyGoModule
	version = "0.1.9"
	tidyGoModule = func(string) error { return nil }
	t.Cleanup(func() {
		version = previousVersion
		tidyGoModule = previousTidyGoModule
	})
	dir := filepath.Join(t.TempDir(), "go-skill")
	if err := scaffoldSkill(dir, "go", ""); err != nil {
		t.Fatal(err)
	}
	manifest := readTestFile(t, filepath.Join(dir, "skill.json"))
	if manifest != "{\"run\":[\"go\",\"run\",\"-mod=readonly\",\".\"]}\n" {
		t.Fatalf("skill.json = %q", manifest)
	}
}

func TestPythonScaffoldUsesInvokingInterpreter(t *testing.T) {
	previousVersion := version
	version = "0.1.9"
	t.Cleanup(func() { version = previousVersion })
	t.Setenv("YIELD_PYTHON", "/opt/yield/.venv/bin/python")
	dir := filepath.Join(t.TempDir(), "python-skill")
	if err := scaffoldSkill(dir, "python", ""); err != nil {
		t.Fatal(err)
	}
	skill := readTestFile(t, filepath.Join(dir, "skill.json"))
	if !strings.Contains(skill, `"/opt/yield/.venv/bin/python"`) {
		t.Fatalf("skill.json does not use the invoking interpreter: %s", skill)
	}
}

func TestScaffoldSkillPreservesExistingSkillAndRejectsUnknownLanguage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldSkill(dir, "typescript", ""); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(dir, "SKILL.md")); got != "keep me\n" {
		t.Fatalf("existing SKILL.md changed: %q", got)
	}
	if err := scaffoldSkill(t.TempDir(), "java", ""); err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("unknown language error = %v", err)
	}
}

func TestPackageVersionFallsBackForDevelopmentBuilds(t *testing.T) {
	previousVersion := version
	version = "dev"
	t.Cleanup(func() { version = previousVersion })
	if got := packageVersion(); got != "0.0.0" {
		t.Fatalf("packageVersion = %q", got)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

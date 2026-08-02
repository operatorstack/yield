package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/operatorstack/yield/internal/engine"
	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

func TestPrintProgressKeepsCompleteStructuredResult(t *testing.T) {
	result := `{"summary":"` + strings.Repeat("x", 300) + `","count":42}`
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = previous })
	callErr := printProgress(&engine.Progress{RunID: "run_test", Terminal: &protocol.TerminalOutcome{
		Status: protocol.StatusCompleted, Result: json.RawMessage(result),
	}})
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = previous
	output, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if callErr != nil {
		t.Fatal(callErr)
	}
	if !strings.Contains(string(output), result) {
		t.Fatalf("terminal result was truncated: %s", output)
	}
}

func TestFixtureCommandReceivesResponseOnStdin(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "effect.json")
	input := []byte(`{"value":"approved"}`)
	command := []string{os.Args[0], "-test.run=TestFixtureCommandHelper", "--", output}
	if err := runFixtureCommands(dir, [][]string{command}, input); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(input) {
		t.Fatalf("fixture stdin = %q, want %q", got, input)
	}
}

func TestFixtureCommandHelper(t *testing.T) {
	if os.Getenv("YIELD_FIXTURE") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(os.Args[separator+1], input, 0o600); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

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

func TestPruneRemovesOnlyOldTerminalRuns(t *testing.T) {
	skill := t.TempDir()
	runs := filepath.Join(skill, ".yield", "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	makeRun := func(id string, closed bool) string {
		log, err := runlog.Create(runs, id)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := log.Append(runlog.RunStarted, map[string]any{"run_id": id}); err != nil {
			t.Fatal(err)
		}
		if closed {
			if _, err := log.Append(runlog.RunCompleted, map[string]any{"result": "ok"}); err != nil {
				t.Fatal(err)
			}
		}
		old := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(log.Path, old, old); err != nil {
			t.Fatal(err)
		}
		return log.Path
	}
	closed := makeRun("run_closed", true)
	active := makeRun("run_active", false)
	if err := cmdPrune([]string{skill, "--older-than", "24h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(closed); !os.IsNotExist(err) {
		t.Fatalf("terminal run was not pruned: %v", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active run was pruned: %v", err)
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
			dir := filepath.Join(t.TempDir(), "my-skill")
			if err := scaffoldSkill(dir, tt.language, "", "Run the test workflow when checking Yield setup."); err != nil {
				t.Fatal(err)
			}
			for _, rel := range append(tt.files, "SKILL.md", "fixtures/responses.json", "fixtures/test.json") {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
					t.Fatalf("%s: %v", rel, err)
				}
			}
			generated, err := readSkillManifest(dir)
			if err != nil {
				t.Fatal(err)
			}
			if generated.Version != 1 || generated.Language != tt.language {
				t.Fatalf("skill.json = version %d language %q", generated.Version, generated.Language)
			}
			skill := readTestFile(t, filepath.Join(dir, "SKILL.md"))
			if !strings.Contains(skill, tt.command) {
				t.Fatalf("SKILL.md does not contain %q:\n%s", tt.command, skill)
			}
			entrypoint := map[string]string{
				"typescript": "main.ts",
				"python":     "main.py",
				"go":         "main.go",
				"rust":       "src/main.rs",
			}[tt.language]
			program := readTestFile(t, filepath.Join(dir, filepath.FromSlash(entrypoint)))
			if !strings.Contains(program, "replace the starter workflow and fixture before testing") {
				t.Fatalf("%s starter can pass without implementation:\n%s", entrypoint, program)
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
	if err := scaffoldSkill(dir, "go", "", "Run the Go workflow when checking Yield setup."); err != nil {
		t.Fatal(err)
	}
	manifest := readTestFile(t, filepath.Join(dir, "skill.json"))
	if manifest != "{\"version\":1,\"language\":\"go\",\"run\":[\"go\",\"run\",\"-mod=readonly\",\".\"]}\n" {
		t.Fatalf("skill.json = %q", manifest)
	}
}

func TestPythonScaffoldUsesRelocatableInterpreter(t *testing.T) {
	previousVersion := version
	version = "0.1.9"
	t.Cleanup(func() { version = previousVersion })
	t.Setenv("YIELD_PYTHON", "/opt/yield/.venv/bin/python")
	dir := filepath.Join(t.TempDir(), "python-skill")
	if err := scaffoldSkill(dir, "python", "", "Run the Python workflow when checking Yield setup."); err != nil {
		t.Fatal(err)
	}
	skill := readTestFile(t, filepath.Join(dir, "skill.json"))
	if !strings.Contains(skill, `"run":["python","main.py"]`) || strings.Contains(skill, `/opt/yield`) {
		t.Fatalf("skill.json is not relocatable: %s", skill)
	}
}

func TestScaffoldSkillPreservesExistingSkillAndRejectsUnknownLanguage(t *testing.T) {
	dir := t.TempDir()
	existing := "---\nname: " + filepath.Base(dir) + "\ndescription: Keep this existing workflow when wrapping it with Yield.\n---\n\nKeep me.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldSkill(dir, "typescript", "", ""); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(dir, "SKILL.md")); got != existing {
		t.Fatalf("existing SKILL.md changed: %q", got)
	}
	if err := scaffoldSkill(t.TempDir(), "java", "", "A valid description for the invalid language test."); err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("unknown language error = %v", err)
	}
}

func TestScaffoldRequiresDescriptionForNewSkill(t *testing.T) {
	if err := scaffoldSkill(filepath.Join(t.TempDir(), "new-skill"), "typescript", "", ""); err == nil || !strings.Contains(err.Error(), "--description is required") {
		t.Fatalf("missing description error = %v", err)
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

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const bootstrapSkillName = "yield-workflow-builder"

type bootstrapProfile struct {
	Version         int      `json:"version"`
	YieldVersion    string   `json:"yield_version"`
	Language        string   `json:"language"`
	LauncherProfile string   `json:"launcher_profile"`
	Agents          []string `json:"agents"`
}

type bootstrapPlan struct {
	Root     string
	Language string
	SkillDir string
	Profile  bootstrapProfile
	Agents   []agentConfig
	Files    map[string]string
	Adapters []string
}

type bootstrapOperation struct {
	kind   string
	dir    string
	name   string
	args   []string
	agents []string
}

const (
	bootstrapOperationCommand  = "command"
	bootstrapOperationDoctor   = "doctor"
	bootstrapOperationRegister = "register"
)

var bootstrapInput io.Reader = os.Stdin
var bootstrapCommand = func(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
var bootstrapDoctor = func(skillDir, root string, agents []string) error {
	args := []string{skillDir, "--root", root, "--test"}
	for _, agent := range agents {
		args = append(args, "--agent", agent)
	}
	return cmdDoctor(args)
}

func cmdBootstrap(args []string) error {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	language := fs.String("language", "", "workflow-builder language (detected by default)")
	root := fs.String("root", "", "repository root")
	dryRun := fs.Bool("dry-run", false, "print the plan without writing files")
	yes := fs.Bool("yes", false, "apply the printed plan without prompting")
	var requested agentListFlag
	fs.Var(&requested, "agent", "agent id, comma-separated ids, or auto")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("helper install takes no positional arguments")
	}
	plan, err := makeBootstrapPlan(*root, *language, requested)
	if err != nil {
		return err
	}
	printBootstrapPlan(plan)
	if *dryRun {
		fmt.Println("helper: dry run complete; no files changed")
		return nil
	}
	if !*yes && !confirmBootstrap(bootstrapInput) {
		fmt.Println("helper: cancelled; no files changed")
		return nil
	}
	if err := applyBootstrapPlan(plan); err != nil {
		return err
	}
	if err := runBootstrapOperations(plan, bootstrapOperations(plan)); err != nil {
		return err
	}
	fmt.Println("helper: Yield developer helper is ready")
	fmt.Println("next: restart your coding agent, then ask it to create or convert a skill workflow")
	fmt.Println("learn: Use Yield to explain how coded skill workflows work.")
	fmt.Println("create: Use Yield to create a tested skill workflow for releasing my package.")
	fmt.Println("convert: Use Yield to convert my existing release SKILL.md into a tested skill workflow.")
	return nil
}

func makeBootstrapPlan(rootArg, language string, requested []string) (bootstrapPlan, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	cwd, err := os.Getwd()
	if err != nil {
		return bootstrapPlan{}, err
	}
	root, err := findRepoRoot(cwd, rootArg)
	if err != nil {
		return bootstrapPlan{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return bootstrapPlan{}, fmt.Errorf("resolve repository root: %w", err)
	}
	if language == "" {
		language, err = detectBootstrapLanguage(root)
		if err != nil {
			return bootstrapPlan{}, err
		}
	}
	if language != "typescript" && language != "python" && language != "go" && language != "rust" {
		return bootstrapPlan{}, fmt.Errorf("unsupported language %q; choose typescript, python, go, or rust", language)
	}
	registry, err := loadAgentRegistry()
	if err != nil {
		return bootstrapPlan{}, err
	}
	agents, err := selectAgents(registry, requested, root)
	if err != nil {
		return bootstrapPlan{}, err
	}
	ids := make([]string, 0, len(agents))
	adapters := make([]string, 0, len(agents))
	for _, agent := range agents {
		ids = append(ids, agent.ID)
		adapters = append(adapters, filepath.Join(root, filepath.FromSlash(agent.ProjectDir), bootstrapSkillName, "SKILL.md"))
	}
	profile := bootstrapProfile{
		Version: 1, YieldVersion: packageVersion(), Language: language, Agents: ids,
		LauncherProfile: map[string]string{"typescript": "typescript-npm", "python": "python-uvx", "go": "repository-runtime", "rust": "repository-runtime"}[language],
	}
	files, _, err := renderBootstrapSkill(language, profile)
	if err != nil {
		return bootstrapPlan{}, err
	}
	skillDir := filepath.Join(root, "skills", bootstrapSkillName)
	ownedSkill := false
	if info, statErr := os.Stat(skillDir); statErr == nil {
		if !info.IsDir() {
			return bootstrapPlan{}, fmt.Errorf("workflow-builder path is not a directory: %s", skillDir)
		}
		b, readErr := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
		if readErr != nil || !strings.Contains(string(b), "generated-by: yskill-bootstrap") {
			return bootstrapPlan{}, fmt.Errorf("refusing to overwrite user-owned directory %s", skillDir)
		}
		var existing struct {
			Language string `json:"language"`
		}
		if config, readErr := os.ReadFile(filepath.Join(skillDir, "builder.json")); readErr != nil || json.Unmarshal(config, &existing) != nil || existing.Language == "" {
			return bootstrapPlan{}, fmt.Errorf("refusing to update workflow builder with missing or invalid ownership metadata: %s", skillDir)
		}
		if existing.Language != language {
			return bootstrapPlan{}, fmt.Errorf("workflow builder already uses %s; remove the generated builder before selecting %s", existing.Language, language)
		}
		ownedSkill = true
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return bootstrapPlan{}, statErr
	}
	for rel := range files {
		if err := preflightBootstrapPath(root, filepath.Join(skillDir, filepath.FromSlash(rel)), ownedSkill); err != nil {
			return bootstrapPlan{}, err
		}
	}
	for _, path := range adapters {
		if err := preflightBootstrapAdapter(root, path); err != nil {
			return bootstrapPlan{}, err
		}
	}
	return bootstrapPlan{Root: root, Language: language, SkillDir: skillDir, Profile: profile, Agents: agents, Files: files, Adapters: adapters}, nil
}

func preflightBootstrapPath(root, path string, ownedSkill bool) error {
	if !within(root, path) {
		return fmt.Errorf("bootstrap path escapes repository: %s", path)
	}
	if err := ensureContainedWrite(root, path); err != nil {
		return err
	}
	if _, err := os.ReadFile(path); err == nil {
		if !ownedSkill {
			return fmt.Errorf("refusing to overwrite user-owned file %s", path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func preflightBootstrapAdapter(root, path string) error {
	if err := ensureContainedWrite(root, path); err != nil {
		return err
	}
	if b, err := os.ReadFile(path); err == nil {
		if !strings.Contains(string(b), generatedAdapterPrefix+"skills/"+bootstrapSkillName+";") {
			return fmt.Errorf("refusing to overwrite user-owned agent skill %s", path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func printBootstrapPlan(plan bootstrapPlan) {
	fmt.Printf("helper install plan: language=%s root=%s\n", plan.Language, plan.Root)
	paths := make([]string, 0, len(plan.Files)+len(plan.Adapters)+1)
	for rel := range plan.Files {
		paths = append(paths, filepath.Join(plan.SkillDir, filepath.FromSlash(rel)))
	}
	if plan.Language == "typescript" {
		paths = append(paths, filepath.Join(plan.SkillDir, "package-lock.json"))
	}
	if plan.Language == "go" {
		paths = append(paths, filepath.Join(plan.SkillDir, "go.sum"))
	}
	if plan.Language == "rust" {
		paths = append(paths, filepath.Join(plan.SkillDir, "Cargo.lock"))
	}
	paths = append(paths, filepath.Join(plan.Root, ".yield", "bootstrap.json"))
	paths = append(paths, filepath.Join(plan.Root, ".yield", ".gitignore"))
	if plan.Language == "go" || plan.Language == "rust" {
		paths = append(paths, localRuntimePath(plan.Root))
	}
	paths = append(paths, plan.Adapters...)
	sort.Strings(paths)
	for _, path := range paths {
		rel, _ := filepath.Rel(plan.Root, path)
		fmt.Printf("  write %s\n", filepath.ToSlash(rel))
	}
	for _, operation := range bootstrapOperations(plan) {
		fmt.Printf("  run   %s\n", renderBootstrapOperation(plan, operation))
	}
}

func bootstrapOperations(plan bootstrapPlan) []bootstrapOperation {
	ids := make([]string, 0, len(plan.Agents))
	for _, agent := range plan.Agents {
		ids = append(ids, agent.ID)
	}
	operations := make([]bootstrapOperation, 0, 4)
	switch plan.Language {
	case "typescript":
		operations = append(operations, bootstrapOperation{kind: bootstrapOperationCommand, dir: plan.SkillDir, name: "npm", args: []string{"install", "--ignore-scripts", "--no-audit", "--no-fund"}})
	case "go":
		operations = append(operations, bootstrapOperation{kind: bootstrapOperationCommand, dir: plan.SkillDir, name: "go", args: []string{"mod", "tidy"}})
	}
	operations = append(operations,
		bootstrapOperation{kind: bootstrapOperationDoctor},
		bootstrapOperation{kind: bootstrapOperationRegister, agents: append([]string(nil), ids...)},
		bootstrapOperation{kind: bootstrapOperationDoctor, agents: append([]string(nil), ids...)},
	)
	return operations
}

func renderBootstrapOperation(plan bootstrapPlan, operation bootstrapOperation) string {
	if operation.kind == bootstrapOperationCommand {
		parts := []string{"cd", shellQuoteForPlatform(operation.dir, runtime.GOOS), "&&", operation.name}
		for _, arg := range operation.args {
			parts = append(parts, shellQuoteForPlatform(arg, runtime.GOOS))
		}
		return strings.Join(parts, " ")
	}
	launcher := "yskill"
	if plan.Language == "go" || plan.Language == "rust" {
		launcher = repositoryRuntimeLauncher(filepath.Join(".yield", "bin", "yskill"), runtime.GOOS)
	}
	args := []string{launcher}
	switch operation.kind {
	case bootstrapOperationDoctor:
		args = append(args, "doctor", plan.SkillDir, "--root", plan.Root)
		for _, agent := range operation.agents {
			args = append(args, "--agent", agent)
		}
		args = append(args, "--test")
	case bootstrapOperationRegister:
		args = append(args, "register", plan.SkillDir, "--root", plan.Root)
		for _, agent := range operation.agents {
			args = append(args, "--agent", agent)
		}
	}
	for index := 1; index < len(args); index++ {
		args[index] = shellQuoteForPlatform(args[index], runtime.GOOS)
	}
	return strings.Join(args, " ")
}

func runBootstrapOperations(plan bootstrapPlan, operations []bootstrapOperation) error {
	for _, operation := range operations {
		switch operation.kind {
		case bootstrapOperationCommand:
			if err := bootstrapCommand(operation.dir, operation.name, operation.args...); err != nil {
				return fmt.Errorf("prepare workflow-builder dependencies: %w", err)
			}
		case bootstrapOperationDoctor:
			if err := bootstrapDoctor(plan.SkillDir, plan.Root, operation.agents); err != nil {
				if len(operation.agents) == 0 {
					return fmt.Errorf("verify workflow builder: %w", err)
				}
				return fmt.Errorf("verify workflow builder adapters: %w", err)
			}
		case bootstrapOperationRegister:
			registrations, err := registerSkill(plan.SkillDir, plan.Root, operation.agents)
			if err != nil {
				return fmt.Errorf("register workflow builder: %w", err)
			}
			for _, item := range registrations {
				fmt.Printf("registered: %-22s %s\n", item.AgentID, item.Path)
			}
		default:
			return fmt.Errorf("unknown helper install operation %q", operation.kind)
		}
	}
	return nil
}

func detectBootstrapLanguage(root string) (string, error) {
	candidates := []struct {
		language string
		files    []string
	}{
		{language: "typescript", files: []string{"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock"}},
		{language: "python", files: []string{"pyproject.toml", "requirements.txt", "uv.lock"}},
		{language: "rust", files: []string{"Cargo.toml", "Cargo.lock"}},
		{language: "go", files: []string{"go.mod", "go.work"}},
	}
	var found []string
	for _, candidate := range candidates {
		for _, name := range candidate.files {
			if _, err := os.Stat(filepath.Join(root, name)); err == nil {
				found = append(found, candidate.language)
				break
			}
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) == 0 {
		return "", fmt.Errorf("cannot detect a project language; pass --language typescript, python, rust, or go")
	}
	return "", fmt.Errorf("multiple project languages detected (%s); pass --language explicitly", strings.Join(found, ", "))
}

func confirmBootstrap(input io.Reader) bool {
	fmt.Print("Apply this helper install plan? [y/N] ")
	scanner := bufio.NewScanner(input)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

func applyBootstrapPlan(plan bootstrapPlan) error {
	if err := ensureLocalStateIgnored(plan.Root); err != nil {
		return err
	}
	profileBytes, err := json.MarshalIndent(plan.Profile, "", "  ")
	if err != nil {
		return err
	}
	if err := writeBootstrapFile(filepath.Join(plan.Root, ".yield", "bootstrap.json"), string(profileBytes)+"\n"); err != nil {
		return err
	}
	keys := make([]string, 0, len(plan.Files))
	for rel := range plan.Files {
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	for _, rel := range keys {
		if rel == ".gitignore" {
			if err := writeBootstrapFileIfAbsent(filepath.Join(plan.SkillDir, filepath.FromSlash(rel)), plan.Files[rel]); err != nil {
				return err
			}
			continue
		}
		if err := writeBootstrapFile(filepath.Join(plan.SkillDir, filepath.FromSlash(rel)), plan.Files[rel]); err != nil {
			return err
		}
	}
	if plan.Language == "go" || plan.Language == "rust" {
		if err := pinRuntimeAtRoot(plan.Root); err != nil {
			return err
		}
	}
	return nil
}

func writeBootstrapFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".yield-bootstrap-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func writeBootstrapFileIfAbsent(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func readBootstrapProfile(repoRoot string) (bootstrapProfile, error) {
	path := filepath.Join(repoRoot, ".yield", "bootstrap.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return bootstrapProfile{}, nil
	}
	if err != nil {
		return bootstrapProfile{}, err
	}
	var profile bootstrapProfile
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&profile); err != nil {
		return bootstrapProfile{}, fmt.Errorf("bootstrap profile does not decode: %w", err)
	}
	if profile.Version != 1 {
		return bootstrapProfile{}, fmt.Errorf("bootstrap profile version must be 1")
	}
	return profile, nil
}

func shellQuoteValue(value string) string {
	return strings.ReplaceAll(value, "'", "'\\''")
}

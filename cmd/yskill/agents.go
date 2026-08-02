package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/operatorstack/yield/internal/protocol"
)

//go:embed registry/agents.json
var agentRegistryJSON []byte

const generatedAdapterPrefix = "<!-- generated-by: yskill; source: "

type agentRegistry struct {
	Source struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
		License    string `json:"license"`
		Path       string `json:"path"`
	} `json:"source"`
	Agents []agentConfig `json:"agents"`
}

type agentConfig struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ProjectDir  string `json:"project_dir"`
	Tier        string `json:"tier"`
}

type agentListFlag []string

func (v *agentListFlag) String() string { return strings.Join(*v, ",") }
func (v *agentListFlag) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			*v = append(*v, item)
		}
	}
	return nil
}

func loadAgentRegistry() (agentRegistry, error) {
	var registry agentRegistry
	if err := json.Unmarshal(agentRegistryJSON, &registry); err != nil {
		return registry, fmt.Errorf("embedded agent registry does not decode: %w", err)
	}
	if registry.Source.Repository == "" || len(registry.Source.Commit) != 40 || registry.Source.License == "" || registry.Source.Path == "" {
		return registry, fmt.Errorf("embedded agent registry source metadata is incomplete")
	}
	seen := map[string]bool{}
	for _, agent := range registry.Agents {
		if agent.ID == "" || seen[agent.ID] {
			return registry, fmt.Errorf("embedded agent registry has missing or duplicate id %q", agent.ID)
		}
		seen[agent.ID] = true
		if agent.DisplayName == "" || !safeProjectSkillDir(agent.ProjectDir) {
			return registry, fmt.Errorf("embedded agent registry entry %q is invalid", agent.ID)
		}
		if agent.Tier != "verified" && agent.Tier != "registry" {
			return registry, fmt.Errorf("embedded agent registry entry %q has invalid tier", agent.ID)
		}
	}
	return registry, nil
}

func safeProjectSkillDir(value string) bool {
	if value == "" || filepath.IsAbs(value) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func cmdAgents(args []string) error {
	fs := flag.NewFlagSet("agents", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("agents takes no positional arguments")
	}
	registry, err := loadAgentRegistry()
	if err != nil {
		return err
	}
	root, _ := os.Getwd()
	for _, agent := range registry.Agents {
		detection := "explicit"
		if agent.Tier == "verified" {
			if agentDetected(agent, root) {
				detection = "detected"
			} else {
				detection = "not detected"
			}
		}
		fmt.Printf("%-22s %-24s %-10s %s\n", agent.ID, agent.DisplayName, agent.Tier, detection+" · "+agent.ProjectDir)
	}
	return nil
}

func cmdRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	var agents agentListFlag
	fs.Var(&agents, "agent", "agent id, comma-separated ids, or auto")
	root := fs.String("root", "", "repository root (detected from .git by default)")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("register takes exactly one skill directory")
	}
	result, err := registerSkill(fs.Arg(0), *root, agents)
	if err != nil {
		return err
	}
	for _, item := range result {
		fmt.Printf("registered: %-22s %s\n", item.AgentID, item.Path)
		fmt.Printf("reload:     %-22s %s\n", item.AgentID, reloadHint(item.AgentID))
	}
	return nil
}

type adapterPlan struct {
	path, sourceRel, content, status string
	agentIDs                         []string
}

func cmdRegisterAll(args []string) error {
	fs := flag.NewFlagSet("register-all", flag.ContinueOnError)
	var agents agentListFlag
	fs.Var(&agents, "agent", "agent id, comma-separated ids, or auto")
	root := fs.String("root", "", "repository root (detected from .git by default)")
	dryRun := fs.Bool("dry-run", false, "print the synchronization plan without writing")
	prune := fs.Bool("prune", false, "remove obsolete generated adapters owned by this workflow directory")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("register-all takes exactly one skills directory")
	}
	parent, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return err
	}
	var skills []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(parent, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			skills = append(skills, dir)
		}
	}
	sort.Strings(skills)
	if len(skills) == 0 {
		return fmt.Errorf("no workflows found directly under %s", parent)
	}
	plansByPath := map[string]*adapterPlan{}
	names := map[string]string{}
	var repoRoot, parentRel string
	var selected []agentConfig
	usesLocalRuntime := false
	for _, skill := range skills {
		skillDir, resolvedRoot, sourceRel, metadata, manifest, _, selectedAgents, inputErr := registrationInputs(skill, *root, agents)
		if inputErr != nil {
			return inputErr
		}
		if previous := names[metadata.Name]; previous != "" {
			return fmt.Errorf("agent-facing name %q is used by both %s and %s; names must be unique before registration", metadata.Name, previous, skillDir)
		}
		names[metadata.Name] = skillDir
		if repoRoot == "" {
			repoRoot, selected = resolvedRoot, selectedAgents
			parentRel, _ = filepath.Rel(repoRoot, parent)
		}
		usesLocalRuntime = usesLocalRuntime || manifest.Language == "go" || manifest.Language == "rust"
		digest, digestErr := protocol.DigestSkillDir(skillDir)
		if digestErr != nil {
			return digestErr
		}
		launcher, launcherErr := launcherFor(manifest.Language, skillDir, repoRoot)
		if launcherErr != nil {
			return launcherErr
		}
		content := renderAdapter(metadata, sourceRel, digest, launcher)
		for _, agent := range selectedAgents {
			path := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir), metadata.Name, "SKILL.md")
			plan := plansByPath[path]
			if plan == nil {
				status, statusErr := generatedAdapterStatus(path, sourceRel, content)
				if statusErr != nil {
					return statusErr
				}
				plan = &adapterPlan{path: path, sourceRel: sourceRel, content: content, status: status}
				plansByPath[path] = plan
			}
			plan.agentIDs = append(plan.agentIDs, agent.ID)
		}
	}
	paths := make([]string, 0, len(plansByPath))
	for path := range plansByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var conflicts []string
	for _, path := range paths {
		plan := plansByPath[path]
		if plan.status == "conflict" {
			existing := "user-owned adapter"
			if b, err := os.ReadFile(path); err == nil {
				if source := generatedSource(string(b)); source != "" {
					existing = source
				}
			}
			conflicts = append(conflicts, fmt.Sprintf("%s and %s at %s", existing, plan.sourceRel, path))
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("refusing bulk registration; resolve every agent-facing name collision before writing:\n  - %s", strings.Join(conflicts, "\n  - "))
	}
	if usesLocalRuntime && !*dryRun {
		if err := ensureLocalStateIgnored(repoRoot); err != nil {
			return err
		}
	}
	for _, path := range paths {
		plan := plansByPath[path]
		sort.Strings(plan.agentIDs)
		status := plan.status
		if *dryRun {
			status = map[string]string{"added": "would add", "updated": "would update", "unchanged": "unchanged"}[status]
		}
		fmt.Printf("%-13s %-24s %s\n", status+":", strings.Join(plan.agentIDs, ","), filepath.ToSlash(path))
		if !*dryRun && plan.status != "unchanged" {
			if _, err := writeGeneratedAdapter(path, plan.sourceRel, plan.content); err != nil {
				return err
			}
		}
	}
	if *prune {
		prefix := filepath.ToSlash(filepath.Clean(parentRel)) + "/"
		for _, agent := range selected {
			base := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir))
			children, _ := os.ReadDir(base)
			for _, child := range children {
				path := filepath.Join(base, child.Name(), "SKILL.md")
				b, readErr := os.ReadFile(path)
				if readErr != nil {
					continue
				}
				source := generatedSource(string(b))
				if !strings.HasPrefix(source, prefix) || plansByPath[path] != nil {
					continue
				}
				status := "removed"
				if *dryRun {
					status = "would remove"
				}
				fmt.Printf("%-13s %-24s %s\n", status+":", agent.ID, filepath.ToSlash(path))
				if !*dryRun {
					if err := os.Remove(path); err != nil {
						return err
					}
					_ = os.Remove(filepath.Dir(path)) // Keep the directory if it contains user files.
				}
			}
		}
	}
	if *dryRun {
		fmt.Println("dry-run: no files written")
	}
	return nil
}

func generatedSource(text string) string {
	start := strings.Index(text, generatedAdapterPrefix)
	if start < 0 {
		return ""
	}
	value := text[start+len(generatedAdapterPrefix):]
	end := strings.Index(value, ";")
	if end < 0 {
		return ""
	}
	return value[:end]
}

func reloadHint(agentIDs string) string {
	if strings.Contains(agentIDs, ",") {
		return "start a new session in each selected agent"
	}
	switch agentIDs {
	case "cursor":
		return "reload the Cursor window or start a new chat"
	case "codex", "claude-code":
		return "start a new agent session"
	default:
		return "reload the agent or start a new session"
	}
}

type registration struct {
	AgentID string
	Path    string
}

func registerSkill(skillArg, rootArg string, requested []string) ([]registration, error) {
	skillDir, repoRoot, sourceRel, metadata, manifest, _, selected, err := registrationInputs(skillArg, rootArg, requested)
	if err != nil {
		return nil, err
	}
	digest, err := protocol.DigestSkillDir(skillDir)
	if err != nil {
		return nil, fmt.Errorf("digest canonical workflow: %w", err)
	}
	launcher, err := launcherFor(manifest.Language, skillDir, repoRoot)
	if err != nil {
		return nil, err
	}
	if manifest.Language == "go" || manifest.Language == "rust" {
		if err := ensureLocalStateIgnored(repoRoot); err != nil {
			return nil, err
		}
	}
	content := renderAdapter(metadata, sourceRel, digest, launcher)
	byDestination := map[string][]string{}
	for _, agent := range selected {
		destination := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir), metadata.Name, "SKILL.md")
		byDestination[destination] = append(byDestination[destination], agent.ID)
	}
	paths := make([]string, 0, len(byDestination))
	for path := range byDestination {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var registrations []registration
	for _, path := range paths {
		if err := ensureContainedWrite(repoRoot, path); err != nil {
			return nil, err
		}
		if _, err := writeGeneratedAdapter(path, sourceRel, content); err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(repoRoot, path)
		ids := byDestination[path]
		sort.Strings(ids)
		registrations = append(registrations, registration{AgentID: strings.Join(ids, ","), Path: filepath.ToSlash(rel)})
	}
	return registrations, nil
}

func registrationInputs(skillArg, rootArg string, requested []string) (string, string, string, skillMetadata, skillManifest, agentRegistry, []agentConfig, error) {
	skillDir, err := filepath.Abs(skillArg)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	repoRoot, err := findRepoRoot(skillDir, rootArg)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	repoRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("resolve repository root: %w", err)
	}
	skillDir, err = filepath.EvalSymlinks(skillDir)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("resolve skill directory: %w", err)
	}
	sourceRel, err := filepath.Rel(repoRoot, skillDir)
	if err != nil || sourceRel == ".." || strings.HasPrefix(sourceRel, ".."+string(filepath.Separator)) {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("skill directory must be inside repository root %s", repoRoot)
	}
	if strings.ContainsAny(sourceRel, ";\r\n<>") {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, fmt.Errorf("skill path contains characters that cannot be represented safely in an adapter marker")
	}
	metadata, err := readSkillMetadata(skillDir)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	manifest, err := readSkillManifest(skillDir)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	if err := verifyWorkflowSDKVersion(manifest, skillDir, repoRoot, runtimeVersion()); err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, agentRegistry{}, nil, err
	}
	registry, err := loadAgentRegistry()
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, registry, nil, err
	}
	selected, err := selectAgents(registry, requested, repoRoot)
	if err != nil {
		return "", "", "", skillMetadata{}, skillManifest{}, registry, nil, err
	}
	for _, agent := range selected {
		canonicalRoot := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir))
		if within(canonicalRoot, skillDir) {
			return "", "", "", skillMetadata{}, skillManifest{}, registry, nil, fmt.Errorf("canonical workflow must not live inside agent discovery directory %s", agent.ProjectDir)
		}
	}
	return skillDir, repoRoot, filepath.ToSlash(sourceRel), metadata, manifest, registry, selected, nil
}

func selectAgents(registry agentRegistry, requested []string, repoRoot string) ([]agentConfig, error) {
	if len(requested) == 0 {
		requested = []string{"auto"}
	}
	lookup := map[string]agentConfig{}
	for _, agent := range registry.Agents {
		lookup[agent.ID] = agent
	}
	selected := map[string]agentConfig{}
	for _, id := range requested {
		if id == "auto" {
			for _, agent := range registry.Agents {
				if agent.Tier == "verified" && agentDetected(agent, repoRoot) {
					selected[agent.ID] = agent
				}
			}
			continue
		}
		agent, ok := lookup[id]
		if !ok {
			return nil, fmt.Errorf("unsupported agent %q; run 'yskill agents' to list ids", id)
		}
		selected[id] = agent
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no verified coding agents detected; pass --agent <id> explicitly")
	}
	result := make([]agentConfig, 0, len(selected))
	for _, agent := range selected {
		result = append(result, agent)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func agentDetected(agent agentConfig, repoRoot string) bool {
	home, _ := os.UserHomeDir()
	exists := func(path string) bool { _, err := os.Stat(path); return err == nil }
	projectRoot := strings.SplitN(filepath.FromSlash(agent.ProjectDir), string(filepath.Separator), 2)[0]
	if repoRoot != "" && exists(filepath.Join(repoRoot, projectRoot)) {
		return true
	}
	switch agent.ID {
	case "cursor":
		return exists(filepath.Join(home, ".cursor")) || commandExists("cursor")
	case "codex":
		root := strings.TrimSpace(os.Getenv("CODEX_HOME"))
		if root == "" {
			root = filepath.Join(home, ".codex")
		}
		return exists(root) || exists("/etc/codex") || commandExists("codex")
	case "claude-code":
		root := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
		if root == "" {
			root = filepath.Join(home, ".claude")
		}
		return exists(root) || commandExists("claude")
	default:
		return false
	}
}

// ensureContainedWrite rejects an existing symlink in an adapter path when it
// would redirect the generated file outside the repository.
func ensureContainedWrite(repoRoot, destination string) error {
	if !within(repoRoot, destination) {
		return fmt.Errorf("adapter path escapes repository: %s", destination)
	}
	existing := destination
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return fmt.Errorf("cannot resolve adapter path %s", destination)
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	if !within(repoRoot, resolved) {
		return fmt.Errorf("adapter path resolves outside repository: %s", destination)
	}
	return nil
}

func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }

func findRepoRoot(skillDir, explicit string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return "", fmt.Errorf("repository root %s is not a directory", root)
		}
		return filepath.Clean(root), nil
	}
	for current := filepath.Clean(skillDir); ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return "", fmt.Errorf("cannot find repository root from %s; pass --root", skillDir)
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func launcherFor(language, skillDir, repoRoot string) (string, error) {
	switch language {
	case "typescript":
		packageRoot, err := findTypeScriptPackageRoot(skillDir, repoRoot)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(repoRoot, packageRoot)
		if rel == "." {
			return "npm exec -- yskill", nil
		}
		return "npm --prefix " + shellQuote(filepath.ToSlash(rel)) + " exec -- yskill", nil
	case "python":
		python := strings.TrimSpace(os.Getenv("YIELD_PYTHON"))
		if filepath.IsAbs(python) && within(repoRoot, python) {
			rel, err := filepath.Rel(repoRoot, python)
			if err == nil {
				return shellQuote(filepath.ToSlash(rel)) + " -m yieldskill", nil
			}
		}
		return "python -m yieldskill", nil
	case "go", "rust":
		path := localRuntimePath(repoRoot)
		if err := verifyLocalRuntime(path, runtimeVersion(), language); err != nil {
			return "", err
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return "", err
		}
		return repositoryRuntimeLauncher(rel, runtime.GOOS), nil
	default:
		return "", fmt.Errorf("unsupported workflow language %q", language)
	}
}

func repositoryRuntimeLauncher(relative, goos string) string {
	if goos == "windows" {
		return `.\` + strings.ReplaceAll(filepath.ToSlash(relative), "/", `\`)
	}
	return filepath.ToSlash(relative)
}

var inspectRuntimeVersion = func(path string) (string, error) {
	out, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s version: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 || fields[0] != "yskill" {
		return "", fmt.Errorf("%s returned an invalid version line: %q", path, strings.TrimSpace(string(out)))
	}
	return strings.TrimPrefix(fields[1], "v"), nil
}

func localRuntimePath(repoRoot string) string {
	name := "yskill"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(repoRoot, ".yield", "bin", name)
}

func ensureLocalStateIgnored(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".yield")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte("*\n"), 0o644)
}

func verifyLocalRuntime(path, expected, language string) error {
	repair := localRuntimeInstallCommand(language, expected)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s workflow needs Yield %s at %s; repair: %s", language, expected, filepath.ToSlash(path), repair)
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("repository-local Yield runtime is not a regular file: %s", filepath.ToSlash(path))
	}
	got, err := inspectRuntimeVersion(path)
	if err != nil {
		return fmt.Errorf("repository-local Yield runtime is unusable: %w", err)
	}
	if got != expected {
		return fmt.Errorf("repository-local Yield runtime version is %s, but this workflow needs %s; repair: %s", got, expected, repair)
	}
	return nil
}

func localRuntimeInstallCommand(language, expected string) string {
	switch language {
	case "go":
		if runtime.GOOS == "windows" {
			return fmt.Sprintf(`New-Item -ItemType Directory -Force .yield\bin | Out-Null; $env:GOBIN="$PWD\.yield\bin"; $env:GOPROXY="https://get.operatorstack.systems/go,direct"; go install github.com/operatorstack/yield/cmd/yskill@v%s`, expected)
		}
		return fmt.Sprintf(`mkdir -p .yield/bin && GOBIN="$PWD/.yield/bin" GOPROXY=https://get.operatorstack.systems/go,direct go install github.com/operatorstack/yield/cmd/yskill@v%s`, expected)
	case "rust":
		return fmt.Sprintf(`cargo install yieldskill@%s --root .yield --index sparse+https://get.operatorstack.systems/cargo/index/ --locked`, expected)
	default:
		return "install the matching Yield package"
	}
}

var pinnedVersionPatterns = map[string]*regexp.Regexp{
	"python": regexp.MustCompile(`(?m)^yieldskill==([^\s]+)$`),
	"go":     regexp.MustCompile(`(?m)^\s*github\.com/operatorstack/yield\s+v([^\s]+)`),
	"rust":   regexp.MustCompile(`(?m)yieldskill\s*=\s*\{[^\n]*version\s*=\s*"=([^"]+)"`),
}

func verifyWorkflowSDKVersion(manifest skillManifest, skillDir, repoRoot, expected string) error {
	if expected == "dev" {
		return nil
	}
	var declared string
	if manifest.Language == "typescript" {
		root, err := findTypeScriptPackageRoot(skillDir, repoRoot)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(filepath.Join(root, "package.json"))
		if err != nil {
			return err
		}
		var packageJSON struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if err := json.Unmarshal(b, &packageJSON); err != nil {
			return fmt.Errorf("package.json does not decode: %w", err)
		}
		declared = packageJSON.Dependencies["@operatorstack/yield"]
		if declared == "" {
			declared = packageJSON.DevDependencies["@operatorstack/yield"]
		}
	} else {
		file := map[string]string{"python": "requirements.txt", "go": "go.mod", "rust": "Cargo.toml"}[manifest.Language]
		b, err := os.ReadFile(filepath.Join(skillDir, file))
		if err != nil {
			return fmt.Errorf("read %s SDK version: %w", file, err)
		}
		match := pinnedVersionPatterns[manifest.Language].FindStringSubmatch(string(b))
		if len(match) == 2 {
			declared = match[1]
		}
	}
	declared = strings.TrimPrefix(strings.TrimSpace(declared), "v")
	if declared == "" {
		return fmt.Errorf("%s workflow must pin the Yield SDK to %s", manifest.Language, expected)
	}
	if declared != expected {
		return fmt.Errorf("%s workflow pins Yield SDK %s, but the runtime is %s", manifest.Language, declared, expected)
	}
	return nil
}

func findTypeScriptPackageRoot(skillDir, repoRoot string) (string, error) {
	var declared string
	for current := filepath.Clean(skillDir); within(repoRoot, current); current = filepath.Dir(current) {
		path := filepath.Join(current, "package.json")
		if hasTypeScriptPackageDependency(path) {
			if declared == "" {
				declared = current
			}
			if _, err := os.Stat(filepath.Join(current, "node_modules", "@operatorstack", "yield")); err == nil {
				return current, nil
			}
		}
		if current == repoRoot {
			break
		}
	}
	if declared != "" {
		return declared, nil
	}
	return "", fmt.Errorf("cannot find a package.json containing @operatorstack/yield between %s and %s", skillDir, repoRoot)
}

func hasTypeScriptPackageDependency(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(b, &manifest) != nil {
		return false
	}
	return manifest.Dependencies["@operatorstack/yield"] != "" || manifest.DevDependencies["@operatorstack/yield"] != ""
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func renderAdapter(metadata skillMetadata, sourceRel, digest, launcher string) string {
	path := shellQuote(sourceRel)
	return fmt.Sprintf(`---
name: %s
description: %s
---

%s%s; digest: %s; version: %s -->

This adapter exposes the canonical Yield workflow at %s.
Read its SKILL.md, then run from the repository root:

    %s run %s

	Follow each returned operation exactly. Answer each operation directly:

	    %s respond <run-id> --value <answer> --skill %s

	For structured agent results, use --result-json instead of --value.

Do not skip an operation or invent its response.
`, metadata.Name, yamlString(metadata.Description), generatedAdapterPrefix, sourceRel, digest, runtimeVersion(), "`"+sourceRel+"`", launcher, path, launcher, path)
}

func generatedAdapterStatus(path, sourceRel, content string) (string, error) {
	if existing, err := os.ReadFile(path); err == nil {
		marker := generatedAdapterPrefix + sourceRel + ";"
		if !strings.Contains(string(existing), marker) {
			return "conflict", nil
		}
		if string(existing) == content {
			return "unchanged", nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	} else {
		return "added", nil
	}
	return "updated", nil
}

func writeGeneratedAdapter(path, sourceRel, content string) (string, error) {
	status, err := generatedAdapterStatus(path, sourceRel, content)
	if err != nil {
		return "", err
	}
	if status == "conflict" {
		return status, fmt.Errorf("refusing to overwrite user-owned or differently sourced adapter %s", path)
	}
	if status == "unchanged" {
		return status, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".yskill-adapter-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return status, os.Rename(tmpName, path)
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	var agents agentListFlag
	fs.Var(&agents, "agent", "agent id, comma-separated ids, or auto")
	root := fs.String("root", "", "repository root (detected from .git by default)")
	runTest := fs.Bool("test", false, "run the workflow fixture after static checks")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("doctor takes exactly one skill directory")
	}
	skillDir, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	skillDir, err = filepath.EvalSymlinks(skillDir)
	if err != nil {
		return err
	}
	metadata, err := readSkillMetadata(skillDir)
	if err != nil {
		return err
	}
	manifest, err := readSkillManifest(skillDir)
	if err != nil {
		return err
	}
	packageBoundary, boundaryErr := findRepoRoot(skillDir, *root)
	if boundaryErr != nil {
		if manifest.Language == "go" || manifest.Language == "rust" {
			return fmt.Errorf("%s workflow needs a repository root for .yield/bin; pass --root: %w", manifest.Language, boundaryErr)
		}
		if len(agents) > 0 || *root != "" {
			return boundaryErr
		}
		volume := filepath.VolumeName(skillDir)
		packageBoundary = volume + string(filepath.Separator)
	} else if packageBoundary, err = filepath.EvalSymlinks(packageBoundary); err != nil {
		return err
	}
	var problems []string
	if err := verifyWorkflowSDKVersion(manifest, skillDir, packageBoundary, runtimeVersion()); err != nil {
		problems = append(problems, "SDK: "+err.Error())
	}
	if _, err := launcherFor(manifest.Language, skillDir, packageBoundary); err != nil {
		problems = append(problems, "runtime: "+err.Error())
	}
	if err := languageDiagnostics(manifest.Language, skillDir); err != nil {
		problems = append(problems, "language: "+err.Error())
	}
	if *runTest && len(problems) == 0 {
		if err := cmdTest([]string{skillDir}); err != nil {
			return err
		}
	}
	if len(problems) == 0 {
		fmt.Printf("ok: workflow               %s\n", filepath.ToSlash(skillDir))
	}
	if len(agents) == 0 {
		if len(problems) > 0 {
			return fmt.Errorf("doctor found problems:\n  - %s", strings.Join(problems, "\n  - "))
		}
		fmt.Printf("doctor: %s workflow is ready\n", filepath.Base(skillDir))
		return nil
	}
	repoRoot := packageBoundary
	sourceRel, err := filepath.Rel(repoRoot, skillDir)
	if err != nil || sourceRel == ".." || strings.HasPrefix(sourceRel, ".."+string(filepath.Separator)) {
		problems = append(problems, "workflow is outside the repository root")
		sourceRel = ""
	}
	sourceRel = filepath.ToSlash(sourceRel)
	registry, err := loadAgentRegistry()
	if err != nil {
		return err
	}
	selected, err := selectAgents(registry, agents, repoRoot)
	if err != nil {
		return err
	}
	digest, err := protocol.DigestSkillDir(skillDir)
	if err != nil {
		return err
	}
	for _, agent := range selected {
		path := filepath.Join(repoRoot, filepath.FromSlash(agent.ProjectDir), metadata.Name, "SKILL.md")
		if err := ensureContainedWrite(repoRoot, path); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", agent.ID, err))
			continue
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			problems = append(problems, fmt.Sprintf("%s: adapter missing at %s", agent.ID, path))
			continue
		}
		text := string(b)
		if !strings.Contains(text, generatedAdapterPrefix+sourceRel+";") || !strings.Contains(text, "digest: "+digest+";") || !strings.Contains(text, "version: "+runtimeVersion()+" -->") {
			problems = append(problems, fmt.Sprintf("%s: adapter is stale or points elsewhere", agent.ID))
			continue
		}
		fmt.Printf("ok: %-22s %s\n", agent.ID, filepath.ToSlash(path))
	}
	if len(problems) > 0 {
		return fmt.Errorf("doctor found problems:\n  - %s\nrun yskill register to update generated adapters after fixing version errors", strings.Join(problems, "\n  - "))
	}
	fmt.Printf("doctor: %s is ready for %d agent(s)\n", filepath.Base(skillDir), len(selected))
	return nil
}

func languageDiagnostics(language, skillDir string) error {
	switch language {
	case "go":
		if !commandExists("go") {
			return fmt.Errorf("Go workflow needs the go command")
		}
		fmt.Printf("validate: (cd %s && go test ./... && go vet ./...)\n", shellQuote(filepath.ToSlash(skillDir)))
	case "rust":
		if !commandExists("cargo") {
			return fmt.Errorf("Rust workflow needs the cargo command")
		}
		if _, err := os.Stat(filepath.Join(skillDir, ".cargo", "config.toml")); err != nil {
			return fmt.Errorf("Rust workflow needs .cargo/config.toml in the workflow directory: %w", err)
		}
		var missing []string
		if !commandExists("rustfmt") {
			missing = append(missing, "rustfmt")
		}
		if !commandExists("cargo-clippy") {
			missing = append(missing, "clippy")
		}
		if len(missing) > 0 {
			fmt.Printf("optional: rustup component add %s\n", strings.Join(missing, " "))
		}
		fmt.Printf("validate: (cd %s && cargo fmt --check && cargo check && cargo clippy -- -D warnings)\n", shellQuote(filepath.ToSlash(skillDir)))
	case "python":
		fmt.Printf("validate: (cd %s && python -m compileall -q .)\n", shellQuote(filepath.ToSlash(skillDir)))
	}
	return nil
}

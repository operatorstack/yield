// yskill is the command-line interface for Yield: it starts runs, validates and
// accepts responses, executes commands as observed fact, and owns the
// append-only run log. The coding agent drives it through a small set of verbs.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/operatorstack/yield/internal/engine"
	"github.com/operatorstack/yield/internal/protocol"
	"github.com/operatorstack/yield/internal/runlog"
)

const usage = `yskill — turn SKILL.md workflows into resumable programs

Usage:
  yskill init <dir>                          scaffold a skill (or wrap an existing prose skill)
         [--language typescript|python|go|rust] [--description text]
  yskill register <skill-dir>                expose one workflow to coding agents
         [--agent cursor,codex,...|auto] [--root repo]
  yskill register-all <skills-dir>           expose every immediate workflow
         [--agent cursor,codex,...|auto] [--root repo] [--dry-run] [--prune]
  yskill agents                              list supported coding agents and paths
  yskill doctor <skill-dir>                  check package, workflow, and adapters
         [--agent cursor,codex,...|auto] [--root repo] [--test]
  yskill run <skill-dir> [--input file]      start a run; prints the first operation envelope
  yskill resume <run-id> --response file     feed a response; prints the next operation
         [--skill dir] [--accept-new-digest]
  yskill respond <run-id> --value text       answer the pending question directly
         [--result-json json|-] [--skill dir]
  yskill inspect <run-id> [--skill dir]      print the run's event log
  yskill prune <skill-dir> --older-than 720h remove old terminal runs
         [--keep-last n] [--dry-run]
  yskill replay <run-id> [--skill dir]       re-derive the run from its log; verify determinism
  yskill test <skill-dir> [--keep-run]       run the skill against fixtures/responses.json
  yskill version                             print the runtime version and target
`

// version is set from the release tag with -ldflags "-X main.version=<version>".
var version = "dev"
var readBuildInfo = debug.ReadBuildInfo

func runtimeVersion() string {
	if version != "" && version != "dev" {
		return strings.TrimPrefix(version, "v")
	}
	if info, ok := readBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "register":
		err = cmdRegister(os.Args[2:])
	case "register-all":
		err = cmdRegisterAll(os.Args[2:])
	case "agents":
		err = cmdAgents(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "resume":
		err = cmdResume(os.Args[2:])
	case "respond":
		err = cmdRespond(os.Args[2:])
	case "inspect":
		err = cmdInspect(os.Args[2:])
	case "prune":
		err = cmdPrune(os.Args[2:])
	case "replay":
		err = cmdReplay(os.Args[2:])
	case "test":
		err = cmdTest(os.Args[2:])
	case "version", "--version":
		fmt.Printf("yskill %s %s/%s\n", runtimeVersion(), runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "yskill: unknown verb %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "yskill: %v\n", err)
		os.Exit(1)
	}
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	input := fs.String("input", "", "path to a JSON input file")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("run takes exactly one skill directory")
	}
	e, err := engine.New(fs.Arg(0))
	if err != nil {
		return err
	}
	var in json.RawMessage
	if *input != "" {
		b, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		in = b
	}
	p, err := e.StartRun(in)
	if err != nil {
		return err
	}
	return printProgress(p)
}

func cmdResume(args []string) error {
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	response := fs.String("response", "", "path to the response envelope JSON")
	skillDir := fs.String("skill", ".", "skill directory the run belongs to")
	migrate := fs.Bool("accept-new-digest", false, "explicitly rebind the run to the current skill source digest")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 || *response == "" {
		return fmt.Errorf("resume takes a run id and --response file")
	}
	e, err := engine.New(*skillDir)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(*response)
	if err != nil {
		return err
	}
	p, err := e.Resume(fs.Arg(0), b, *migrate)
	if err != nil {
		return err
	}
	return printProgress(p)
}

func cmdRespond(args []string) error {
	fs := flag.NewFlagSet("respond", flag.ExitOnError)
	value := fs.String("value", "", "answer value for an ask_user operation")
	resultJSON := fs.String("result-json", "", "JSON result, or - to read JSON from stdin")
	skillDir := fs.String("skill", ".", "skill directory the run belongs to")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	seen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	if fs.NArg() != 1 || seen["value"] == seen["result-json"] {
		return fmt.Errorf("respond takes one run id and exactly one of --value or --result-json")
	}
	e, err := engine.New(*skillDir)
	if err != nil {
		return err
	}
	var result json.RawMessage
	if seen["value"] {
		result, err = json.Marshal(map[string]string{"value": *value})
	} else if *resultJSON == "-" {
		result, err = io.ReadAll(os.Stdin)
	} else {
		result = json.RawMessage(*resultJSON)
	}
	if err != nil {
		return err
	}
	if !json.Valid(result) {
		return fmt.Errorf("result is not valid JSON")
	}
	pending, pendingErr := e.Pending(fs.Arg(0))
	if pendingErr == nil && seen["value"] && pending.Request.Kind != protocol.OpAskUser {
		return fmt.Errorf("--value is only valid for ask_user; use --result-json for %s", pending.Request.Kind)
	}
	if pendingErr != nil && !strings.Contains(pendingErr.Error(), "has no pending operation") && !strings.Contains(pendingErr.Error(), "terminal state") {
		return pendingErr
	}
	p, err := e.Respond(fs.Arg(0), result)
	if err != nil {
		return err
	}
	return printProgress(p)
}

func cmdInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	skillDir := fs.String("skill", ".", "skill directory the run belongs to")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	e, err := engine.New(*skillDir)
	if err != nil {
		return err
	}
	if fs.NArg() == 0 {
		ids, err := e.ListRuns()
		if err != nil {
			return err
		}
		for _, id := range ids {
			fmt.Println(id)
		}
		return nil
	}
	l, err := e.Log(fs.Arg(0))
	if err != nil {
		return err
	}
	for _, ev := range l.Events() {
		fmt.Printf("%03d  %-22s %s  %s\n", ev.Seq, ev.Type, ev.At.Format("2006-01-02T15:04:05Z"), compact(ev.Data))
	}
	return nil
}

func cmdPrune(args []string) error {
	fs := flag.NewFlagSet("prune", flag.ExitOnError)
	olderThan := fs.Duration("older-than", 0, "minimum terminal-run age, for example 24h or 720h")
	keepLast := fs.Int("keep-last", 0, "always keep this many newest terminal runs")
	dryRun := fs.Bool("dry-run", false, "print runs without deleting them")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 || *olderThan <= 0 || *keepLast < 0 {
		return fmt.Errorf("prune takes one skill directory, --older-than greater than zero, and a nonnegative --keep-last")
	}
	e, err := engine.New(fs.Arg(0))
	if err != nil {
		return err
	}
	ids, err := e.ListRuns()
	if err != nil {
		return err
	}
	type candidate struct {
		id  string
		mod time.Time
	}
	var terminal []candidate
	for _, id := range ids {
		log, openErr := e.Log(id)
		if openErr != nil {
			return openErr
		}
		closed := false
		for _, event := range log.Events() {
			if event.Type == runlog.RunCompleted || event.Type == runlog.RunBlocked || event.Type == runlog.RunRefused {
				closed = true
			}
		}
		if !closed {
			continue
		}
		info, statErr := os.Stat(log.Path)
		if statErr != nil {
			return statErr
		}
		terminal = append(terminal, candidate{id: id, mod: info.ModTime()})
	}
	sort.Slice(terminal, func(i, j int) bool { return terminal[i].mod.After(terminal[j].mod) })
	cutoff := time.Now().Add(-*olderThan)
	removed := 0
	for index, run := range terminal {
		if index < *keepLast || !run.mod.Before(cutoff) {
			continue
		}
		fmt.Printf("prune: %s\n", run.id)
		if !*dryRun {
			if err := os.Remove(filepath.Join(e.RunsDir, run.id+".jsonl")); err != nil {
				return err
			}
			_ = os.Remove(filepath.Join(e.RunsDir, run.id+".lock"))
		}
		removed++
	}
	fmt.Printf("prune: %d terminal run(s) selected\n", removed)
	return nil
}

func cmdReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	skillDir := fs.String("skill", ".", "skill directory the run belongs to")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("replay takes exactly one run id")
	}
	e, err := engine.New(*skillDir)
	if err != nil {
		return err
	}
	p, err := e.Replay(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Println("replay: deterministic — the log reproduces the run's frontier")
	return printProgress(p)
}

// cmdTest drives a skill against fixtures/responses.json: an ordered map
// of request id -> scripted result. run_command operations execute for
// real; everything else is answered from the script.
func cmdTest(args []string) (retErr error) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	keepRun := fs.Bool("keep-run", false, "keep the fixture run under the workflow's .yield directory")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("test takes exactly one skill directory")
	}
	dir := fs.Arg(0)
	b, err := os.ReadFile(filepath.Join(dir, "fixtures", "responses.json"))
	if err != nil {
		return fmt.Errorf("fixtures/responses.json is required: %w", err)
	}
	var script map[string]json.RawMessage
	if err := json.Unmarshal(b, &script); err != nil {
		return fmt.Errorf("fixtures/responses.json does not decode: %w", err)
	}
	fixture, err := readFixtureConfig(dir)
	if err != nil {
		return err
	}
	defer func() {
		if err := runFixtureCommands(dir, fixture.Teardown, nil); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("fixture teardown: %w", err))
		}
	}()
	if err := runFixtureCommands(dir, fixture.Setup, nil); err != nil {
		return fmt.Errorf("fixture setup: %w", err)
	}
	var e *engine.Engine
	if *keepRun {
		e, err = engine.New(dir)
	} else {
		var runs string
		runs, err = os.MkdirTemp("", "yield-test-runs-*")
		if err == nil {
			defer os.RemoveAll(runs)
			e, err = engine.NewWithRunsDir(dir, runs)
		}
	}
	if err != nil {
		return err
	}
	p, err := e.StartRun(nil)
	if err != nil {
		return err
	}
	for p.Terminal == nil {
		requestID := p.Envelope.Request.ID
		result, ok := script[requestID]
		if !ok {
			return fmt.Errorf("no scripted response for request %q (sequence %d)", p.Envelope.Request.ID, p.Envelope.Sequence)
		}
		resp, err := json.Marshal(protocol.ResponseEnvelope{
			RunID: p.RunID, Sequence: p.Envelope.Sequence,
			RequestID: p.Envelope.Request.ID, Status: "completed", Result: result,
		})
		if err != nil {
			return err
		}
		if err := protocol.ValidateResult(p.Envelope.Request.OutputSchema, result); err != nil {
			return fmt.Errorf("fixture response for %q: %w", requestID, err)
		}
		if err := runFixtureCommands(dir, fixture.AfterResponse[requestID], result); err != nil {
			return fmt.Errorf("fixture effect for %q: %w", requestID, err)
		}
		if p, err = e.Resume(p.RunID, resp, false); err != nil {
			return err
		}
	}
	fmt.Printf("test: run %s reached %s\n", p.RunID, p.Terminal.Status)
	if p.Terminal.Status != protocol.StatusCompleted {
		return fmt.Errorf("terminal status %s: %s", p.Terminal.Status, p.Terminal.Reason)
	}
	return nil
}

type fixtureConfig struct {
	Version       int                   `json:"version"`
	Setup         [][]string            `json:"setup"`
	AfterResponse map[string][][]string `json:"after_response"`
	Teardown      [][]string            `json:"teardown"`
}

func readFixtureConfig(dir string) (fixtureConfig, error) {
	path := filepath.Join(dir, "fixtures", "test.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fixtureConfig{Version: 1}, nil
	}
	if err != nil {
		return fixtureConfig{}, err
	}
	var config fixtureConfig
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return fixtureConfig{}, fmt.Errorf("fixtures/test.json does not decode: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fixtureConfig{}, fmt.Errorf("fixtures/test.json must contain one JSON object")
	}
	if config.Version != 1 {
		return fixtureConfig{}, fmt.Errorf("fixtures/test.json version must be 1")
	}
	return config, nil
}

func runFixtureCommands(dir string, commands [][]string, input json.RawMessage) error {
	for index, argv := range commands {
		if len(argv) == 0 {
			return fmt.Errorf("command %d is empty", index+1)
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "YIELD_FIXTURE=1")
		if len(input) > 0 {
			cmd.Stdin = bytes.NewReader(input)
		}
		var output bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%q failed: %w: %s", argv, err, strings.TrimSpace(output.String()))
		}
	}
	return nil
}

// parseOnePositional accepts the documented command shape where the target
// comes first and flags follow it. The standard flag package stops parsing at
// the first positional argument, so move that one target behind the flags.
// Flag-first calls keep working unchanged.
func parseOnePositional(fs *flag.FlagSet, args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	return fs.Parse(args)
}

func printProgress(p *engine.Progress) error {
	if p.Terminal != nil {
		fmt.Printf("run %s: %s\n", p.RunID, p.Terminal.Status)
		if p.Terminal.Reason != "" {
			fmt.Printf("reason: %s\n", p.Terminal.Reason)
		}
		if len(p.Terminal.Result) > 0 {
			fmt.Printf("result: %s\n", p.Terminal.Result)
		}
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(p.Envelope)
}

func compact(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	s := string(raw)
	if len(s) > 140 {
		s = s[:140] + "…"
	}
	return s
}

// cmdInit scaffolds a minimal skill: a thin SKILL.md, a skill program, and
// a fixtures directory. If the directory already has a SKILL.md, it is
// preserved — init wraps existing prose skills rather than replacing them.
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	sdkPath := fs.String("sdk", "", "filesystem path to the yield module (written as a go.mod replace directive)")
	language := fs.String("language", defaultLanguage(), "workflow language: typescript, python, go, or rust")
	description := fs.String("description", "", "what the skill does and when an agent should use it")
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("init takes exactly one directory")
	}
	return scaffoldSkill(fs.Arg(0), *language, *sdkPath, *description)
}

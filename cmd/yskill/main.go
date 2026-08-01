// yskill is the supervisor CLI for Yield: it starts runs, validates and
// accepts responses, executes commands as observed fact, and owns the
// append-only run log. The coding agent drives it through six verbs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/operatorstack/yield/internal/engine"
	"github.com/operatorstack/yield/internal/protocol"
)

const usage = `yskill — turn SKILL.md workflows into resumable programs

Usage:
  yskill init <dir>                          scaffold a skill (or wrap an existing prose skill)
         [--language typescript|python|go|rust]
  yskill run <skill-dir> [--input file]      start a run; prints the first operation envelope
  yskill resume <run-id> --response file     feed a response; prints the next operation
         [--skill dir] [--accept-new-digest]
  yskill inspect <run-id> [--skill dir]      print the run's event log
  yskill replay <run-id> [--skill dir]       re-derive the run from its log; verify determinism
  yskill test <skill-dir>                    run the skill against fixtures/responses.json
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
	case "run":
		err = cmdRun(os.Args[2:])
	case "resume":
		err = cmdResume(os.Args[2:])
	case "inspect":
		err = cmdInspect(os.Args[2:])
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
func cmdTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
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
	e, err := engine.New(dir)
	if err != nil {
		return err
	}
	p, err := e.StartRun(nil)
	if err != nil {
		return err
	}
	for p.Terminal == nil {
		result, ok := script[p.Envelope.Request.ID]
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
			fmt.Printf("result: %s\n", compact(p.Terminal.Result))
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
	if err := parseOnePositional(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("init takes exactly one directory")
	}
	return scaffoldSkill(fs.Arg(0), *language, *sdkPath)
}

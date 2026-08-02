package yield_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublicAskUserOptionCompilesFromExternalModule(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate SDK source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	dir := t.TempDir()
	goMod := "module example.com/yield-consumer\n\ngo 1.26.5\n\nrequire github.com/operatorstack/yield v0.0.0\nreplace github.com/operatorstack/yield => " + filepath.ToSlash(root) + "\n"
	main := `package main

import yield "github.com/operatorstack/yield/sdk/yield"

func choose(ctx *yield.Context) string {
	return ctx.AskUser("approve", "Continue?", yield.Option{Value: "yes", Label: "Yes"})
}
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOWORK=off")
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("external Go module cannot resolve Yield dependencies: %v\n%s", err, output)
	}

	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("external Go module cannot use yield.Option: %v\n%s", err, out)
	}
}

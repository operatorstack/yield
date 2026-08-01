package main

import (
	"flag"
	"testing"
)

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

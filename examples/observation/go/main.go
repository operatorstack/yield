package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/operatorstack/yield/observation"
)

type receiptSummary struct {
	Schema              string                         `json:"schema"`
	ReceiptDigest       string                         `json:"receipt_digest"`
	RunID               string                         `json:"run_id"`
	Skill               string                         `json:"skill"`
	Phase               string                         `json:"phase"`
	TerminalDisposition *string                        `json:"terminal_disposition"`
	OperationSummaries  []observation.OperationSummary `json:"operation_summaries"`
}

func main() {
	if len(os.Args) != 2 {
		fail(fmt.Errorf("usage: go run ./examples/observation/go <canonical-receipt.json>"))
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail(err)
	}
	receipt, err := observation.Parse(raw)
	if err != nil {
		fail(err)
	}

	var disposition *string
	if receipt.Outcome.TerminalDisposition != "" {
		disposition = &receipt.Outcome.TerminalDisposition
	}
	summary := receiptSummary{
		Schema:              receipt.Schema,
		ReceiptDigest:       receipt.ReceiptDigest,
		RunID:               receipt.Run.ID,
		Skill:               receipt.Skill.Name,
		Phase:               receipt.Outcome.Phase,
		TerminalDisposition: disposition,
		OperationSummaries:  receipt.OperationSummaries,
	}
	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "receipt example: %v\n", err)
	os.Exit(1)
}

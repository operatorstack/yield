package observation

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildReportIsDeterministicAndMakesNoCausalClaim(t *testing.T) {
	r := RunReceipt{
		Run: RunIdentity{ID: "run_1"}, Timing: TimingSummary{StartedAt: "2026-08-20T10:00:00Z"},
		Outcome:            OutcomeSummary{Phase: "awaiting_response"},
		OperationSummaries: []OperationSummary{{Kind: OperationAskUser, Requested: 1}},
		Experiment:         &ExperimentContext{ExperimentID: "exp-1", VariantID: "candidate", Role: "candidate"},
	}
	options := ReportOptions{
		From: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		OpenAgeThreshold: time.Hour, ReferenceTime: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
	}
	first, err := BuildReport([]RunReceipt{r}, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildReport([]RunReceipt{r}, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same receipts and window produced different reports")
	}
	if first.OpenOlderThanThreshold != 1 || first.ExperimentGroups[0].Count != 1 {
		t.Fatalf("unexpected report: %+v", first)
	}
}

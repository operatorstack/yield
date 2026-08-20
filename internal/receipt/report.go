package receipt

import (
	"fmt"
	"sort"
	"time"

	"github.com/operatorstack/yield/internal/protocol"
)

type ReportOptions struct {
	From             time.Time
	To               time.Time
	OpenAgeThreshold time.Duration
	ReferenceTime    time.Time
	ExperimentID     string
}

type LocalReport struct {
	ReceiptCount           int                      `json:"receipt_count"`
	Lifecycle              []NamedCount             `json:"lifecycle"`
	Terminal               []NamedCount             `json:"terminal"`
	Operations             []ReportOperationSummary `json:"operations"`
	ResponseRejections     []NamedCount             `json:"response_rejections"`
	Requirements           []NamedCount             `json:"requirements"`
	DivergenceCount        int                      `json:"divergence_count"`
	RuntimeGroups          []NamedCount             `json:"runtime_groups"`
	SourceGroups           []NamedCount             `json:"source_groups"`
	ExperimentGroups       []NamedCount             `json:"experiment_groups"`
	OpenOlderThanThreshold int                      `json:"open_older_than_threshold,omitempty"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type ReportOperationSummary struct {
	Kind           protocol.OpKind `json:"kind"`
	Requested      int             `json:"requested"`
	Completed      int             `json:"completed"`
	TotalElapsedMS int64           `json:"total_elapsed_ms"`
}

func BuildReport(receipts []*RunReceipt, options ReportOptions) (LocalReport, error) {
	report := LocalReport{
		Lifecycle: []NamedCount{}, Terminal: []NamedCount{}, Operations: []ReportOperationSummary{},
		ResponseRejections: []NamedCount{}, Requirements: []NamedCount{}, RuntimeGroups: []NamedCount{},
		SourceGroups: []NamedCount{}, ExperimentGroups: []NamedCount{},
	}
	lifecycle := map[string]int{}
	terminal := map[string]int{}
	rejections := map[string]int{}
	requirements := map[string]int{}
	runtimes := map[string]int{}
	sources := map[string]int{}
	experiments := map[string]int{}
	operations := map[protocol.OpKind]*ReportOperationSummary{}
	for _, r := range receipts {
		started, err := time.Parse(time.RFC3339Nano, r.Timing.StartedAt)
		if err != nil {
			return LocalReport{}, fmt.Errorf("report: receipt %s has invalid start time: %w", r.Run.ID, err)
		}
		if !options.From.IsZero() && started.Before(options.From) || !options.To.IsZero() && started.After(options.To) {
			continue
		}
		if options.ExperimentID != "" && (r.Experiment == nil || r.Experiment.ExperimentID != options.ExperimentID) {
			continue
		}
		report.ReceiptCount++
		lifecycle[r.Outcome.Phase]++
		if r.Outcome.TerminalDisposition != "" {
			terminal[r.Outcome.TerminalDisposition]++
		}
		for _, summary := range r.OperationSummaries {
			aggregate := operations[summary.Kind]
			if aggregate == nil {
				aggregate = &ReportOperationSummary{Kind: summary.Kind}
				operations[summary.Kind] = aggregate
			}
			aggregate.Requested += summary.Requested
			aggregate.Completed += summary.Completed
			aggregate.TotalElapsedMS += summary.TotalElapsedMS
		}
		for _, rejection := range r.ResponseRejections {
			rejections[rejection.Reason] += rejection.Count
		}
		for _, requirement := range r.Requirements {
			requirements[requirement.Outcome]++
		}
		report.DivergenceCount += len(r.Divergences)
		if r.Runtime != nil {
			runtimes[r.Runtime.SupervisorVersion+"|"+r.Runtime.RequiredVersion]++
		}
		if r.Skill.SourceDigest != nil {
			sources[r.Skill.SourceDigest.Profile+"|"+r.Skill.SourceDigest.Value]++
		}
		if r.Experiment != nil {
			experiments[r.Experiment.ExperimentID+"|"+r.Experiment.VariantID+"|"+r.Experiment.Role]++
		}
		if options.OpenAgeThreshold > 0 && r.Outcome.Phase != "terminal" && r.Outcome.Phase != "initialization_failed" {
			reference := options.ReferenceTime
			if reference.IsZero() {
				reference = options.To
			}
			if !reference.IsZero() && reference.Sub(started) > options.OpenAgeThreshold {
				report.OpenOlderThanThreshold++
			}
		}
	}
	report.Lifecycle = namedCounts(lifecycle)
	report.Terminal = namedCounts(terminal)
	report.ResponseRejections = namedCounts(rejections)
	report.Requirements = namedCounts(requirements)
	report.RuntimeGroups = namedCounts(runtimes)
	report.SourceGroups = namedCounts(sources)
	report.ExperimentGroups = namedCounts(experiments)
	for _, kind := range []protocol.OpKind{protocol.OpAskUser, protocol.OpAgentTask, protocol.OpRunCommand} {
		if summary := operations[kind]; summary != nil {
			report.Operations = append(report.Operations, *summary)
		}
	}
	return report, nil
}

func namedCounts(values map[string]int) []NamedCount {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]NamedCount, 0, len(names))
	for _, name := range names {
		result = append(result, NamedCount{Name: name, Count: values[name]})
	}
	return result
}

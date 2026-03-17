package analysis

import (
	"context"
	"sort"

	"pgwatchai/internal/model"
)

type Analyzer interface {
	Name() model.IntentKind
	Analyze(ctx context.Context, exec model.ExecutionContext) ([]model.Finding, error)
}

type Engine struct {
	analyzers map[model.IntentKind]Analyzer
}

func NewEngine(items ...Analyzer) *Engine {
	m := make(map[model.IntentKind]Analyzer, len(items))
	for _, a := range items {
		m[a.Name()] = a
	}
	return &Engine{analyzers: m}
}

func NewDefaultEngine() *Engine {
	return NewEngine(
		NewSlowQueriesAnalyzer(),
		NewConnectionsAnalyzer(),
		NewLocksAnalyzer(),
		NewReplicationAnalyzer(),
	)
}

func (e *Engine) Run(ctx context.Context, exec model.ExecutionContext, intents []model.Intent) ([]model.Finding, error) {
	findings := make([]model.Finding, 0, 32)

	seen := make(map[model.IntentKind]bool)
	for _, in := range intents {
		if seen[in.Kind] {
			continue
		}
		seen[in.Kind] = true

		a, ok := e.analyzers[in.Kind]
		if !ok {
			continue
		}

		ff, err := a.Analyze(ctx, exec)
		if err != nil {
			return nil, err
		}
		findings = append(findings, ff...)
	}

	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
	})

	return findings, nil
}

func severityRank(s model.Severity) int {
	switch s {
	case model.SeverityCritical:
		return 3
	case model.SeverityWarning:
		return 2
	default:
		return 1
	}
}

package analysis

import (
	"context"
	"fmt"
	"strings"

	"pgwatchai/internal/db"
	"pgwatchai/internal/model"
	"pgwatchai/internal/reader"
)

type SlowQueriesAnalyzer struct {
	TopN            int
	WarnMeanExecMS  float64
	WarnTotalExecMS float64
}

func NewSlowQueriesAnalyzer() *SlowQueriesAnalyzer {
	return &SlowQueriesAnalyzer{
		TopN:            5,
		WarnMeanExecMS:  200.0,
		WarnTotalExecMS: 2000.0,
	}
}

func (a *SlowQueriesAnalyzer) Name() model.IntentKind {
	return model.IntentSlowQueries
}

func (a *SlowQueriesAnalyzer) Analyze(ctx context.Context, exec model.ExecutionContext) ([]model.Finding, error) {
	if strings.TrimSpace(exec.SinkDSN) == "" {
		return nil, fmt.Errorf("sink dsn is empty in execution context")
	}

	pool, err := db.NewPool(ctx, db.DefaultConfig(exec.SinkDSN))
	if err != nil {
		return nil, fmt.Errorf("open sink pool: %w", err)
	}
	defer db.ClosePool(pool)

	sinkReader := reader.NewSinkReader(pool)
	stats, err := sinkReader.GetSlowQueries(ctx, exec.Since, exec.DBFilter, a.TopN)
	if err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return []model.Finding{
			{
				ID:       "slowq-none",
				Title:    "No slow-query rows found in time window",
				Severity: model.SeverityInfo,
				Category: model.IntentSlowQueries,
				Summary:  "No slow-query records were returned from pgwatch sink for the selected time range.",
			},
		}, nil
	}

	findings := make([]model.Finding, 0, len(stats))
	for i, s := range stats {
		sev := model.SeverityInfo
		if s.MeanExecMS >= a.WarnMeanExecMS || s.TotalExecMS >= a.WarnTotalExecMS {
			sev = model.SeverityWarning
		}

		findings = append(findings, model.Finding{
			ID:       fmt.Sprintf("slowq-%d", i+1),
			Title:    "Slow query candidate",
			Severity: sev,
			Category: model.IntentSlowQueries,
			Summary: fmt.Sprintf(
				"DB=%s query_id=%s mean=%.2fms total=%.2fms calls=%.0f",
				s.DBName, s.QueryID, s.MeanExecMS, s.TotalExecMS, s.Calls,
			),
			Evidence: map[string]any{
				"db_name":       s.DBName,
				"query_id":      s.QueryID,
				"query_text":    s.QueryText,
				"mean_exec_ms":  s.MeanExecMS,
				"total_exec_ms": s.TotalExecMS,
				"calls":         s.Calls,
			},
			Suggestion: "Inspect execution plan and index usage; compare with historical baseline for this query fingerprint.",
		})
	}

	return findings, nil
}

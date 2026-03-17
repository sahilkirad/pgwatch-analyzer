package analysis

import (
	"context"
	"fmt"
	"strings"

	"pgwatchai/internal/db"
	"pgwatchai/internal/model"
	"pgwatchai/internal/reader"
)

type ReplicationAnalyzer struct {
	WarnLagS float64
	CritLagS float64
}

func NewReplicationAnalyzer() *ReplicationAnalyzer {
	return &ReplicationAnalyzer{
		WarnLagS: 5,
		CritLagS: 30,
	}
}

func (a *ReplicationAnalyzer) Name() model.IntentKind {
	return model.IntentReplication
}

func (a *ReplicationAnalyzer) Analyze(ctx context.Context, exec model.ExecutionContext) ([]model.Finding, error) {
	if strings.TrimSpace(exec.SinkDSN) == "" {
		return nil, fmt.Errorf("sink dsn is empty in execution context")
	}

	pool, err := db.NewPool(ctx, db.DefaultConfig(exec.SinkDSN))
	if err != nil {
		return nil, fmt.Errorf("open sink pool: %w", err)
	}
	defer db.ClosePool(pool)

	sinkReader := reader.NewSinkReader(pool)
	stats, err := sinkReader.GetReplicationLag(ctx, exec.Since, exec.DBFilter)
	if err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return []model.Finding{{
			ID:       "repl-none",
			Title:    "No replication rows found in time window",
			Severity: model.SeverityInfo,
			Category: model.IntentReplication,
			Summary:  "No replication metrics were returned from pgwatch sink for the selected time range.",
		}}, nil
	}

	findings := make([]model.Finding, 0, len(stats))
	for i, s := range stats {
		primaryLag := s.ReplayLagS
		sev := model.SeverityInfo
		if primaryLag >= a.CritLagS {
			sev = model.SeverityCritical
		} else if primaryLag >= a.WarnLagS {
			sev = model.SeverityWarning
		}

		findings = append(findings, model.Finding{
			ID:       fmt.Sprintf("repl-%d", i+1),
			Title:    "Replication lag status",
			Severity: sev,
			Category: model.IntentReplication,
			Summary: fmt.Sprintf(
				"DB=%s replay_lag=%.2fs write_lag=%.2fs flush_lag=%.2fs apply_lag=%.2fs",
				s.DBName, s.ReplayLagS, s.WriteLagS, s.FlushLagS, s.ApplyLagS,
			),
			Evidence: map[string]any{
				"db_name":      s.DBName,
				"replay_lag_s": s.ReplayLagS,
				"write_lag_s":  s.WriteLagS,
				"flush_lag_s":  s.FlushLagS,
				"apply_lag_s":  s.ApplyLagS,
			},
			Suggestion: "Check WAL generation spikes, network/IO throughput, and standby apply performance.",
		})
	}

	return findings, nil
}

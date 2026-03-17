package analysis

import (
	"context"
	"fmt"
	"strings"

	"pgwatchai/internal/db"
	"pgwatchai/internal/model"
	"pgwatchai/internal/reader"
)

type LocksAnalyzer struct {
	WarnWaiting float64
	CritWaiting float64
}

func NewLocksAnalyzer() *LocksAnalyzer {
	return &LocksAnalyzer{
		WarnWaiting: 5,
		CritWaiting: 20,
	}
}

func (a *LocksAnalyzer) Name() model.IntentKind {
	return model.IntentLocks
}

func (a *LocksAnalyzer) Analyze(ctx context.Context, exec model.ExecutionContext) ([]model.Finding, error) {
	if strings.TrimSpace(exec.SinkDSN) == "" {
		return nil, fmt.Errorf("sink dsn is empty in execution context")
	}

	pool, err := db.NewPool(ctx, db.DefaultConfig(exec.SinkDSN))
	if err != nil {
		return nil, fmt.Errorf("open sink pool: %w", err)
	}
	defer db.ClosePool(pool)

	sinkReader := reader.NewSinkReader(pool)
	stats, err := sinkReader.GetLockContention(ctx, exec.Since, exec.DBFilter)
	if err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return []model.Finding{{
			ID:       "lock-none",
			Title:    "No lock-contention rows found in time window",
			Severity: model.SeverityInfo,
			Category: model.IntentLocks,
			Summary:  "No lock metrics were returned from pgwatch sink for the selected time range.",
		}}, nil
	}

	findings := make([]model.Finding, 0, len(stats))
	for i, s := range stats {
		sev := model.SeverityInfo
		if s.WaitingCount >= a.CritWaiting {
			sev = model.SeverityCritical
		} else if s.WaitingCount >= a.WarnWaiting {
			sev = model.SeverityWarning
		}

		findings = append(findings, model.Finding{
			ID:       fmt.Sprintf("lock-%d", i+1),
			Title:    "Lock contention status",
			Severity: sev,
			Category: model.IntentLocks,
			Summary: fmt.Sprintf(
				"DB=%s waiting=%.0f blocking=%.0f deadlocks=%.0f",
				s.DBName, s.WaitingCount, s.BlockingCount, s.Deadlocks,
			),
			Evidence: map[string]any{
				"db_name":        s.DBName,
				"waiting_count":  s.WaitingCount,
				"blocking_count": s.BlockingCount,
				"deadlocks":      s.Deadlocks,
			},
			Suggestion: "Investigate blockers and long-running transactions; terminate or optimize offending sessions if needed.",
		})
	}
	return findings, nil
}

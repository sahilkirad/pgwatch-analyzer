package analysis

import (
	"context"
	"fmt"
	"strings"

	"pgwatchai/internal/db"
	"pgwatchai/internal/model"
	"pgwatchai/internal/reader"
)

type ConnectionsAnalyzer struct {
	WarnPeakBackends float64
	CritPeakBackends float64
}

func NewConnectionsAnalyzer() *ConnectionsAnalyzer {
	return &ConnectionsAnalyzer{
		WarnPeakBackends: 30,
		CritPeakBackends: 60,
	}
}

func (a *ConnectionsAnalyzer) Name() model.IntentKind {
	return model.IntentConnections
}

func (a *ConnectionsAnalyzer) Analyze(ctx context.Context, exec model.ExecutionContext) ([]model.Finding, error) {
	if strings.TrimSpace(exec.SinkDSN) == "" {
		return nil, fmt.Errorf("sink dsn is empty in execution context")
	}

	pool, err := db.NewPool(ctx, db.DefaultConfig(exec.SinkDSN))
	if err != nil {
		return nil, fmt.Errorf("open sink pool: %w", err)
	}
	defer db.ClosePool(pool)

	sinkReader := reader.NewSinkReader(pool)
	stats, err := sinkReader.GetConnectionPressure(ctx, exec.Since, exec.DBFilter)
	if err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return []model.Finding{
			{
				ID:       "conn-none",
				Title:    "No connection-pressure rows found in time window",
				Severity: model.SeverityInfo,
				Category: model.IntentConnections,
				Summary:  "No connection metrics were returned from pgwatch sink for the selected time range.",
			},
		}, nil
	}

	findings := make([]model.Finding, 0, len(stats))
	for i, s := range stats {
		sev := model.SeverityInfo
		if s.PeakBackends >= a.CritPeakBackends {
			sev = model.SeverityCritical
		} else if s.PeakBackends >= a.WarnPeakBackends {
			sev = model.SeverityWarning
		}

		summary := fmt.Sprintf(
			"DB=%s peak_backends=%.0f avg_backends=%.1f peak_active=%.0f peak_idle=%.0f",
			s.DBName, s.PeakBackends, s.AvgBackends, s.PeakActive, s.PeakIdle,
		)

		findings = append(findings, model.Finding{
			ID:       fmt.Sprintf("conn-%d", i+1),
			Title:    "Connection pressure status",
			Severity: sev,
			Category: model.IntentConnections,
			Summary:  summary,
			Evidence: map[string]any{
				"db_name":       s.DBName,
				"peak_backends": s.PeakBackends,
				"avg_backends":  s.AvgBackends,
				"peak_active":   s.PeakActive,
				"peak_idle":     s.PeakIdle,
			},
			Suggestion: "If peak_backends stays high, tune connection pools, reduce idle clients, and investigate bursty traffic.",
		})
	}

	return findings, nil
}

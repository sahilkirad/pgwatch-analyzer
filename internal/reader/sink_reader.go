package reader

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SinkReader struct {
	pool *pgxpool.Pool
}

type SlowQueryStat struct {
	DBName      string
	QueryID     string
	QueryText   string
	MeanExecMS  float64
	TotalExecMS float64
	Calls       float64
}

type ConnectionStat struct {
	DBName       string
	PeakBackends float64
	AvgBackends  float64
	PeakActive   float64
	PeakIdle     float64
}

type LockStat struct {
	DBName        string
	WaitingCount  float64
	BlockingCount float64
	Deadlocks     float64
}

type ReplicationStat struct {
	DBName     string
	ReplayLagS float64
	WriteLagS  float64
	FlushLagS  float64
	ApplyLagS  float64
}

func (r *SinkReader) GetLockContention(ctx context.Context, since time.Duration, dbFilter string) ([]LockStat, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("sink pool is nil")
	}

	tableName, err := r.pickFirstExistingMetricTable(ctx, []string{"locks", "lock_waits", "stat_activity"})
	if err != nil {
		return nil, err
	}

	intervalArg := fmt.Sprintf("%d seconds", int(since.Seconds()))
	sql := fmt.Sprintf(`
WITH base AS (
	SELECT
		dbname,
		COALESCE(NULLIF(data->>'waiting','')::double precision, NULLIF(data->>'waiting_locks','')::double precision, 0) AS waiting_count,
		COALESCE(NULLIF(data->>'blocking','')::double precision, NULLIF(data->>'blocking_locks','')::double precision, 0) AS blocking_count,
		COALESCE(NULLIF(data->>'deadlocks','')::double precision, 0) AS deadlocks
	FROM public.%s
	WHERE time >= now() - $1::interval
	  AND ($2 = '' OR dbname = $2)
)
SELECT dbname, MAX(waiting_count), MAX(blocking_count), MAX(deadlocks)
FROM base
GROUP BY dbname
ORDER BY MAX(waiting_count) DESC, MAX(deadlocks) DESC;
`, tableName)

	rows, err := r.pool.Query(ctx, sql, intervalArg, dbFilter)
	if err != nil {
		return nil, fmt.Errorf("query locks from %s: %w", tableName, err)
	}
	defer rows.Close()

	out := make([]LockStat, 0, 16)
	for rows.Next() {
		var s LockStat
		if err := rows.Scan(&s.DBName, &s.WaitingCount, &s.BlockingCount, &s.Deadlocks); err != nil {
			return nil, fmt.Errorf("scan lock row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lock rows: %w", err)
	}
	return out, nil
}

func (r *SinkReader) GetReplicationLag(ctx context.Context, since time.Duration, dbFilter string) ([]ReplicationStat, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("sink pool is nil")
	}

	tableName, err := r.pickFirstExistingMetricTable(ctx, []string{
		"replication",
		"replication_lag",
		"wal_receiver",
	})
	if err != nil {
		return nil, err
	}

	intervalArg := fmt.Sprintf("%d seconds", int(since.Seconds()))
	sql := fmt.Sprintf(`
WITH latest AS (
	SELECT DISTINCT ON (dbname)
		dbname,
		COALESCE(NULLIF(data->>'replay_lag','')::double precision, 0) AS replay_lag_s,
		COALESCE(NULLIF(data->>'write_lag','')::double precision, 0) AS write_lag_s,
		COALESCE(NULLIF(data->>'flush_lag','')::double precision, 0) AS flush_lag_s,
		COALESCE(NULLIF(data->>'apply_lag','')::double precision, 0) AS apply_lag_s
	FROM public.%s
	WHERE time >= now() - $1::interval
	  AND ($2 = '' OR dbname = $2)
	ORDER BY dbname, time DESC
)
SELECT dbname, replay_lag_s, write_lag_s, flush_lag_s, apply_lag_s
FROM latest
ORDER BY replay_lag_s DESC, apply_lag_s DESC;
`, tableName)

	rows, err := r.pool.Query(ctx, sql, intervalArg, dbFilter)
	if err != nil {
		return nil, fmt.Errorf("query replication from %s: %w", tableName, err)
	}
	defer rows.Close()

	out := make([]ReplicationStat, 0, 16)
	for rows.Next() {
		var s ReplicationStat
		if err := rows.Scan(&s.DBName, &s.ReplayLagS, &s.WriteLagS, &s.FlushLagS, &s.ApplyLagS); err != nil {
			return nil, fmt.Errorf("scan replication row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate replication rows: %w", err)
	}
	return out, nil
}

func NewSinkReader(pool *pgxpool.Pool) *SinkReader {
	return &SinkReader{pool: pool}
}

func (r *SinkReader) GetSlowQueries(ctx context.Context, since time.Duration, dbFilter string, limit int) ([]SlowQueryStat, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("sink pool is nil")
	}
	if limit <= 0 {
		limit = 5
	}

	tableName, err := r.pickFirstExistingMetricTable(ctx, []string{
		"stat_statements",
		"stat_statements_time",
		"stat_statements_calls",
	})
	if err != nil {
		return nil, err
	}

	intervalArg := fmt.Sprintf("%d seconds", int(since.Seconds()))
	sql := fmt.Sprintf(`
WITH base AS (
	SELECT
		dbname,
		COALESCE(NULLIF(tag_data->>'queryid',''), NULLIF(data->>'queryid',''), 'unknown') AS query_id,
		COALESCE(NULLIF(data->>'query',''), NULLIF(tag_data->>'query',''), '<query text not available>') AS query_text,
		COALESCE(
			NULLIF(data->>'mean_exec_time','')::double precision,
			NULLIF(data->>'mean_time','')::double precision,
			CASE
				WHEN COALESCE(NULLIF(data->>'calls','')::double precision, 0) > 0
				 AND COALESCE(NULLIF(data->>'total_exec_time','')::double precision, NULLIF(data->>'total_time','')::double precision, 0) > 0
				THEN COALESCE(NULLIF(data->>'total_exec_time','')::double precision, NULLIF(data->>'total_time','')::double precision, 0)
				   / COALESCE(NULLIF(data->>'calls','')::double precision, 1)
				ELSE 0
			END
		) AS mean_exec_ms,
		COALESCE(NULLIF(data->>'total_exec_time','')::double precision, NULLIF(data->>'total_time','')::double precision, 0) AS total_exec_ms,
		COALESCE(NULLIF(data->>'calls','')::double precision, 0) AS calls
	FROM public.%s
	WHERE time >= now() - $1::interval
	  AND ($2 = '' OR dbname = $2)
),
agg AS (
	SELECT
		dbname,
		query_id,
		MAX(query_text) AS query_text,
		AVG(mean_exec_ms) AS mean_exec_ms,
		MAX(total_exec_ms) AS total_exec_ms,
		MAX(calls) AS calls
	FROM base
	GROUP BY dbname, query_id
)
SELECT
	dbname, query_id, query_text, mean_exec_ms, total_exec_ms, calls
FROM agg
ORDER BY mean_exec_ms DESC, total_exec_ms DESC
LIMIT $3;

`, tableName)

	rows, err := r.pool.Query(ctx, sql, intervalArg, dbFilter, limit)
	if err != nil {
		return nil, fmt.Errorf("query slow queries from %s: %w", tableName, err)
	}
	defer rows.Close()

	out := make([]SlowQueryStat, 0, limit)
	for rows.Next() {
		var s SlowQueryStat
		if err := rows.Scan(&s.DBName, &s.QueryID, &s.QueryText, &s.MeanExecMS, &s.TotalExecMS, &s.Calls); err != nil {
			return nil, fmt.Errorf("scan slow query row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate slow query rows: %w", err)
	}
	return out, nil
}

func (r *SinkReader) GetConnectionPressure(ctx context.Context, since time.Duration, dbFilter string) ([]ConnectionStat, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("sink pool is nil")
	}

	tableName, err := r.pickFirstExistingMetricTable(ctx, []string{
		"db_stats",
		"stat_database",
		"sessions",
	})
	if err != nil {
		return nil, err
	}

	intervalArg := fmt.Sprintf("%d seconds", int(since.Seconds()))
	sql := fmt.Sprintf(`
WITH base AS (
	SELECT
		dbname,
		COALESCE(NULLIF(data->>'numbackends','')::double precision, 0) AS num_backends,
		COALESCE(NULLIF(data->>'active_backends','')::double precision, NULLIF(data->>'numbackends_active','')::double precision, 0) AS active_backends,
		COALESCE(NULLIF(data->>'idle_backends','')::double precision, NULLIF(data->>'numbackends_idle','')::double precision, 0) AS idle_backends
	FROM public.%s
	WHERE time >= now() - $1::interval
	  AND ($2 = '' OR dbname = $2)
)
SELECT
	dbname,
	MAX(num_backends) AS peak_backends,
	AVG(num_backends) AS avg_backends,
	MAX(active_backends) AS peak_active,
	MAX(idle_backends) AS peak_idle
FROM base
GROUP BY dbname
ORDER BY peak_backends DESC;
`, tableName)

	rows, err := r.pool.Query(ctx, sql, intervalArg, dbFilter)
	if err != nil {
		return nil, fmt.Errorf("query connections from %s: %w", tableName, err)
	}
	defer rows.Close()

	out := make([]ConnectionStat, 0, 16)
	for rows.Next() {
		var s ConnectionStat
		if err := rows.Scan(
			&s.DBName,
			&s.PeakBackends,
			&s.AvgBackends,
			&s.PeakActive,
			&s.PeakIdle,
		); err != nil {
			return nil, fmt.Errorf("scan connection row: %w", err)
		}

		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connection rows: %w", err)
	}
	return out, nil
}

func (r *SinkReader) pickFirstExistingMetricTable(ctx context.Context, candidates []string) (string, error) {
	for _, name := range candidates {
		var regclass *string
		if err := r.pool.QueryRow(ctx, `SELECT to_regclass($1)`, "public."+name).Scan(&regclass); err != nil {
			return "", fmt.Errorf("check metric table %s: %w", name, err)
		}
		if regclass != nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("none of candidate metric tables exist: %v", candidates)
}

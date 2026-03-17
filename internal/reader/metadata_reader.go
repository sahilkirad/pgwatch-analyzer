package reader

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MetadataReader struct {
	pool *pgxpool.Pool
}

func NewMetadataReader(pool *pgxpool.Pool) *MetadataReader {
	return &MetadataReader{pool: pool}
}

// GetSources returns source inventory from pgwatch config DB.
func (r *MetadataReader) GetSources(ctx context.Context) ([]map[string]any, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("metadata pool is nil")
	}

	sql := `
SELECT name, "group", dbtype, is_enabled
FROM pgwatch.source
ORDER BY name;
`
	return queryAsMap(ctx, r.pool, sql)
}

// GetMetricDefs returns metric descriptions from pgwatch config DB.
func (r *MetadataReader) GetMetricDefs(ctx context.Context) ([]map[string]any, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("metadata pool is nil")
	}

	sql := `
SELECT name, description, storage_name
FROM pgwatch.metric
ORDER BY name;
`
	return queryAsMap(ctx, r.pool, sql)
}

func queryAsMap(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) ([]map[string]any, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query metadata: %w", err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	out := make([]map[string]any, 0, 64)

	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("read metadata row: %w", err)
		}
		m := make(map[string]any, len(fields))
		for i, f := range fields {
			m[string(f.Name)] = vals[i]
		}
		out = append(out, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metadata rows: %w", err)
	}
	return out, nil
}

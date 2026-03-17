package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DSN            string
	MaxConns       int32
	MinConns       int32
	ConnectTimeout time.Duration
	HealthTimeout  time.Duration
}

func DefaultConfig(dsn string) Config {
	return Config{
		DSN:            dsn,
		MaxConns:       10,
		MinConns:       1,
		ConnectTimeout: 10 * time.Second,
		HealthTimeout:  3 * time.Second,
	}
}

func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	fmt.Printf("DEBUG DSN host=%s user=%s db=%s pass_len=%d\n",
		pcfg.ConnConfig.Host,
		pcfg.ConnConfig.User,
		pcfg.ConnConfig.Database,
		len(pcfg.ConnConfig.Password),
	)

	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	pcfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	hctx, cancel := context.WithTimeout(ctx, cfg.HealthTimeout)
	defer cancel()
	if err := pool.Ping(hctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}

func ClosePool(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

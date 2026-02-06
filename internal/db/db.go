package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Url    string
	Schema string
}

func InitDB(ctx context.Context, url string, schema string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2

	// Set search_path for the connection pool using RuntimeParams
	if schema != "" {
		poolConfig.ConnConfig.RuntimeParams["search_path"] = schema
		slog.Info("Setting search_path for connection pool", "schema", schema)

		// Also set an after-connect hook to ensure search_path is set for each connection
		// This is especially important for connection poolers like PgBouncer (used by Supabase)
		// that may reset session-level settings between transactions
		poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s", pgx.Identifier{schema}.Sanitize()))
			if err != nil {
				slog.Warn("Failed to set search_path in AfterConnect", "error", err)
				return err
			}
			slog.Debug("Set search_path for new connection", "schema", schema)
			return nil
		}
	}

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	slog.Info("Connected to PostgreSQL")

	return pool, nil
}

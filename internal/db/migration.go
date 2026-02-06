package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// RunMigrations runs all pending database migrations.
func RunMigrations(dbURL string, schema string) error {
	slog.Info("Running database migrations...")

	// Use default schema if not specified
	if schema == "" {
		schema = "public"
	}

	// Open database connection for migrations using pgx driver
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// Verify connectivity before running migrations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	// Create schema if it doesn't exist and set search_path
	if err := ensureSchemaExists(db, schema); err != nil {
		return err
	}

	// Set goose to use embedded migrations
	goose.SetBaseFS(embedMigrations)

	// Run migrations
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}

	slog.Info("Database migrations completed successfully")
	return nil
}

func ensureSchemaExists(db *sql.DB, schema string) error {
	// Create schema if it doesn't exist
	query := "CREATE SCHEMA IF NOT EXISTS " + pgx.Identifier{schema}.Sanitize()
	_, err := db.Exec(query)
	if err != nil {
		return err
	}
	slog.Info("Schema is ready", "schema", schema)

	// Set search_path to the schema to ensure migrations run there
	setPathQuery := "SET search_path TO " + pgx.Identifier{schema}.Sanitize()
	_, err = db.Exec(setPathQuery)
	if err != nil {
		return err
	}
	slog.Info("Set search_path", "schema", schema)

	return nil
}

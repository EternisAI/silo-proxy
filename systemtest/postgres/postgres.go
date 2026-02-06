package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartPostgres(ctx context.Context, dbUser, dbPassword, dbName string) (*postgres.PostgresContainer, error) {
	container, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.WithDatabase(dbName),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start Postgres container: %w", err)
	}

	state, err := container.State(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container state: %w", err)
	}

	if !state.Running {
		return nil, fmt.Errorf("postgres container is not running")
	}

	return container, nil
}

func TerminatePostgres(ctx context.Context, container *postgres.PostgresContainer) error {
	if err := container.Terminate(ctx); err != nil {
		return fmt.Errorf("failed to terminate Postgres container: %w", err)
	}
	return nil
}

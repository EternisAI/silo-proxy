package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrUserNotFound = errors.New("user not found")

type UserInfo struct {
	ID        string
	Username  string
	Role      string
	CreatedAt time.Time
}

type Service struct {
	queries *sqlc.Queries
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return ErrUserNotFound
	}

	pgID := pgtype.UUID{Bytes: parsed, Valid: true}
	if err := s.queries.DeleteUser(ctx, pgID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]UserInfo, int64, error) {
	dbUsers, err := s.queries.ListUsersPaginated(ctx, sqlc.ListUsersPaginatedParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	total, err := s.queries.CountUsers(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	result := make([]UserInfo, len(dbUsers))
	for i, u := range dbUsers {
		result[i] = UserInfo{
			ID:        uuidToString(u.ID.Bytes),
			Username:  u.Username,
			Role:      string(u.Role),
			CreatedAt: u.CreatedAt.Time,
		}
	}
	return result, total, nil
}

func uuidToString(id [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
}

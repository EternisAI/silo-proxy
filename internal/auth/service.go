package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
	"github.com/EternisAI/silo-proxy/internal/users"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUsernameExists     = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type RegisterResult struct {
	ID       string
	Username string
	Role     string
}

type Service struct {
	queries *sqlc.Queries
	config  Config
}

func NewService(queries *sqlc.Queries, config Config) *Service {
	return &Service{
		queries: queries,
		config:  config,
	}
}

func (s *Service) Register(ctx context.Context, username, password string) (RegisterResult, error) {
	hash, err := users.HashPassword(password)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.queries.CreateUser(ctx, sqlc.CreateUserParams{
		Username:     username,
		PasswordHash: hash,
		Role:         sqlc.UserRoleUser,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return RegisterResult{}, ErrUsernameExists
		}
		return RegisterResult{}, fmt.Errorf("create user: %w", err)
	}

	return RegisterResult{
		ID:       uuidToString(user.ID.Bytes),
		Username: user.Username,
		Role:     string(user.Role),
	}, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInvalidCredentials
		}
		return "", fmt.Errorf("query user: %w", err)
	}

	if !users.CheckPassword(password, user.PasswordHash) {
		return "", ErrInvalidCredentials
	}

	token, err := GenerateToken(s.config, uuidToString(user.ID.Bytes), user.Username, string(user.Role))
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

func uuidToString(id [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
}

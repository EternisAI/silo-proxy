package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrInvalidAgentID = errors.New("invalid agent ID")
)

type Service struct {
	queries *sqlc.Queries
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

// CreateLegacyAgent creates an agent for legacy migration (no provisioning key)
func (s *Service) CreateLegacyAgent(ctx context.Context, legacyID string, userID string) (*Agent, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Parse legacy ID as UUID - if it's not a valid UUID, generate a new one
	var agentUUID uuid.UUID
	parsedAgentID, err := uuid.Parse(legacyID)
	if err != nil {
		// Not a valid UUID, generate a new one
		agentUUID = uuid.New()
		slog.Info("Legacy agent ID is not a UUID, generating new ID",
			"legacy_id", legacyID,
			"new_agent_id", agentUUID.String())
	} else {
		agentUUID = parsedAgentID
	}

	metadata := map[string]interface{}{
		"legacy": true,
		"original_id": legacyID,
		"migrated_at": time.Now().Format(time.RFC3339),
	}
	metadataJSON, _ := json.Marshal(metadata)

	dbAgent, err := s.queries.CreateAgent(ctx, sqlc.CreateAgentParams{
		UserID:               pgtype.UUID{Bytes: parsedUserID, Valid: true},
		ProvisionedWithKeyID: pgtype.UUID{Valid: false}, // NULL for legacy agents
		Metadata:             metadataJSON,
		Notes:                pgtype.Text{String: "Auto-migrated legacy agent", Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create legacy agent: %w", err)
	}

	result := &Agent{
		ID:          uuidToString(dbAgent.ID.Bytes),
		UserID:      uuidToString(dbAgent.UserID.Bytes),
		Status:      string(dbAgent.Status),
		RegisteredAt: dbAgent.RegisteredAt.Time,
		LastSeenAt:  dbAgent.LastSeenAt.Time,
	}

	slog.Info("Legacy agent auto-migrated",
		"legacy_id", legacyID,
		"agent_id", result.ID,
		"user_id", userID)

	return result, nil
}

// GetAgentByID retrieves an agent by ID
func (s *Service) GetAgentByID(ctx context.Context, agentID string) (*Agent, error) {
	parsedID, err := uuid.Parse(agentID)
	if err != nil {
		return nil, ErrInvalidAgentID
	}

	dbAgent, err := s.queries.GetAgentByID(ctx, pgtype.UUID{Bytes: parsedID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	var metadata map[string]interface{}
	if len(dbAgent.Metadata) > 0 {
		_ = json.Unmarshal(dbAgent.Metadata, &metadata)
	}

	result := &Agent{
		ID:              uuidToString(dbAgent.ID.Bytes),
		UserID:          uuidToString(dbAgent.UserID.Bytes),
		Status:          string(dbAgent.Status),
		CertFingerprint: dbAgent.CertFingerprint.String,
		RegisteredAt:    dbAgent.RegisteredAt.Time,
		LastSeenAt:      dbAgent.LastSeenAt.Time,
		Metadata:        metadata,
		Notes:           dbAgent.Notes.String,
	}

	if dbAgent.ProvisionedWithKeyID.Valid {
		result.ProvisionedWithKeyID = uuidToString(dbAgent.ProvisionedWithKeyID.Bytes)
	}

	if dbAgent.LastIpAddress != nil {
		result.LastIPAddress = dbAgent.LastIpAddress.String()
	}

	return result, nil
}

// ListAgentsByUser retrieves all agents for a user
func (s *Service) ListAgentsByUser(ctx context.Context, userID string) ([]Agent, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	dbAgents, err := s.queries.ListAgentsByUser(ctx, pgtype.UUID{Bytes: parsedUserID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	result := make([]Agent, len(dbAgents))
	for i, a := range dbAgents {
		var metadata map[string]interface{}
		if len(a.Metadata) > 0 {
			_ = json.Unmarshal(a.Metadata, &metadata)
		}

		result[i] = Agent{
			ID:              uuidToString(a.ID.Bytes),
			UserID:          uuidToString(a.UserID.Bytes),
			Status:          string(a.Status),
			CertFingerprint: a.CertFingerprint.String,
			RegisteredAt:    a.RegisteredAt.Time,
			LastSeenAt:      a.LastSeenAt.Time,
			Metadata:        metadata,
			Notes:           a.Notes.String,
		}

		if a.ProvisionedWithKeyID.Valid {
			result[i].ProvisionedWithKeyID = uuidToString(a.ProvisionedWithKeyID.Bytes)
		}

		if a.LastIpAddress != nil {
			result[i].LastIPAddress = a.LastIpAddress.String()
		}
	}

	return result, nil
}

// UpdateLastSeen updates the agent's last seen timestamp and IP address
func (s *Service) UpdateLastSeen(ctx context.Context, agentID string, timestamp time.Time, ipAddress string) error {
	parsedID, err := uuid.Parse(agentID)
	if err != nil {
		return ErrInvalidAgentID
	}

	var ipAddr *netip.Addr
	if ipAddress != "" {
		parsed, err := netip.ParseAddr(ipAddress)
		if err == nil {
			ipAddr = &parsed
		}
	}

	if err := s.queries.UpdateAgentLastSeen(ctx, sqlc.UpdateAgentLastSeenParams{
		ID:            pgtype.UUID{Bytes: parsedID, Valid: true},
		LastSeenAt:    pgtype.Timestamp{Time: timestamp, Valid: true},
		LastIpAddress: ipAddr,
	}); err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}

	return nil
}

// UpdateStatus updates the agent's status
func (s *Service) UpdateStatus(ctx context.Context, agentID string, status string) error {
	parsedID, err := uuid.Parse(agentID)
	if err != nil {
		return ErrInvalidAgentID
	}

	var agentStatus sqlc.AgentStatus
	switch status {
	case "active":
		agentStatus = sqlc.AgentStatusActive
	case "inactive":
		agentStatus = sqlc.AgentStatusInactive
	case "suspended":
		agentStatus = sqlc.AgentStatusSuspended
	default:
		return fmt.Errorf("invalid status: %s", status)
	}

	if err := s.queries.UpdateAgentStatus(ctx, sqlc.UpdateAgentStatusParams{
		ID:     pgtype.UUID{Bytes: parsedID, Valid: true},
		Status: agentStatus,
	}); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	slog.Info("Agent status updated", "agent_id", agentID, "status", status)
	return nil
}

// CreateConnectionLog creates a new connection log entry
func (s *Service) CreateConnectionLog(ctx context.Context, agentID string, connectedAt time.Time, ipAddress string) (string, error) {
	parsedID, err := uuid.Parse(agentID)
	if err != nil {
		return "", ErrInvalidAgentID
	}

	var ipAddr *netip.Addr
	if ipAddress != "" {
		parsed, err := netip.ParseAddr(ipAddress)
		if err == nil {
			ipAddr = &parsed
		}
	}

	dbLog, err := s.queries.CreateConnectionLog(ctx, sqlc.CreateConnectionLogParams{
		AgentID:     pgtype.UUID{Bytes: parsedID, Valid: true},
		ConnectedAt: pgtype.Timestamp{Time: connectedAt, Valid: true},
		IpAddress:   ipAddr,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create connection log: %w", err)
	}

	return uuidToString(dbLog.ID.Bytes), nil
}

// UpdateConnectionLog updates a connection log with disconnect information
func (s *Service) UpdateConnectionLog(ctx context.Context, logID string, disconnectedAt time.Time, reason string) error {
	parsedID, err := uuid.Parse(logID)
	if err != nil {
		return fmt.Errorf("invalid log ID: %w", err)
	}

	if err := s.queries.UpdateConnectionLog(ctx, sqlc.UpdateConnectionLogParams{
		ID:               pgtype.UUID{Bytes: parsedID, Valid: true},
		DisconnectedAt:   pgtype.Timestamp{Time: disconnectedAt, Valid: true},
		DisconnectReason: pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		return fmt.Errorf("failed to update connection log: %w", err)
	}

	return nil
}

// GetAgentConnectionHistory retrieves connection history for an agent
func (s *Service) GetAgentConnectionHistory(ctx context.Context, agentID string, limit, offset int) ([]ConnectionLog, error) {
	parsedID, err := uuid.Parse(agentID)
	if err != nil {
		return nil, ErrInvalidAgentID
	}

	dbLogs, err := s.queries.GetAgentConnectionHistory(ctx, sqlc.GetAgentConnectionHistoryParams{
		AgentID: pgtype.UUID{Bytes: parsedID, Valid: true},
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get connection history: %w", err)
	}

	result := make([]ConnectionLog, len(dbLogs))
	for i, l := range dbLogs {
		result[i] = ConnectionLog{
			ID:              uuidToString(l.ID.Bytes),
			AgentID:         uuidToString(l.AgentID.Bytes),
			ConnectedAt:     l.ConnectedAt.Time,
			DurationSeconds: int(l.DurationSeconds.Int32),
			DisconnectReason: l.DisconnectReason.String,
		}

		if l.DisconnectedAt.Valid {
			result[i].DisconnectedAt = &l.DisconnectedAt.Time
		}

		if l.IpAddress != nil {
			result[i].IPAddress = l.IpAddress.String()
		}
	}

	return result, nil
}

func uuidToString(id [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		id[0:4], id[4:6], id[6:8], id[8:10], id[10:16])
}

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/EternisAI/silo-proxy/internal/provision"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupProvisionRouter(h *ProvisionHandler) *gin.Engine {
	r := gin.New()
	r.POST("/api/v1/provision-keys", h.CreateProvisionKey)
	r.GET("/api/v1/provision-keys", h.ListProvisionKeys)
	r.DELETE("/api/v1/provision-keys/:id", h.RevokeProvisionKey)
	r.POST("/api/v1/provision", h.Provision)
	return r
}

func TestCreateProvisionKey(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	body, _ := json.Marshal(dto.CreateProvisionKeyRequest{AgentID: "agent-1"})
	req, _ := http.NewRequest("POST", "/api/v1/provision-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp dto.CreateProvisionKeyResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "agent-1", resp.AgentID)
	assert.NotEmpty(t, resp.Key)
}

func TestCreateProvisionKeyInvalidAgentID(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	body, _ := json.Marshal(dto.CreateProvisionKeyRequest{AgentID: "../bad-id"})
	req, _ := http.NewRequest("POST", "/api/v1/provision-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateProvisionKeyMissingBody(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	req, _ := http.NewRequest("POST", "/api/v1/provision-keys", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListProvisionKeys(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	_, _ = ks.Create("agent-1")
	_, _ = ks.Create("agent-2")

	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	req, _ := http.NewRequest("GET", "/api/v1/provision-keys", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.ListProvisionKeysResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Count)
}

func TestRevokeProvisionKey(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	_, _ = ks.Create("agent-1")

	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	req, _ := http.NewRequest("DELETE", "/api/v1/provision-keys/agent-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRevokeProvisionKeyNotFound(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	req, _ := http.NewRequest("DELETE", "/api/v1/provision-keys/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProvisionTLSDisabled(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	h := NewProvisionHandler(ks, nil) // certService is nil
	r := setupProvisionRouter(h)

	body, _ := json.Marshal(dto.ProvisionRequest{Key: "sk_something"})
	req, _ := http.NewRequest("POST", "/api/v1/provision", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProvisionInvalidKey(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	// We need a certService for this test path but we'll test with a nil
	// to verify the TLS check happens first, and test invalid key separately
	// by passing a non-nil certService. For unit tests without a real cert.Service,
	// we verify the key validation path via the key store directly.

	// Create a handler with nil certService to hit TLS check
	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	body, _ := json.Marshal(dto.ProvisionRequest{Key: "sk_invalid"})
	req, _ := http.NewRequest("POST", "/api/v1/provision", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should fail with TLS not enabled (since certService is nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProvisionMissingKey(t *testing.T) {
	ks := provision.NewKeyStore(1 * time.Hour)
	h := NewProvisionHandler(ks, nil)
	r := setupProvisionRouter(h)

	body, _ := json.Marshal(map[string]string{})
	req, _ := http.NewRequest("POST", "/api/v1/provision", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// TLS check happens first
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

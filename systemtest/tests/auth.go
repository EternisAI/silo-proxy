package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/EternisAI/silo-proxy/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T, router *gin.Engine, jwtSecret string) {
	t.Run("success", func(t *testing.T) {
		body := dto.RegisterRequest{Username: "testuser", Password: "password123"}
		rr := doJSON(router, "POST", "/auth/register", body)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var resp dto.RegisterResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "testuser", resp.Username)
		assert.Equal(t, "Student", resp.Role)
		assert.NotEmpty(t, resp.ID)
	})

	t.Run("duplicate username", func(t *testing.T) {
		body := dto.RegisterRequest{Username: "dupuser", Password: "password123"}
		rr := doJSON(router, "POST", "/auth/register", body)
		require.Equal(t, http.StatusCreated, rr.Code)

		rr = doJSON(router, "POST", "/auth/register", body)
		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("missing username", func(t *testing.T) {
		body := dto.RegisterRequest{Password: "password123"}
		rr := doJSON(router, "POST", "/auth/register", body)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("password too short", func(t *testing.T) {
		body := dto.RegisterRequest{Username: "shortpw", Password: "short"}
		rr := doJSON(router, "POST", "/auth/register", body)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestLogin(t *testing.T, router *gin.Engine, jwtSecret string) {
	// Create a user to login with
	regBody := dto.RegisterRequest{Username: "loginuser", Password: "password123"}
	rr := doJSON(router, "POST", "/auth/register", regBody)
	require.Equal(t, http.StatusCreated, rr.Code)

	t.Run("success", func(t *testing.T) {
		body := dto.LoginRequest{Username: "loginuser", Password: "password123"}
		rr := doJSON(router, "POST", "/auth/login", body)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp dto.LoginResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp.Token)

		claims, err := auth.ValidateToken(jwtSecret, resp.Token)
		require.NoError(t, err)
		assert.Equal(t, "loginuser", claims.Username)
		assert.Equal(t, "Student", claims.Role)
	})

	t.Run("wrong password", func(t *testing.T) {
		body := dto.LoginRequest{Username: "loginuser", Password: "wrongpassword"}
		rr := doJSON(router, "POST", "/auth/login", body)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("nonexistent user", func(t *testing.T) {
		body := dto.LoginRequest{Username: "nouser", Password: "password123"}
		rr := doJSON(router, "POST", "/auth/login", body)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func doJSON(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

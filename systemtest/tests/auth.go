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

func TestDeleteUser(t *testing.T, router *gin.Engine, jwtSecret string) {
	// Register a user
	regBody := dto.RegisterRequest{Username: "deleteuser", Password: "password123"}
	rr := doJSON(router, "POST", "/auth/register", regBody)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Login to get a token
	loginBody := dto.LoginRequest{Username: "deleteuser", Password: "password123"}
	rr = doJSON(router, "POST", "/auth/login", loginBody)
	require.Equal(t, http.StatusOK, rr.Code)

	var loginResp dto.LoginResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &loginResp))

	t.Run("success", func(t *testing.T) {
		rr := doJSONWithAuth(router, "DELETE", "/users/me", nil, loginResp.Token)
		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("login fails after deletion", func(t *testing.T) {
		rr := doJSON(router, "POST", "/auth/login", loginBody)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("401 without token", func(t *testing.T) {
		rr := doJSON(router, "DELETE", "/users/me", nil)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestListUsers(t *testing.T, router *gin.Engine, jwtSecret string) {
	// Login as root (admin)
	loginBody := dto.LoginRequest{Username: "root", Password: "changeme"}
	rr := doJSON(router, "POST", "/auth/login", loginBody)
	require.Equal(t, http.StatusOK, rr.Code)

	var loginResp dto.LoginResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &loginResp))

	t.Run("success", func(t *testing.T) {
		rr := doJSONWithAuth(router, "GET", "/users", nil, loginResp.Token)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp dto.ListUsersResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.GreaterOrEqual(t, resp.Total, int64(1))
		assert.Equal(t, 1, resp.Page)
		assert.Equal(t, 20, resp.PageSize)
		assert.NotEmpty(t, resp.Users)
	})

	t.Run("with pagination params", func(t *testing.T) {
		rr := doJSONWithAuth(router, "GET", "/users?page=1&page_size=2", nil, loginResp.Token)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp dto.ListUsersResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, 1, resp.Page)
		assert.Equal(t, 2, resp.PageSize)
	})

	t.Run("403 for non-admin", func(t *testing.T) {
		// Register a regular user
		regBody := dto.RegisterRequest{Username: "regularuser", Password: "password123"}
		rr := doJSON(router, "POST", "/auth/register", regBody)
		require.Equal(t, http.StatusCreated, rr.Code)

		loginBody := dto.LoginRequest{Username: "regularuser", Password: "password123"}
		rr = doJSON(router, "POST", "/auth/login", loginBody)
		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.LoginResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

		rr = doJSONWithAuth(router, "GET", "/users", nil, resp.Token)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("401 without token", func(t *testing.T) {
		rr := doJSON(router, "GET", "/users", nil)
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

func doJSONWithAuth(router *gin.Engine, method, path string, body any, token string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

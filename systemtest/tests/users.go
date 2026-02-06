package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserCRUD(t *testing.T, router *gin.Engine, jwtSecret string) {
	// Login as admin
	adminLogin := dto.LoginRequest{Username: "root", Password: "changeme"}
	rr := doJSON(router, "POST", "/auth/login", adminLogin)
	require.Equal(t, http.StatusOK, rr.Code)

	var adminResp dto.LoginResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &adminResp))

	t.Run("list users as admin", func(t *testing.T) {
		rr := doJSONWithAuth(router, "GET", "/users", nil, adminResp.Token)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp dto.ListUsersResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.GreaterOrEqual(t, resp.Total, int64(1))
		assert.Equal(t, 1, resp.Page)
		assert.Equal(t, 20, resp.PageSize)
		assert.NotEmpty(t, resp.Users)
	})

	t.Run("list users with pagination", func(t *testing.T) {
		rr := doJSONWithAuth(router, "GET", "/users?page=1&page_size=2", nil, adminResp.Token)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp dto.ListUsersResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, 1, resp.Page)
		assert.Equal(t, 2, resp.PageSize)
	})

	t.Run("list users 403 for non-admin", func(t *testing.T) {
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

	t.Run("list users 401 without token", func(t *testing.T) {
		rr := doJSON(router, "GET", "/users", nil)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("delete user", func(t *testing.T) {
		regBody := dto.RegisterRequest{Username: "deleteuser", Password: "password123"}
		rr := doJSON(router, "POST", "/auth/register", regBody)
		require.Equal(t, http.StatusCreated, rr.Code)

		loginBody := dto.LoginRequest{Username: "deleteuser", Password: "password123"}
		rr = doJSON(router, "POST", "/auth/login", loginBody)
		require.Equal(t, http.StatusOK, rr.Code)

		var loginResp dto.LoginResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &loginResp))

		rr = doJSONWithAuth(router, "DELETE", "/users/me", nil, loginResp.Token)
		require.Equal(t, http.StatusNoContent, rr.Code)

		// Login fails after deletion
		rr = doJSON(router, "POST", "/auth/login", loginBody)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("delete user 401 without token", func(t *testing.T) {
		rr := doJSON(router, "DELETE", "/users/me", nil)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

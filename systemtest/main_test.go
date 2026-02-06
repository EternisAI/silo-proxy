package systemtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/EternisAI/silo-proxy/internal/api/http"
	"github.com/EternisAI/silo-proxy/internal/auth"
	"github.com/EternisAI/silo-proxy/internal/db"
	"github.com/EternisAI/silo-proxy/internal/db/sqlc"
	"github.com/EternisAI/silo-proxy/internal/users"
	"github.com/EternisAI/silo-proxy/systemtest/postgres"
	"github.com/EternisAI/silo-proxy/systemtest/tests"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSystemIntegration(t *testing.T) {
	dbUser := "silo"
	dbPassword := "silo"
	dbName := "silo"
	dbHost := "localhost"
	schema := "public"
	jwtSecret := "test-jwt-secret-for-system-tests-only"

	container, err := postgres.StartPostgres(context.Background(), dbUser, dbPassword, dbName)
	assert.NoError(t, err)
	defer func() {
		err := postgres.TerminatePostgres(context.Background(), container)
		assert.NoError(t, err)
	}()

	port, err := container.MappedPort(context.Background(), "5432/tcp")
	assert.NoError(t, err)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", dbUser, dbPassword, dbHost, port.Int(), dbName)

	err = db.RunMigrations(dbURL, schema)
	assert.NoError(t, err)

	pool, err := db.InitDB(context.Background(), dbURL, schema)
	assert.NoError(t, err)
	defer pool.Close()

	queries := sqlc.New(pool)

	authService := auth.NewService(queries, auth.Config{Secret: jwtSecret, ExpirationMinutes: 60})
	userService := users.NewService(queries)

	services := &http.Services{
		AuthService: authService,
		UserService: userService,
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	http.SetupRoute(engine, services, "admin-api-key", jwtSecret)

	t.Run("HealthCheck", func(t *testing.T) { tests.TestHealthCheck(t, engine) })
	t.Run("Register", func(t *testing.T) { tests.TestRegister(t, engine, jwtSecret) })
	t.Run("Login", func(t *testing.T) { tests.TestLogin(t, engine, jwtSecret) })
	t.Run("UserCRUD", func(t *testing.T) { tests.TestUserCRUD(t, engine, jwtSecret) })
}

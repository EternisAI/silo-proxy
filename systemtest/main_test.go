package systemtest

import (
	"testing"

	"github.com/EternisAI/silo-proxy/systemtest/tests"
	"github.com/gin-gonic/gin"
)

func TestSystemIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	t.Run("HealthCheck", func(t *testing.T) { tests.TestHealthCheck(t, engine) })
}

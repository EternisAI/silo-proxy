package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	apiKeyHeader = "X-API-Key"
)

func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			slog.Warn("Admin API key not configured, rejecting request",
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "Admin API is not configured",
			})
			return
		}

		providedKey := c.GetHeader(apiKeyHeader)
		if providedKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing API key",
			})
			return
		}

		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) != 1 {
			slog.Warn("Invalid API key attempt",
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			return
		}

		c.Next()
	}
}

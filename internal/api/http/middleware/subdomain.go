package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const SubdomainAgentIDKey = "subdomain_agent_id"

func SubdomainExtractor() gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host

		parts := strings.Split(host, ":")
		hostname := parts[0]

		dotIndex := strings.Index(hostname, ".")
		if dotIndex > 0 {
			subdomain := hostname[:dotIndex]
			if subdomain != "" {
				c.Set(SubdomainAgentIDKey, subdomain)
			}
		}

		c.Next()
	}
}

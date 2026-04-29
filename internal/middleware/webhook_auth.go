package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebhookAuth returns a Gin middleware that validates the shared webhook
// secret.  The client must pass the header X-Webhook-Secret (or the query
// parameter ?secret=).  If secret is empty, the middleware rejects ALL
// requests — this is intentional so that deployments must explicitly
// configure a secret before the webhook endpoint becomes usable.
func WebhookAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    50001,
				"message": "webhook secret not configured on server",
			})
			return
		}

		provided := c.GetHeader("X-Webhook-Secret")
		if provided == "" {
			provided = c.Query("secret")
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    40001,
				"message": "invalid webhook secret",
			})
			return
		}

		c.Next()
	}
}

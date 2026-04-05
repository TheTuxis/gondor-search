package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	jwtpkg "github.com/TheTuxis/gondor-search/internal/pkg/jwt"
)

// skipPaths defines routes that don't require authentication.
var skipPaths = map[string]map[string]bool{
	"GET": {
		"/health":  true,
		"/metrics": true,
	},
}

func AuthMiddleware(jwtManager *jwtpkg.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this path should skip auth
		if paths, ok := skipPaths[c.Request.Method]; ok {
			if paths[c.Request.URL.Path] {
				c.Next()
				return
			}
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing authorization header",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid authorization header format",
			})
			return
		}

		claims, err := jwtManager.ValidateToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid or expired token",
			})
			return
		}

		// Set user info in context
		userID, _ := strconv.ParseUint(claims.Subject, 10, 64)
		c.Set("user_id", uint(userID))
		c.Set("email", claims.Email)
		c.Set("company_id", claims.CompanyID)
		c.Set("is_superuser", claims.IsSuperuser)

		c.Next()
	}
}

package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdaptHTTPMiddleware wraps a net/http middleware for use in Gin.
func AdaptHTTPMiddleware(middleware func(http.Handler) http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed := false
		request := c.Request

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed = true
			request = r
		})

		middleware(next).ServeHTTP(c.Writer, c.Request)
		if !allowed {
			c.Abort()
			return
		}

		c.Request = request
		c.Next()
	}
}

// GinSecurityMiddleware applies the existing API-key/IP security checks to Gin routes.
func GinSecurityMiddleware() gin.HandlerFunc {
	return AdaptHTTPMiddleware(SecurityMiddleware)
}

// GinJWTMiddleware applies the existing JWT validation middleware to Gin routes.
func GinJWTMiddleware() gin.HandlerFunc {
	return AdaptHTTPMiddleware(JWTMiddleware)
}

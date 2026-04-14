package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetBusinessIDFromContext extracts numeric business ID for webhook handlers.
func GetBusinessIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	if c == nil {
		return uuid.Nil, false
	}

	if id, ok := c.Get("business_id"); ok {
		if businessID, parsed := toUUID(id); parsed {
			return businessID, true
		}
	}

	for _, raw := range []string{
		c.Param("businessCode"),
		c.Param("businessId"),
		c.Query("business_code"),
		c.Query("business_id"),
		c.GetHeader("X-Business-Code"),
		c.GetHeader("X-Business-ID"),
	} {
		if businessID := ResolveBusinessIdentifier(raw); businessID != uuid.Nil {
			return businessID, true
		}
	}

	return uuid.Nil, false
}

func toUUID(v interface{}) (uuid.UUID, bool) {
	switch t := v.(type) {
	case uuid.UUID:
		return t, t != uuid.Nil
	case *uuid.UUID:
		if t != nil {
			return *t, *t != uuid.Nil
		}
	case string:
		if parsed, err := uuid.Parse(t); err == nil {
			return parsed, true
		}
	}

	return uuid.Nil, false
}

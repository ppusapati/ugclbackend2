package middleware

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetBusinessIDFromContext extracts numeric business ID for webhook handlers.
func GetBusinessIDFromContext(c *gin.Context) (uint, bool) {
	if c == nil {
		return 0, false
	}

	if id, ok := c.Get("business_id"); ok {
		if businessID, parsed := toUint(id); parsed {
			return businessID, true
		}
	}

	if raw := c.Param("businessId"); raw != "" {
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(parsed), true
		}
	}

	if raw := c.Query("business_id"); raw != "" {
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(parsed), true
		}
	}

	if raw := c.GetHeader("X-Business-ID"); raw != "" {
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(parsed), true
		}
	}

	return 0, false
}

func toUint(v interface{}) (uint, bool) {
	switch t := v.(type) {
	case uint:
		return t, true
	case uint8:
		return uint(t), true
	case uint16:
		return uint(t), true
	case uint32:
		return uint(t), true
	case uint64:
		return uint(t), true
	case int:
		if t >= 0 {
			return uint(t), true
		}
	case int8:
		if t >= 0 {
			return uint(t), true
		}
	case int16:
		if t >= 0 {
			return uint(t), true
		}
	case int32:
		if t >= 0 {
			return uint(t), true
		}
	case int64:
		if t >= 0 {
			return uint(t), true
		}
	case float64:
		if t >= 0 {
			return uint(t), true
		}
	case string:
		if parsed, err := strconv.ParseUint(t, 10, 64); err == nil {
			return uint(parsed), true
		}
	default:
		s := fmt.Sprint(v)
		if parsed, err := strconv.ParseUint(s, 10, 64); err == nil {
			return uint(parsed), true
		}
	}

	return 0, false
}

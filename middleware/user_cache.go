package middleware

import (
	"sync"
	"time"

	"p9e.in/ugcl/models"
)

// userCacheTTL is how long a cached user entry is considered fresh.
// Mutation handlers invalidate immediately, so a longer TTL avoids periodic cold auth bursts.
const userCacheTTL = 30 * time.Minute

type cachedUser struct {
	user      models.User
	expiresAt time.Time
}

// userCache is a process-level, thread-safe store of recently loaded users.
// It eliminates the repeated full Preload round-trips that happen on every request.
var userCache = &userContextCache{entries: make(map[string]cachedUser)}

type userContextCache struct {
	mu      sync.Mutex
	entries map[string]cachedUser
}

// get returns the cached user if present and not expired.
// Expired entries are deleted on detection to prevent unbounded map growth.
func (c *userContextCache) get(userID string) (models.User, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[userID]
	if !ok {
		return models.User{}, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, userID)
		return models.User{}, false
	}
	return entry.user, true
}

// set stores a user in the cache with the standard TTL.
func (c *userContextCache) set(userID string, user models.User) {
	c.mu.Lock()
	c.entries[userID] = cachedUser{
		user:      user,
		expiresAt: time.Now().Add(userCacheTTL),
	}
	c.mu.Unlock()
}

// invalidate removes a single user from the cache (call after role/permission changes).
func (c *userContextCache) invalidate(userID string) {
	c.mu.Lock()
	delete(c.entries, userID)
	c.mu.Unlock()
}

// InvalidateUserCache evicts a user so the next request re-fetches from DB.
// Call this from any handler that updates a user's role or permissions.
func InvalidateUserCache(userID string) {
	userCache.invalidate(userID)
}

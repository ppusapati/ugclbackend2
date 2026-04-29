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
	mu      sync.RWMutex
	entries map[string]cachedUser
}

// get returns the cached user if present and not expired.
// Uses RLock so concurrent reads from multiple goroutines do not serialise.
// Expired entries are lazily removed under a full write-lock.
func (c *userContextCache) get(userID string) (models.User, bool) {
	c.mu.RLock()
	entry, ok := c.entries[userID]
	c.mu.RUnlock()

	if !ok {
		return models.User{}, false
	}
	if time.Now().After(entry.expiresAt) {
		// Upgrade to write-lock to evict the stale entry.
		c.mu.Lock()
		// Re-check: another goroutine may have refreshed the entry between RUnlock and Lock.
		if e, still := c.entries[userID]; still && time.Now().After(e.expiresAt) {
			delete(c.entries, userID)
		}
		c.mu.Unlock()
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

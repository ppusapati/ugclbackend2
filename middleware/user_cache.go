package middleware

import (
	"container/list"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"p9e.in/ugcl/models"
)

// userCacheTTL is how long a cached user entry is considered fresh.
// Mutation handlers invalidate immediately, so a longer TTL avoids periodic cold auth bursts.
const userCacheTTL = 30 * time.Minute
const defaultUserCacheMaxEntries = 5000

type cachedUser struct {
	userID            string
	user              *models.User
	globalPermissions []string
	expiresAt         time.Time
}

// userCache is a process-level, thread-safe store of recently loaded users.
// It eliminates the repeated full Preload round-trips that happen on every request.
var userCache = newUserContextCache(loadUserCacheMaxEntries())

type userContextCache struct {
	mu         sync.Mutex
	maxEntries int
	ll         *list.List
	entries    map[string]*list.Element
}

func loadUserCacheMaxEntries() int {
	raw := strings.TrimSpace(os.Getenv("AUTH_USER_CACHE_MAX_ENTRIES"))
	if raw == "" {
		return defaultUserCacheMaxEntries
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultUserCacheMaxEntries
	}

	return value
}

func newUserContextCache(maxEntries int) *userContextCache {
	if maxEntries <= 0 {
		maxEntries = defaultUserCacheMaxEntries
	}

	return &userContextCache{
		maxEntries: maxEntries,
		ll:         list.New(),
		entries:    make(map[string]*list.Element, maxEntries),
	}
}

func (c *userContextCache) removeElement(elem *list.Element) {
	if elem == nil {
		return
	}
	entry := elem.Value.(cachedUser)
	delete(c.entries, entry.userID)
	c.ll.Remove(elem)
}

func (c *userContextCache) getEntry(userID string) (cachedUser, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[userID]
	if !ok {
		return cachedUser{}, false
	}

	entry := elem.Value.(cachedUser)
	if time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		return cachedUser{}, false
	}

	c.ll.MoveToFront(elem)
	return entry, true
}

// get returns the cached user if present and not expired.
// LRU ordering updates on cache hits, so this method always takes the write lock.
func (c *userContextCache) get(userID string) (*models.User, bool) {
	entry, ok := c.getEntry(userID)
	if !ok {
		return nil, false
	}
	return entry.user, true
}

// getAuthData returns cached user plus precomputed global permissions.
func (c *userContextCache) getAuthData(userID string) (*models.User, []string, bool) {
	entry, ok := c.getEntry(userID)
	if !ok {
		return nil, nil, false
	}

	perms := make([]string, len(entry.globalPermissions))
	copy(perms, entry.globalPermissions)
	return entry.user, perms, true
}

// set stores a user in the cache with the standard TTL.
func (c *userContextCache) set(userID string, user models.User) {
	c.mu.Lock()
	defer c.mu.Unlock()

	userCopy := user
	globalPermissions := make([]string, 0)
	if userCopy.RoleModel != nil {
		globalPermissions = make([]string, 0, len(userCopy.RoleModel.Permissions))
		for _, permission := range userCopy.RoleModel.Permissions {
			globalPermissions = append(globalPermissions, permission.Name)
		}
	}

	if elem, ok := c.entries[userID]; ok {
		elem.Value = cachedUser{
			userID:            userID,
			user:              &userCopy,
			globalPermissions: globalPermissions,
			expiresAt:         time.Now().Add(userCacheTTL),
		}
		c.ll.MoveToFront(elem)
		return
	}

	elem := c.ll.PushFront(cachedUser{
		userID:            userID,
		user:              &userCopy,
		globalPermissions: globalPermissions,
		expiresAt:         time.Now().Add(userCacheTTL),
	})
	c.entries[userID] = elem

	if c.ll.Len() > c.maxEntries {
		c.removeElement(c.ll.Back())
	}
}

// invalidate removes a single user from the cache (call after role/permission changes).
func (c *userContextCache) invalidate(userID string) {
	c.mu.Lock()
	if elem, ok := c.entries[userID]; ok {
		c.removeElement(elem)
	}
	c.mu.Unlock()
}

// InvalidateUserCache evicts a user so the next request re-fetches from DB.
// Call this from any handler that updates a user's role or permissions.
func InvalidateUserCache(userID string) {
	userCache.invalidate(userID)
}

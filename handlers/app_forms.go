package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/sync/singleflight"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/handlers/reports"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

const (
	formsListCacheTTL      = 10 * time.Minute
	formByCodeCacheTTL     = 30 * time.Second
	formsListStaleTTL      = 2 * time.Minute
	formByCodeStaleTTL     = 2 * time.Minute
	cacheHotThreshold      = 3
	cacheHotRetention      = 10 * time.Minute
	cacheStateHit          = "HIT"
	cacheStateMiss         = "MISS"
	cacheStateStale        = "STALE"
	cacheStateBypass       = "BYPASS"
	cacheLookupStateFresh  = "fresh"
	cacheLookupStateStale  = "stale"
	cacheLookupStateAbsent = "absent"
)

type cachedJSONResponse struct {
	payload    []byte
	expiresAt  time.Time
	staleUntil time.Time
	hitCount   int
	hotUntil   time.Time
}

var (
	formsListCache   = make(map[string]cachedJSONResponse)
	formsListCacheMu sync.RWMutex
	formsListVersion atomic.Uint64
	formsListLoad    singleflight.Group

	formByCodeCache   = make(map[string]cachedJSONResponse)
	formByCodeCacheMu sync.RWMutex
	formByCodeVersion atomic.Uint64
	formByCodeLoad    singleflight.Group
)

func init() {
	formsListVersion.Store(1)
	formByCodeVersion.Store(1)
}

func isPublicFormPermission(permission string) bool {
	switch strings.TrimSpace(permission) {
	case "", "*", "*:*:*":
		return true
	default:
		return false
	}
}

func getCachedJSONState(cache map[string]cachedJSONResponse, mu *sync.RWMutex, key string) ([]byte, string) {
	mu.Lock()
	defer mu.Unlock()

	entry, ok := cache[key]
	if !ok {
		return nil, cacheLookupStateAbsent
	}

	now := time.Now()
	if now.After(entry.staleUntil) {
		delete(cache, key)
		return nil, cacheLookupStateAbsent
	}

	entry.hitCount++
	if entry.hitCount >= cacheHotThreshold {
		entry.hotUntil = now.Add(cacheHotRetention)
	}
	cache[key] = entry

	if !now.After(entry.expiresAt) {
		return entry.payload, cacheLookupStateFresh
	}

	if !now.After(entry.hotUntil) {
		return entry.payload, cacheLookupStateStale
	}

	delete(cache, key)
	return nil, cacheLookupStateAbsent
}

func setCachedJSON(cache map[string]cachedJSONResponse, mu *sync.RWMutex, key string, payload []byte, ttl time.Duration, staleTTL time.Duration) {
	mu.Lock()
	now := time.Now()
	cache[key] = cachedJSONResponse{
		payload:    payload,
		expiresAt:  now.Add(ttl),
		staleUntil: now.Add(ttl + staleTTL),
		hitCount:   0,
		hotUntil:   time.Time{},
	}
	mu.Unlock()
}

func versionedFormsListCacheKey(rawKey string) string {
	return fmt.Sprintf("forms:list:v%d:%s", formsListVersion.Load(), rawKey)
}

func versionedFormByCodeCacheKey(rawKey string) string {
	return fmt.Sprintf("forms:by_code:v%d:%s", formByCodeVersion.Load(), rawKey)
}

func writeJSONBytes(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=30")
	_, _ = w.Write(payload)
}

func writeJSONBytesWithETag(w http.ResponseWriter, r *http.Request, payload []byte) {
	checksum := sha256.Sum256(payload)
	etag := fmt.Sprintf(`W/"%x"`, checksum)
	if matchesIfNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, max-age=30")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", etag)
	writeJSONBytes(w, payload)
}

func matchesIfNoneMatch(headerValue string, etag string) bool {
	trimmed := strings.TrimSpace(headerValue)
	if trimmed == "" {
		return false
	}

	for _, candidate := range strings.Split(trimmed, ",") {
		value := strings.TrimSpace(candidate)
		if value == "*" || value == etag {
			return true
		}
		if strings.HasPrefix(value, "W/") && strings.TrimPrefix(value, "W/") == etag {
			return true
		}
	}

	return false
}

// invalidateFormsCache clears all entries from the admin forms list cache and
// any per-vertical list caches so mutating operations are immediately visible.
func invalidateFormsCache() {
	formsListVersion.Add(1)
	formByCodeVersion.Add(1)

	formsListCacheMu.Lock()
	formsListCache = make(map[string]cachedJSONResponse)
	formsListCacheMu.Unlock()

	formByCodeCacheMu.Lock()
	formByCodeCache = make(map[string]cachedJSONResponse)
	formByCodeCacheMu.Unlock()
}

// GetFormsForVertical returns all forms accessible in a specific business vertical
// GET /api/v1/business/{vertical}/forms
func GetFormsForVertical(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	verticalCode := vars["businessCode"]

	if verticalCode == "" {
		http.Error(w, "vertical code is required", http.StatusBadRequest)
		return
	}

	isMobileClient := strings.Contains(r.Header.Get("User-Agent"), "Dart")
	normalizedVertical := strings.ToUpper(strings.TrimSpace(verticalCode))
	formsListCacheKey := versionedFormsListCacheKey(strings.Join([]string{claims.UserID, normalizedVertical, fmt.Sprintf("mobile:%t", isMobileClient)}, "|"))
	if payload, state := getCachedJSONState(formsListCache, &formsListCacheMu, formsListCacheKey); state == cacheLookupStateFresh {
		w.Header().Set("X-App-Forms-Cache", cacheStateHit)
		writeJSONBytesWithETag(w, r, payload)
		return
	} else if state == cacheLookupStateStale {
		w.Header().Set("X-App-Forms-Cache", cacheStateStale)
		writeJSONBytesWithETag(w, r, payload)
		return
	}
	w.Header().Set("X-App-Forms-Cache", cacheStateMiss)

	log.Printf("📋 Fetching forms for vertical: %s, user: %s", verticalCode, claims.UserID)

	// Reuse auth-service cache to avoid repeated role/permission graph queries.
	authService := middleware.NewAuthService()
	userCtx, err := authService.LoadUserContext(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	user := userCtx.User

	// Resolve business vertical so we can match both code and UUID in accessible_verticals.
	candidateTokens := map[string]struct{}{
		verticalCode:                  {},
		strings.ToUpper(verticalCode): {},
	}

	var matchedVerticals []models.BusinessVertical
	if err := config.DB.Where("LOWER(code) = LOWER(?)", verticalCode).Find(&matchedVerticals).Error; err != nil {
		log.Printf("⚠️ Failed to resolve business vertical %s: %v", verticalCode, err)
	}

	for _, v := range matchedVerticals {
		candidateTokens[v.Code] = struct{}{}
		candidateTokens[v.ID.String()] = struct{}{}
	}

	requestedVertical := strings.ToLower(strings.TrimSpace(verticalCode))

	// Mobile clients (Dart/Flutter) must never see inactive forms — users cannot
	// activate forms from mobile and inactive forms break the mobile UX.
	// Web super admins see ALL forms (active + inactive) so they can manage them.
	// Web regular users only see active forms.
	isSuperAdmin := user.HasPermission("admin_all") || user.HasPermission("super_admin") || user.HasPermission("*:*:*")
	filterInactive := isMobileClient || !isSuperAdmin

	// Get forms for this vertical using JSONB contains operator.
	// Include forms with empty accessible_verticals (globally accessible forms).
	var forms []models.AppForm
	filterArgs := make([]interface{}, 0, len(candidateTokens))
	arrayPlaceholders := make([]string, 0, len(candidateTokens))
	for token := range candidateTokens {
		if strings.TrimSpace(token) == "" {
			continue
		}
		arrayPlaceholders = append(arrayPlaceholders, "?")
		filterArgs = append(filterArgs, token)
	}

	filterCondition := "accessible_verticals = '[]'::jsonb"
	if len(arrayPlaceholders) > 0 {
		// JSONB ?| checks whether any candidate token exists in top-level array values.
		filterCondition = filterCondition + " OR accessible_verticals ?| ARRAY[" + strings.Join(arrayPlaceholders, ",") + "]"
	}

	query := config.DB.
		Select("id, code, title, description, module_id, route, icon, display_order, required_permission, accessible_verticals, is_active").
		Preload("Module").
		Where(filterCondition, filterArgs...).
		Order("display_order ASC, title ASC")

	if filterInactive {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Find(&forms).Error; err != nil {
		log.Printf("❌ Error fetching forms: %v", err)
		http.Error(w, "failed to fetch forms", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Found %d forms for vertical %s", len(forms), verticalCode)

	// Convert to DTOs and filter by user permissions
	formDTOs := make([]models.AppFormDTO, 0, len(forms))
	moduleMap := make(map[string][]models.AppFormDTO)

	// Build a set of vertical UUIDs matched for this request (used for business-role permission check).
	matchedVerticalIDSet := make(map[uuid.UUID]struct{}, len(matchedVerticals)+len(user.UserBusinessRoles))
	for _, v := range matchedVerticals {
		matchedVerticalIDSet[v.ID] = struct{}{}
	}

	// Fallback: derive vertical mapping from user's business roles when DB code lookup misses.
	for _, ubr := range user.UserBusinessRoles {
		if !ubr.IsActive || ubr.BusinessRole.ID == uuid.Nil {
			continue
		}

		roleVerticalID := strings.ToLower(strings.TrimSpace(ubr.BusinessRole.BusinessVerticalID.String()))
		roleVerticalCode := strings.ToLower(strings.TrimSpace(ubr.BusinessRole.BusinessVertical.Code))

		if requestedVertical == roleVerticalID || requestedVertical == roleVerticalCode {
			matchedVerticalIDSet[ubr.BusinessRole.BusinessVerticalID] = struct{}{}
			candidateTokens[ubr.BusinessRole.BusinessVerticalID.String()] = struct{}{}
			if strings.TrimSpace(ubr.BusinessRole.BusinessVertical.Code) != "" {
				candidateTokens[ubr.BusinessRole.BusinessVertical.Code] = struct{}{}
				candidateTokens[strings.ToUpper(ubr.BusinessRole.BusinessVertical.Code)] = struct{}{}
			}
		}
	}

	matchedVerticalIDs := make([]uuid.UUID, 0, len(matchedVerticalIDSet))
	for vid := range matchedVerticalIDSet {
		matchedVerticalIDs = append(matchedVerticalIDs, vid)
	}

	// userCanAccess returns true if the user holds the required permission via
	// their global role OR via any business role in one of the matched verticals.
	userCanAccess := func(permission string) bool {
		if user.HasPermission(permission) {
			return true
		}
		for _, vid := range matchedVerticalIDs {
			if user.HasPermissionInVertical(permission, vid) {
				return true
			}
		}
		return false
	}

	for _, form := range forms {
		// Check if user has required permission (global role OR business role in this vertical)
		if !isPublicFormPermission(form.RequiredPermission) && !userCanAccess(form.RequiredPermission) {
			log.Printf("   ⊘ Skipping form %s - user lacks permission %s", form.Code, form.RequiredPermission)
			continue
		}

		dto := form.ToDTO()
		formDTOs = append(formDTOs, dto)

		// Group by module
		moduleCode := dto.Module
		if moduleCode != "" {
			moduleMap[moduleCode] = append(moduleMap[moduleCode], dto)
		}
	}

	log.Printf("✅ Returning %d forms after permission filtering", len(formDTOs))

	response := map[string]interface{}{
		"forms":   formDTOs,
		"modules": moduleMap,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to encode forms response", http.StatusInternalServerError)
		return
	}

	loaded, loadErr, _ := formsListLoad.Do(formsListCacheKey, func() (interface{}, error) {
		setCachedJSON(formsListCache, &formsListCacheMu, formsListCacheKey, payload, formsListCacheTTL, formsListStaleTTL)
		return payload, nil
	})
	if loadErr != nil {
		http.Error(w, "failed to cache forms response", http.StatusInternalServerError)
		return
	}
	writeJSONBytes(w, loaded.([]byte))
}

// GetFormByCode returns a specific form by its code with full schema
// GET /api/v1/business/{vertical}/forms/{code}
func GetFormByCode(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	verticalCode := vars["businessCode"]
	formCode := vars["code"]

	formByCodeCacheKey := versionedFormByCodeCacheKey(strings.Join([]string{claims.UserID, strings.ToUpper(strings.TrimSpace(verticalCode)), strings.TrimSpace(formCode)}, "|"))
	if payload, state := getCachedJSONState(formByCodeCache, &formByCodeCacheMu, formByCodeCacheKey); state == cacheLookupStateFresh {
		w.Header().Set("X-App-Form-Cache", cacheStateHit)
		writeJSONBytesWithETag(w, r, payload)
		return
	} else if state == cacheLookupStateStale {
		w.Header().Set("X-App-Form-Cache", cacheStateStale)
		writeJSONBytesWithETag(w, r, payload)
		return
	}
	w.Header().Set("X-App-Form-Cache", cacheStateMiss)

	// if verticalCode == "" || formCode == "" {
	// 	http.Error(w, "vertical code and form code are required", http.StatusBadRequest)
	// 	return
	// }

	log.Printf("📋 Fetching form: %s for vertical: %s", formCode, verticalCode)

	// Reuse auth-service cache to avoid repeated role/permission graph queries.
	authService := middleware.NewAuthService()
	userCtx, err := authService.LoadUserContext(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	user := userCtx.User

	// Get the form
	var form models.AppForm
	if err := config.DB.
		Preload("Module").
		Where("code = ? AND is_active = ?", formCode, true).
		// Where("accessible_verticals @> ?", `["`+verticalCode+`"]`).
		First(&form).Error; err != nil {
		log.Printf("❌ Form not found: %s", formCode)
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	// Check permission — allow via global role OR any business role in this vertical
	if !isPublicFormPermission(form.RequiredPermission) {
		var verticalForForm []models.BusinessVertical
		_ = config.DB.Where("LOWER(code) = LOWER(?)", verticalCode).Find(&verticalForForm)
		requestedVertical := strings.ToLower(strings.TrimSpace(verticalCode))
		verticalIDSet := make(map[uuid.UUID]struct{}, len(verticalForForm)+len(user.UserBusinessRoles))
		for _, v := range verticalForForm {
			verticalIDSet[v.ID] = struct{}{}
		}
		for _, ubr := range user.UserBusinessRoles {
			if !ubr.IsActive || ubr.BusinessRole.ID == uuid.Nil {
				continue
			}
			roleVerticalID := strings.ToLower(strings.TrimSpace(ubr.BusinessRole.BusinessVerticalID.String()))
			roleVerticalCode := strings.ToLower(strings.TrimSpace(ubr.BusinessRole.BusinessVertical.Code))
			if requestedVertical == roleVerticalID || requestedVertical == roleVerticalCode {
				verticalIDSet[ubr.BusinessRole.BusinessVerticalID] = struct{}{}
			}
		}

		hasAccess := user.HasPermission(form.RequiredPermission)
		if !hasAccess {
			for vid := range verticalIDSet {
				if user.HasPermissionInVertical(form.RequiredPermission, vid) {
					hasAccess = true
					break
				}
			}
		}
		if !hasAccess {
			log.Printf("❌ User lacks permission %s for form %s", form.RequiredPermission, formCode)
			http.Error(w, "forbidden - insufficient permissions", http.StatusForbidden)
			return
		}
	}

	// Return full form with schema
	response := form.ToDTOWithSchema()
	rewriteAbsoluteDropdownEndpoints(response, verticalCode)
	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to encode form response", http.StatusInternalServerError)
		return
	}

	loaded, loadErr, _ := formByCodeLoad.Do(formByCodeCacheKey, func() (interface{}, error) {
		setCachedJSON(formByCodeCache, &formByCodeCacheMu, formByCodeCacheKey, payload, formByCodeCacheTTL, formByCodeStaleTTL)
		return payload, nil
	})
	if loadErr != nil {
		http.Error(w, "failed to cache form response", http.StatusInternalServerError)
		return
	}
	writeJSONBytes(w, loaded.([]byte))
}

func rewriteAbsoluteDropdownEndpoints(node interface{}, businessCode string) {
	switch typed := node.(type) {
	case map[string]interface{}:
		for key, value := range typed {
			if key == "apiEndpoint" {
				if endpoint, ok := value.(string); ok {
					if rewritten, ok := buildSafeDropdownProxyEndpoint(endpoint, businessCode); ok {
						typed[key] = rewritten
					}
				}
				continue
			}
			rewriteAbsoluteDropdownEndpoints(value, businessCode)
		}
	case []interface{}:
		for _, item := range typed {
			rewriteAbsoluteDropdownEndpoints(item, businessCode)
		}
	}
}

func buildSafeDropdownProxyEndpoint(rawEndpoint, businessCode string) (string, bool) {
	endpoint := strings.TrimSpace(rawEndpoint)
	if endpoint == "" {
		return "", false
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return "", false
	}

	return fmt.Sprintf("business/%s/integrations/external-dropdown?target=%s", url.PathEscape(strings.TrimSpace(businessCode)), url.QueryEscape(endpoint)), true
}

// GetAllForms returns all forms in the system (admin only)
// GET /api/v1/admin/forms
func GetAllAppForms(w http.ResponseWriter, r *http.Request) {
	if middleware.GetClaims(r) == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	allAdminFormsCacheKey := versionedFormsListCacheKey("admin:all")
	if payload, state := getCachedJSONState(formsListCache, &formsListCacheMu, allAdminFormsCacheKey); state == cacheLookupStateFresh {
		writeJSONBytesWithETag(w, r, payload)
		return
	} else if state == cacheLookupStateStale {
		writeJSONBytesWithETag(w, r, payload)
		return
	}

	var forms []models.AppForm
	if err := config.DB.
		Preload("Module").
		Order("module_id ASC, display_order ASC").
		Find(&forms).Error; err != nil {
		http.Error(w, "failed to fetch forms", http.StatusInternalServerError)
		return
	}

	formDTOs := make([]models.AppFormDTO, 0, len(forms))
	for _, form := range forms {
		formDTOs = append(formDTOs, form.ToDTO())
	}

	sort.SliceStable(formDTOs, func(i, j int) bool {
		if formDTOs[i].Module == formDTOs[j].Module {
			if formDTOs[i].DisplayOrder == formDTOs[j].DisplayOrder {
				return formDTOs[i].Title < formDTOs[j].Title
			}
			return formDTOs[i].DisplayOrder < formDTOs[j].DisplayOrder
		}
		return formDTOs[i].Module < formDTOs[j].Module
	})

	w.Header().Set("Content-Type", "application/json")
	if payload, err := json.Marshal(map[string]interface{}{
		"forms": formDTOs,
		"count": len(formDTOs),
	}); err == nil {
		loaded, loadErr, _ := formsListLoad.Do(allAdminFormsCacheKey, func() (interface{}, error) {
			setCachedJSON(formsListCache, &formsListCacheMu, allAdminFormsCacheKey, payload, formsListCacheTTL, formsListStaleTTL)
			return payload, nil
		})
		if loadErr != nil {
			http.Error(w, "failed to cache forms response", http.StatusInternalServerError)
			return
		}
		writeJSONBytesWithETag(w, r, loaded.([]byte))
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"forms": formDTOs,
			"count": len(formDTOs),
		})
	}
}

func deriveReadPermission(permission string) string {
	trimmed := strings.TrimSpace(permission)
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return trimmed
	}
	return parts[0] + ":read"
}

func normalizeLookupFieldValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		if name, ok := typed["name"]; ok && name != nil {
			return fmt.Sprint(name)
		}
		if id, ok := typed["id"]; ok && id != nil {
			return fmt.Sprint(id)
		}
		return typed
	case []interface{}:
		flattened := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			flattened = append(flattened, normalizeLookupFieldValue(item))
		}
		return flattened
	default:
		return value
	}
}

// GetFormLookupOptions exposes flattened dedicated-form submissions as dropdown options.
// GET /api/v1/business/{businessCode}/forms/{formCode}/lookup
func GetFormLookupOptions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	verticalCode := vars["businessCode"]
	formCode := vars["formCode"]
	if strings.TrimSpace(verticalCode) == "" || strings.TrimSpace(formCode) == "" {
		http.Error(w, "business code and form code are required", http.StatusBadRequest)
		return
	}

	context := middleware.GetUserBusinessContext(r)
	if context == nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	businessID, ok := context["business_id"].(uuid.UUID)
	if !ok {
		http.Error(w, "invalid business context", http.StatusInternalServerError)
		return
	}

	authService := middleware.NewAuthService()
	userCtx, err := authService.LoadUserContext(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	user := userCtx.User

	var form models.AppForm
	if err := config.DB.Preload("Module").Where("code = ? AND is_active = ?", formCode, true).First(&form).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	if !isPublicFormPermission(form.RequiredPermission) {
		var verticals []models.BusinessVertical
		_ = config.DB.Where("LOWER(code) = LOWER(?)", verticalCode).Find(&verticals)

		requestedVertical := strings.ToLower(strings.TrimSpace(verticalCode))
		verticalIDSet := make(map[uuid.UUID]struct{}, len(verticals)+len(user.UserBusinessRoles))
		for _, v := range verticals {
			verticalIDSet[v.ID] = struct{}{}
		}
		for _, ubr := range user.UserBusinessRoles {
			if !ubr.IsActive || ubr.BusinessRole.ID == uuid.Nil {
				continue
			}
			roleVerticalID := strings.ToLower(strings.TrimSpace(ubr.BusinessRole.BusinessVerticalID.String()))
			roleVerticalCode := strings.ToLower(strings.TrimSpace(ubr.BusinessRole.BusinessVertical.Code))
			if requestedVertical == roleVerticalID || requestedVertical == roleVerticalCode {
				verticalIDSet[ubr.BusinessRole.BusinessVerticalID] = struct{}{}
			}
		}

		fallbackReadPermission := deriveReadPermission(form.RequiredPermission)
		hasAccess := user.HasPermission(form.RequiredPermission) || user.HasPermission(fallbackReadPermission)
		if !hasAccess {
			for vid := range verticalIDSet {
				if user.HasPermissionInVertical(form.RequiredPermission, vid) || user.HasPermissionInVertical(fallbackReadPermission, vid) {
					hasAccess = true
					break
				}
			}
		}
		if !hasAccess {
			http.Error(w, "forbidden - insufficient permissions", http.StatusForbidden)
			return
		}
	}

	filters := make(map[string]interface{})
	if state := strings.TrimSpace(r.URL.Query().Get("state")); state != "" {
		filters["current_state"] = state
	}

	records, err := getWorkflowEngineDedicated().GetSubmissionsByFormDedicated(formCode, businessID, filters)
	if err != nil {
		log.Printf("❌ Error fetching lookup options for form %s: %v", formCode, err)
		http.Error(w, "failed to fetch lookup options", http.StatusInternalServerError)
		return
	}

	labelField := strings.TrimSpace(r.URL.Query().Get("label_field"))
	options := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		item := map[string]interface{}{
			"id":            record.ID.String(),
			"value":         record.ID.String(),
			"current_state": record.CurrentState,
		}
		for key, value := range record.FormData {
			item[key] = normalizeLookupFieldValue(value)
		}
		if labelField != "" {
			if value, exists := item[labelField]; exists {
				item["label"] = fmt.Sprint(value)
			}
		}
		options = append(options, item)
	}

	if labelField != "" {
		sort.SliceStable(options, func(i, j int) bool {
			return fmt.Sprint(options[i]["label"]) < fmt.Sprint(options[j]["label"])
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  options,
		"count": len(options),
	})
}

// UpdateFormVerticalAccess updates which verticals have access to a form (admin only)
// POST /api/v1/admin/forms/{formCode}/verticals
func UpdateFormVerticalAccess(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	var requestBody struct {
		VerticalCodes []string `json:"vertical_codes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get the form
	var form models.AppForm
	if err := config.DB.Where("code = ?", formCode).First(&form).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	// Update accessible verticals
	form.AccessibleVerticals = requestBody.VerticalCodes
	if err := config.DB.Save(&form).Error; err != nil {
		log.Printf("❌ Error updating form: %v", err)
		http.Error(w, "failed to update form", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated form %s vertical access to: %v", formCode, requestBody.VerticalCodes)
	invalidateFormsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":          "form vertical access updated successfully",
		"form":             formCode,
		"vertical_codes":   requestBody.VerticalCodes,
		"accessible_count": len(requestBody.VerticalCodes),
	})
}

// CreateForm creates a new form (admin only)
// POST /api/v1/admin/forms
func CreateForm(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Println(err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Error serializing request body: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var form models.AppForm
	if err := json.Unmarshal(bodyBytes, &form); err != nil {
		log.Printf("❌ Error parsing request body into form: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if _, hasIsActive := payload["is_active"]; !hasIsActive {
		form.IsActive = true
	}

	form.CreatedBy = claims.UserID

	// Get the module to retrieve its schema name
	var module models.Module
	if err := config.DB.First(&module, "id = ?", form.ModuleID).Error; err != nil {
		log.Printf("❌ Module not found for form %s: %v", form.Code, err)
		http.Error(w, "module not found", http.StatusBadRequest)
		return
	}

	// Generate table name if not provided
	if form.DBTableName == "" {
		// Generate table name from form code (sanitized)
		form.DBTableName = generateTableName(form.Code)
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("❌ Error starting transaction for form create: %v", tx.Error)
		http.Error(w, "failed to create form", http.StatusInternalServerError)
		return
	}

	// Create form record in database first
	if err := tx.Create(&form).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Error creating form: %v", err)
		http.Error(w, "failed to create form", http.StatusInternalServerError)
		return
	}

	if form.IsActive {
		if _, err := reports.EnsureReportFormViewForForm(tx, form); err != nil {
			tx.Rollback()
			log.Printf("❌ Error creating report view for form %s: %v", form.Code, err)
			http.Error(w, "failed to create reporting view for form", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Error committing form create transaction: %v", err)
		http.Error(w, "failed to create form", http.StatusInternalServerError)
		return
	}

	// Create dedicated table for the form in the module's schema
	var schemaName string
	var tableCreated bool
	if module.SchemaName != "" {
		formTableManager := NewFormTableManager()
		if err := formTableManager.CreateFormTableInSchema(&form, module.SchemaName); err != nil {
			log.Printf("⚠️  Warning: Failed to create dedicated table for form %s in schema %s: %v", form.Code, module.SchemaName, err)
			// Don't fail the request - the form is created, table creation is optional
		} else {
			schemaName = module.SchemaName
			tableCreated = true
			log.Printf("✅ Created dedicated table %s.%s for form %s", module.SchemaName, form.DBTableName, form.Code)
		}
	}

	log.Printf("✅ Created new form: %s", form.Code)
	invalidateFormsCache()

	response := map[string]interface{}{
		"message": "form created successfully",
		"form":    form.ToDTO(),
	}

	if tableCreated {
		response["schema_name"] = schemaName
		response["table_name"] = form.DBTableName
		response["full_table_name"] = fmt.Sprintf("%s.%s", schemaName, form.DBTableName)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// generateTableName generates a valid PostgreSQL table name from form code
func generateTableName(formCode string) string {
	// Convert to lowercase
	name := strings.ToLower(formCode)

	// Replace spaces and hyphens with underscores
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Remove any characters that are not letters, digits, or underscores
	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			result.WriteRune(c)
		}
	}
	name = result.String()

	// Ensure it starts with a letter or underscore (prefix with underscore if starts with digit)
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	// Limit length (PostgreSQL identifier limit is 63 bytes)
	if len(name) > 63 {
		name = name[:63]
	}

	return name
}

// ToggleFormStatus activates or deactivates a form (admin only)
// PATCH /api/v1/admin/app-forms/{formCode}/status
func ToggleFormStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var form models.AppForm
	if err := config.DB.Where("code = ?", formCode).First(&form).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("❌ Error starting transaction for form status update: %v", tx.Error)
		http.Error(w, "failed to update form status", http.StatusInternalServerError)
		return
	}

	form.IsActive = body.IsActive
	if body.IsActive {
		if _, err := reports.EnsureReportFormViewForForm(tx, form); err != nil {
			tx.Rollback()
			log.Printf("❌ Error creating report view while activating form %s: %v", form.Code, err)
			http.Error(w, "failed to activate form reporting view", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Save(&form).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Error updating form status: %v", err)
		http.Error(w, "failed to update form status", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Error committing form status transaction: %v", err)
		http.Error(w, "failed to update form status", http.StatusInternalServerError)
		return
	}

	status := "inactive"
	if body.IsActive {
		status = "active"
	}
	log.Printf("✅ Form %s marked %s by %s", formCode, status, claims.UserID)
	invalidateFormsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "form status updated successfully",
		"form_code": formCode,
		"is_active": body.IsActive,
	})
}

// UpdateForm updates an existing form (admin only)
// PUT /api/v1/admin/app-forms/{formCode}
func UpdateForm(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	// Get existing form
	var existingForm models.AppForm
	if err := config.DB.Where("code = ?", formCode).First(&existingForm).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	// Parse update request — read raw bytes first to detect explicit is_active
	var payload map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("❌ Error decoding update request for form %s: %v", formCode, err)
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	bodyBytes, _ := json.Marshal(payload)
	var updateData models.AppForm
	if err := json.Unmarshal(bodyBytes, &updateData); err != nil {
		log.Printf("❌ Error parsing update body for form %s: %v", formCode, err)
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("📝 Updating form: %s, title=%s, description=%s", formCode, updateData.Title, updateData.Description)

	// Update allowed fields
	if updateData.Title != "" {
		existingForm.Title = updateData.Title
	}
	if updateData.Description != "" {
		existingForm.Description = updateData.Description
	}
	if updateData.ModuleID != uuid.Nil {
		existingForm.ModuleID = updateData.ModuleID
	}
	if len(updateData.FormSchema) > 0 {
		existingForm.FormSchema = updateData.FormSchema
	}
	if len(updateData.Steps) > 0 {
		existingForm.Steps = updateData.Steps
	}
	if len(updateData.CoreFields) > 0 {
		existingForm.CoreFields = updateData.CoreFields
	}
	if len(updateData.Validations) > 0 {
		existingForm.Validations = updateData.Validations
	}
	if len(updateData.Dependencies) > 0 {
		existingForm.Dependencies = updateData.Dependencies
	}
	if updateData.WorkflowID != nil {
		existingForm.WorkflowID = updateData.WorkflowID
	}
	if updateData.InitialState != "" {
		existingForm.InitialState = updateData.InitialState
	}
	if updateData.RequiredPermission != "" {
		existingForm.RequiredPermission = updateData.RequiredPermission
	}
	if updateData.DisplayOrder > 0 {
		existingForm.DisplayOrder = updateData.DisplayOrder
	}
	if len(updateData.AccessibleVerticals) > 0 {
		existingForm.AccessibleVerticals = updateData.AccessibleVerticals
	}
	if updateData.DBTableName != "" {
		existingForm.DBTableName = updateData.DBTableName
	}
	// Honour explicit is_active when sent in payload
	if _, hasIsActive := payload["is_active"]; hasIsActive {
		existingForm.IsActive = updateData.IsActive
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("❌ Error starting transaction for form update: %v", tx.Error)
		http.Error(w, "failed to update form", http.StatusInternalServerError)
		return
	}

	// Save updates
	if err := tx.Save(&existingForm).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Error updating form: %v", err)
		http.Error(w, "failed to update form", http.StatusInternalServerError)
		return
	}

	if existingForm.IsActive {
		if _, err := reports.EnsureReportFormViewForForm(tx, existingForm); err != nil {
			tx.Rollback()
			log.Printf("❌ Error syncing report view for form %s: %v", existingForm.Code, err)
			http.Error(w, "failed to sync reporting view for form", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Error committing form update transaction: %v", err)
		http.Error(w, "failed to update form", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated form: %s", formCode)
	invalidateFormsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "form updated successfully",
		"form":    existingForm.ToDTO(),
	})
}

// DeleteForm permanently deletes a form (admin only)
// DELETE /api/v1/admin/app-forms/{formCode}
func DeleteForm(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	var form models.AppForm
	if err := config.DB.Where("code = ?", formCode).First(&form).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	if err := config.DB.Delete(&form).Error; err != nil {
		log.Printf("❌ Error deleting form %s: %v", formCode, err)
		http.Error(w, "failed to delete form", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Deleted form: %s by %s", formCode, claims.UserID)
	invalidateFormsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "form deleted successfully",
		"form_code": formCode,
	})
}

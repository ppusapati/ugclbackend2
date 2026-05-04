package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// ─── request / response DTOs ────────────────────────────────────────────────

type createIntegrationRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Provider     string   `json:"provider"`
	EndpointURL  string   `json:"endpoint_url"`
	Model        string   `json:"model"`
	AuthHeader   string   `json:"auth_header"`
	AuthScheme   string   `json:"auth_scheme"`
	Secret       string   `json:"secret"`
	AllowedURLs  []string `json:"allowed_urls"`
	AllowedIPs   []string `json:"allowed_ips"`
	DataScopes   []string `json:"data_scopes"`
	ContactEmail string   `json:"contact_email"`
}

type updateIntegrationRequest struct {
	Name         *string  `json:"name"`
	Description  *string  `json:"description"`
	Status       *string  `json:"status"`
	Provider     *string  `json:"provider"`
	EndpointURL  *string  `json:"endpoint_url"`
	Model        *string  `json:"model"`
	AuthHeader   *string  `json:"auth_header"`
	AuthScheme   *string  `json:"auth_scheme"`
	Secret       *string  `json:"secret"`
	AllowedURLs  []string `json:"allowed_urls"`
	AllowedIPs   []string `json:"allowed_ips"`
	DataScopes   []string `json:"data_scopes"`
	ContactEmail *string  `json:"contact_email"`
}

type integrationResponse struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Status       string     `json:"status"`
	Provider     string     `json:"provider,omitempty"`
	EndpointURL  string     `json:"endpoint_url,omitempty"`
	Model        string     `json:"model,omitempty"`
	AuthHeader   string     `json:"auth_header,omitempty"`
	AuthScheme   string     `json:"auth_scheme,omitempty"`
	HasSecret    bool       `json:"has_secret"`
	APIKeyPrefix string     `json:"api_key_prefix"`
	APIKey       string     `json:"api_key,omitempty"` // only on create / regenerate
	AllowedURLs  []string   `json:"allowed_urls"`
	AllowedIPs   []string   `json:"allowed_ips"`
	DataScopes   []string   `json:"data_scopes"`
	ContactEmail string     `json:"contact_email,omitempty"`
	CreatedBy    uuid.UUID  `json:"created_by,omitempty"`
	LastAccessAt *time.Time `json:"last_accessed_at,omitempty"`
	AccessCount  int64      `json:"access_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

var allowedIntegrationScopes = map[string]struct{}{
	models.IntegrationScopePartnerDPRSiteRead:    {},
	models.IntegrationScopePartnerWrappingRead:   {},
	models.IntegrationScopePartnerEWayRead:       {},
	models.IntegrationScopePartnerWaterRead:      {},
	models.IntegrationScopePartnerStockRead:      {},
	models.IntegrationScopePartnerDairySiteRead:  {},
	models.IntegrationScopePartnerPaymentRead:    {},
	models.IntegrationScopePartnerMaterialRead:   {},
	models.IntegrationScopePartnerMNRRead:        {},
	models.IntegrationScopePartnerNMRVehicleRead: {},
	models.IntegrationScopePartnerContractorRead: {},
	models.IntegrationScopePartnerPaintingRead:   {},
	models.IntegrationScopePartnerDieselRead:     {},
	models.IntegrationScopePartnerTasksRead:      {},
	models.IntegrationScopePartnerVehicleLogRead: {},
	models.IntegrationScopeDropdownProxyUse:      {},
	models.IntegrationScopeDocumentAIUse:         {},
}

const integrationAPIKeyBcryptCost = 12

// ─── helpers ────────────────────────────────────────────────────────────────

func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ugcl_" + hex.EncodeToString(b), nil
}

func hashAPIKey(key string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(key), integrationAPIKeyBcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func integrationToResponse(m models.ThirdPartyIntegration, plainKey string) integrationResponse {
	return integrationResponse{
		ID:           m.ID,
		Name:         m.Name,
		Description:  m.Description,
		Status:       string(m.Status),
		Provider:     m.Provider,
		EndpointURL:  m.EndpointURL,
		Model:        m.Model,
		AuthHeader:   m.AuthHeader,
		AuthScheme:   m.AuthScheme,
		HasSecret:    strings.TrimSpace(m.SecretCipher) != "",
		APIKeyPrefix: m.APIKeyPrefix,
		APIKey:       plainKey,
		AllowedURLs:  []string(m.AllowedURLs),
		AllowedIPs:   []string(m.AllowedIPs),
		DataScopes:   []string(m.DataScopes),
		ContactEmail: m.ContactEmail,
		CreatedBy:    m.CreatedBy,
		LastAccessAt: m.LastAccessAt,
		AccessCount:  m.AccessCount,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func parseUUIDParam(r *http.Request, param string) (uuid.UUID, error) {
	return uuid.Parse(mux.Vars(r)[param])
}

func normalizeIntegrationURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return "", http.ErrMissingFile
	}
	parsed.User = nil
	parsed.Fragment = ""
	parsed.RawQuery = ""
	parsed.Path = strings.TrimRight(parsed.EscapedPath(), "/")
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func normalizeAndValidateURLs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		value, err := normalizeIntegrationURL(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeAndValidateIPs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			if _, _, err := net.ParseCIDR(value); err != nil {
				return nil, err
			}
		} else if net.ParseIP(value) == nil {
			return nil, http.ErrNotSupported
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeAndValidateScopes(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := allowedIntegrationScopes[value]; !ok {
			return nil, http.ErrNotSupported
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

// ─── CRUD handlers ──────────────────────────────────────────────────────────

// ListIntegrations  GET /api/v1/admin/integrations
func ListIntegrations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var items []models.ThirdPartyIntegration
	if err := config.DB.Order("created_at DESC").Find(&items).Error; err != nil {
		http.Error(w, "failed to list integrations", http.StatusInternalServerError)
		return
	}

	resp := make([]integrationResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, integrationToResponse(it, ""))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"integrations": resp,
		"total":        len(resp),
	})
}

// GetIntegration  GET /api/v1/admin/integrations/{id}
func GetIntegration(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid integration id", http.StatusBadRequest)
		return
	}

	var item models.ThirdPartyIntegration
	if err := config.DB.First(&item, "id = ?", id).Error; err != nil {
		http.Error(w, "integration not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(integrationToResponse(item, ""))
}

// CreateIntegration  POST /api/v1/admin/integrations
func CreateIntegration(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	plainKey, err := generateAPIKey()
	if err != nil {
		http.Error(w, "failed to generate api key", http.StatusInternalServerError)
		return
	}
	allowedURLs, err := normalizeAndValidateURLs(req.AllowedURLs)
	if err != nil {
		http.Error(w, "allowed_urls contains an invalid absolute URL", http.StatusBadRequest)
		return
	}
	allowedIPs, err := normalizeAndValidateIPs(req.AllowedIPs)
	if err != nil {
		http.Error(w, "allowed_ips contains an invalid IP or CIDR", http.StatusBadRequest)
		return
	}
	dataScopes, err := normalizeAndValidateScopes(req.DataScopes)
	if err != nil {
		http.Error(w, "data_scopes contains an unsupported scope", http.StatusBadRequest)
		return
	}
	keyHash, err := hashAPIKey(plainKey)
	if err != nil {
		http.Error(w, "failed to hash api key", http.StatusInternalServerError)
		return
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	endpointURL := strings.TrimSpace(req.EndpointURL)
	model := strings.TrimSpace(req.Model)
	authHeader := strings.TrimSpace(req.AuthHeader)
	authScheme := strings.TrimSpace(req.AuthScheme)

	if provider != "" && endpointURL == "" {
		http.Error(w, "endpoint_url is required when provider is set", http.StatusBadRequest)
		return
	}
	if endpointURL != "" {
		if _, err := normalizeIntegrationURL(endpointURL); err != nil {
			http.Error(w, "endpoint_url must be a valid absolute URL", http.StatusBadRequest)
			return
		}
	}
	if authHeader == "" {
		authHeader = "Authorization"
	}
	if authScheme == "" {
		authScheme = "Bearer"
	}

	secretCipher := ""
	if strings.TrimSpace(req.Secret) != "" {
		secretCipher, err = encryptIntegrationSecret(strings.TrimSpace(req.Secret))
		if err != nil {
			http.Error(w, "failed to encrypt integration secret", http.StatusInternalServerError)
			return
		}
	}

	creatorID, _ := uuid.Parse(claims.UserID)

	item := models.ThirdPartyIntegration{
		ID:           uuid.New(),
		Name:         name,
		Description:  strings.TrimSpace(req.Description),
		Status:       models.IntegrationStatusActive,
		Provider:     provider,
		EndpointURL:  endpointURL,
		Model:        model,
		AuthHeader:   authHeader,
		AuthScheme:   authScheme,
		SecretCipher: secretCipher,
		APIKeyHash:   keyHash,
		APIKeyPrefix: plainKey[:12],
		AllowedURLs:  datatypes.JSONSlice[string](allowedURLs),
		AllowedIPs:   datatypes.JSONSlice[string](allowedIPs),
		DataScopes:   datatypes.JSONSlice[string](dataScopes),
		ContactEmail: strings.TrimSpace(req.ContactEmail),
		CreatedBy:    creatorID,
	}

	if err := config.DB.Create(&item).Error; err != nil {
		http.Error(w, "failed to create integration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(integrationToResponse(item, plainKey))
}

// UpdateIntegration  PATCH /api/v1/admin/integrations/{id}
func UpdateIntegration(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid integration id", http.StatusBadRequest)
		return
	}

	var item models.ThirdPartyIntegration
	if err := config.DB.First(&item, "id = ?", id).Error; err != nil {
		http.Error(w, "integration not found", http.StatusNotFound)
		return
	}

	var req updateIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		if n := strings.TrimSpace(*req.Name); n != "" {
			item.Name = n
		}
	}
	if req.Description != nil {
		item.Description = strings.TrimSpace(*req.Description)
	}
	if req.ContactEmail != nil {
		item.ContactEmail = strings.TrimSpace(*req.ContactEmail)
	}
	if req.Status != nil {
		s := models.IntegrationStatus(strings.TrimSpace(*req.Status))
		switch s {
		case models.IntegrationStatusActive, models.IntegrationStatusInactive, models.IntegrationStatusSuspended:
			item.Status = s
		default:
			http.Error(w, "invalid status value", http.StatusBadRequest)
			return
		}
	}
	if req.Provider != nil {
		item.Provider = strings.ToLower(strings.TrimSpace(*req.Provider))
	}
	if req.EndpointURL != nil {
		endpointURL := strings.TrimSpace(*req.EndpointURL)
		if endpointURL != "" {
			if _, err := normalizeIntegrationURL(endpointURL); err != nil {
				http.Error(w, "endpoint_url must be a valid absolute URL", http.StatusBadRequest)
				return
			}
		}
		item.EndpointURL = endpointURL
	}
	if req.Model != nil {
		item.Model = strings.TrimSpace(*req.Model)
	}
	if req.AuthHeader != nil {
		header := strings.TrimSpace(*req.AuthHeader)
		if header == "" {
			header = "Authorization"
		}
		item.AuthHeader = header
	}
	if req.AuthScheme != nil {
		scheme := strings.TrimSpace(*req.AuthScheme)
		if scheme == "" {
			scheme = "Bearer"
		}
		item.AuthScheme = scheme
	}
	if req.Secret != nil {
		secret := strings.TrimSpace(*req.Secret)
		if secret == "" {
			item.SecretCipher = ""
		} else {
			cipherText, err := encryptIntegrationSecret(secret)
			if err != nil {
				http.Error(w, "failed to encrypt integration secret", http.StatusInternalServerError)
				return
			}
			item.SecretCipher = cipherText
		}
	}
	if req.AllowedURLs != nil {
		allowedURLs, err := normalizeAndValidateURLs(req.AllowedURLs)
		if err != nil {
			http.Error(w, "allowed_urls contains an invalid absolute URL", http.StatusBadRequest)
			return
		}
		item.AllowedURLs = datatypes.JSONSlice[string](allowedURLs)
	}
	if req.AllowedIPs != nil {
		allowedIPs, err := normalizeAndValidateIPs(req.AllowedIPs)
		if err != nil {
			http.Error(w, "allowed_ips contains an invalid IP or CIDR", http.StatusBadRequest)
			return
		}
		item.AllowedIPs = datatypes.JSONSlice[string](allowedIPs)
	}
	if req.DataScopes != nil {
		dataScopes, err := normalizeAndValidateScopes(req.DataScopes)
		if err != nil {
			http.Error(w, "data_scopes contains an unsupported scope", http.StatusBadRequest)
			return
		}
		item.DataScopes = datatypes.JSONSlice[string](dataScopes)
	}

	if err := config.DB.Save(&item).Error; err != nil {
		http.Error(w, "failed to update integration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(integrationToResponse(item, ""))
}

// DeleteIntegration  DELETE /api/v1/admin/integrations/{id}
func DeleteIntegration(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid integration id", http.StatusBadRequest)
		return
	}

	if err := config.DB.Delete(&models.ThirdPartyIntegration{}, "id = ?", id).Error; err != nil {
		http.Error(w, "failed to delete integration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegenerateIntegrationKey  POST /api/v1/admin/integrations/{id}/regenerate-key
func RegenerateIntegrationKey(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid integration id", http.StatusBadRequest)
		return
	}

	var item models.ThirdPartyIntegration
	if err := config.DB.First(&item, "id = ?", id).Error; err != nil {
		http.Error(w, "integration not found", http.StatusNotFound)
		return
	}

	plainKey, err := generateAPIKey()
	if err != nil {
		http.Error(w, "failed to generate api key", http.StatusInternalServerError)
		return
	}
	keyHash, err := hashAPIKey(plainKey)
	if err != nil {
		http.Error(w, "failed to hash api key", http.StatusInternalServerError)
		return
	}

	if err := config.DB.Model(&item).Updates(map[string]interface{}{
		"api_key_hash":   keyHash,
		"api_key_prefix": plainKey[:12],
	}).Error; err != nil {
		http.Error(w, "failed to regenerate key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"api_key": plainKey})
}

// ProxyIntegrationDropdown  GET /api/v1/admin/integrations/{id}/proxy?path=/api/v1/masters/projects
//
// Fetches a remote path on behalf of the caller using the stored secret for the
// given integration.  Only integrations that have the
// "integration.dropdown.proxy" scope and are "active" may be proxied.
// The caller must be a valid JWT user — the stored secret is never exposed to
// the browser.
func ProxyIntegrationDropdown(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid integration id", http.StatusBadRequest)
		return
	}

	var item models.ThirdPartyIntegration
	if err := config.DB.First(&item, "id = ? AND status = ?", id, models.IntegrationStatusActive).Error; err != nil {
		http.Error(w, "integration not found or inactive", http.StatusNotFound)
		return
	}

	// Scope check — integration must allow dropdown proxying
	hasScope := false
	for _, s := range item.DataScopes {
		if s == models.IntegrationScopeDropdownProxyUse {
			hasScope = true
			break
		}
	}
	if !hasScope {
		http.Error(w, "integration does not have dropdown proxy scope", http.StatusForbidden)
		return
	}

	// Validate & resolve the remote path query param
	remotePath := strings.TrimSpace(r.URL.Query().Get("path"))
	if remotePath == "" {
		http.Error(w, "path query parameter is required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	// Build the full target URL using the integration's endpoint_url as base
	baseURL := strings.TrimRight(strings.TrimSpace(item.EndpointURL), "/")
	if baseURL == "" {
		http.Error(w, "integration has no endpoint_url configured", http.StatusBadRequest)
		return
	}
	targetURL := baseURL + remotePath
	if _, err := url.ParseRequestURI(targetURL); err != nil {
		http.Error(w, "resolved target URL is invalid", http.StatusBadRequest)
		return
	}

	// Decrypt the stored secret
	if strings.TrimSpace(item.SecretCipher) == "" {
		http.Error(w, "integration has no secret configured", http.StatusBadRequest)
		return
	}
	plainSecret, err := decryptIntegrationSecret(item.SecretCipher)
	if err != nil {
		http.Error(w, "failed to decrypt integration secret", http.StatusInternalServerError)
		return
	}

	// Build outbound request
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		http.Error(w, "failed to build proxy request", http.StatusInternalServerError)
		return
	}

	authHeader := item.AuthHeader
	if authHeader == "" {
		authHeader = "Authorization"
	}
	authScheme := strings.TrimSpace(item.AuthScheme)
	if authScheme == "" || strings.EqualFold(authScheme, "apikey") || strings.EqualFold(authScheme, "none") {
		// Raw key — just set the header value directly (e.g. X-Api-Key: <key>)
		req.Header.Set(authHeader, plainSecret)
	} else {
		// Scheme-prefixed (e.g. Bearer <token>)
		req.Header.Set(authHeader, fmt.Sprintf("%s %s", authScheme, plainSecret))
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read upstream response", http.StatusBadGateway)
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, fmt.Sprintf("upstream returned %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body) //nolint:errcheck
}

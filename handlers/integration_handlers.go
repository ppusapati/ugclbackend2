package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

type integrationFormCatalogItem struct {
	Code  string `json:"code"`
	Title string `json:"title"`
}

type integrationDropdownOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type vendorSitesProxyConfig struct {
	URL        string
	APIKey     string
	AuthHeader string
	AuthScheme string
	Timeout    time.Duration
	ValueField string
	LabelField string
}

// IntegrationHealth returns service health for third-party integrations.
func IntegrationHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"service":   "integration-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// WebhookContract returns webhook delivery contract details for provider implementations.
func WebhookContract(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	exampleBusinessID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	exampleSubmissionID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"supported_events": []string{
			string(models.EventFormSubmitted),
		},
		"supported_resource_types": []string{
			formSubmissionWebhookResourceType,
		},
		"payload_scope": "Submitted form payload only. Workflow history, permissions, and internal approval metadata are excluded.",
		"delivery_headers": []string{
			"X-Webhook-Signature",
			"X-Webhook-Delivery-ID",
			"X-Webhook-Attempt",
			"X-Webhook-Max-Retries",
			"X-Webhook-Timestamp",
			"X-Partner-Key",
		},
		"signature_algorithm": "HMAC-SHA256",
		"timestamp_format":    "RFC3339",
		"notes": []string{
			"Validate X-Webhook-Signature using shared secret.",
			"Reject stale or replayed events using X-Webhook-Timestamp and delivery ID.",
			"Return 2xx only when payload is successfully processed.",
			"Use event form.submitted for partner integrations that consume submitted form data.",
			"Treat data.form_data as the source of truth for submitted fields.",
			"X-Partner-Key is mandatory for form.submitted deliveries and is sourced from PARTNER_PORTAL_KEY.",
		},
		"example_subscription": map[string]interface{}{
			"url":            "https://partner.example.com/webhooks/ugcl/forms",
			"events":         []string{string(models.EventFormSubmitted)},
			"resource_types": []string{formSubmissionWebhookResourceType},
			"max_retries":    5,
			"retry_interval": 300,
		},
		"example_payload": models.NewWebhookPayload(
			models.EventFormSubmitted,
			formSubmissionWebhookResourceType,
			exampleSubmissionID.String(),
			exampleBusinessID,
			map[string]interface{}{
				"form_data": map[string]interface{}{
					"consumer_name": "Acme Infra Pvt Ltd",
					"connection_id": "WTR-0091",
					"meter_reading": 1842,
				},
			},
		),
		"management_endpoints": map[string]string{
			"create_webhook":     "/api/v1/webhooks",
			"list_webhooks":      "/api/v1/webhooks",
			"test_webhook":       "/api/v1/webhooks/{id}/test",
			"delivery_history":   "/api/v1/webhooks/{id}/deliveries",
			"delivery_logs":      "/api/v1/webhooks/deliveries/{deliveryId}/logs",
			"integration_health": "/api/v1/integrations/health",
			"webhook_contract":   "/api/v1/integrations/webhook-contract",
		},
	})
}

// IntegrationFormCatalog returns active form definitions for third-party discovery.
func IntegrationFormCatalog(w http.ResponseWriter, r *http.Request) {
	allowedCodes := allowedThirdPartyFormCodes()
	allowAll := len(allowedCodes) == 0

	query := config.DB.Model(&models.AppForm{}).
		Select("code, title").
		Where("is_active = ?", true)

	if !allowAll {
		codes := make([]string, 0, len(allowedCodes))
		for code := range allowedCodes {
			codes = append(codes, code)
		}
		query = query.Where("LOWER(code) IN ?", codes)
	}

	var forms []integrationFormCatalogItem
	if err := query.Order("code ASC").Find(&forms).Error; err != nil {
		http.Error(w, "failed to fetch form catalog", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"forms": forms,
		"count": len(forms),
		"notes": []string{
			"Use these form codes to configure webhook allowlists via X-UGCL-Allowed-Form-Codes.",
			"Outbound form.submitted payload contains submitted fields under data.form_data only.",
		},
	})
}

// IntegrationVendorSitesDropdown proxies third-party vendor site data into dropdown options.
// GET /api/v1/business/{businessCode}/integrations/vendor/sites
func IntegrationVendorSitesDropdown(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	businessCode := strings.TrimSpace(vars["businessCode"])
	if businessCode == "" {
		http.Error(w, "business code is required", http.StatusBadRequest)
		return
	}

	cfg, err := getVendorSitesProxyConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.Timeout)
	defer cancel()

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.URL, nil)
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}

	query := upstreamReq.URL.Query()
	query.Set("business_code", businessCode)
	query.Set("business_id", businessID.String())
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		query.Set("q", search)
	}
	upstreamReq.URL.RawQuery = query.Encode()
	upstreamReq.Header.Set("Accept", "application/json")

	if cfg.APIKey != "" {
		authValue := cfg.APIKey
		if cfg.AuthScheme != "" {
			authValue = cfg.AuthScheme + " " + cfg.APIKey
		}
		upstreamReq.Header.Set(cfg.AuthHeader, authValue)
	}

	resp, err := (&http.Client{Timeout: cfg.Timeout}).Do(upstreamReq)
	if err != nil {
		log.Printf("❌ Vendor sites upstream request failed: %v", err)
		http.Error(w, "failed to fetch vendor sites", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		log.Printf("❌ Vendor sites upstream returned status %d", resp.StatusCode)
		http.Error(w, "vendor service unavailable", http.StatusBadGateway)
		return
	}

	var payload interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		log.Printf("❌ Failed to decode vendor sites response: %v", err)
		http.Error(w, "invalid vendor response", http.StatusBadGateway)
		return
	}

	options := normalizeVendorDropdownOptions(payload, cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"options": options,
		"count":   len(options),
		"source":  "vendor_proxy",
	})
}

// IntegrationExternalDropdownProxy safely proxies whitelisted absolute dropdown endpoints.
// GET /api/v1/business/{businessCode}/integrations/external-dropdown?target=https://api.thirdparty.com/sites
func IntegrationExternalDropdownProxy(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business context not found", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	businessCode := strings.TrimSpace(vars["businessCode"])
	if businessCode == "" {
		http.Error(w, "business code is required", http.StatusBadRequest)
		return
	}

	targetURL, err := parseSafeDropdownTarget(r.URL.Query().Get("target"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proxyCfg := getExternalDropdownProxyConfig()
	if !isAllowedDropdownHost(targetURL.Hostname(), proxyCfg.AllowedHosts) {
		http.Error(w, "target host is not allowed", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), proxyCfg.Timeout)
	defer cancel()

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL.String(), nil)
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}

	query := upstreamReq.URL.Query()
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		query.Set("q", search)
	}
	query.Set("business_code", businessCode)
	query.Set("business_id", businessID.String())
	upstreamReq.URL.RawQuery = query.Encode()
	upstreamReq.Header.Set("Accept", "application/json")

	if proxyCfg.APIKey != "" {
		authValue := proxyCfg.APIKey
		if proxyCfg.AuthScheme != "" {
			authValue = proxyCfg.AuthScheme + " " + proxyCfg.APIKey
		}
		upstreamReq.Header.Set(proxyCfg.AuthHeader, authValue)
	}

	resp, err := (&http.Client{Timeout: proxyCfg.Timeout}).Do(upstreamReq)
	if err != nil {
		log.Printf("❌ External dropdown upstream request failed: %v", err)
		http.Error(w, "failed to fetch external dropdown data", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		log.Printf("❌ External dropdown upstream returned status %d", resp.StatusCode)
		http.Error(w, "external service unavailable", http.StatusBadGateway)
		return
	}

	var payload interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		log.Printf("❌ Failed to decode external dropdown response: %v", err)
		http.Error(w, "invalid external response", http.StatusBadGateway)
		return
	}

	options := normalizeVendorDropdownOptions(payload, vendorSitesProxyConfig{
		ValueField: proxyCfg.ValueField,
		LabelField: proxyCfg.LabelField,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"options": options,
		"count":   len(options),
		"source":  "external_dropdown_proxy",
		"target":  targetURL.Hostname(),
	})
}

func getVendorSitesProxyConfig() (vendorSitesProxyConfig, error) {
	cfg := vendorSitesProxyConfig{
		URL: envFirst(
			"THIRD_PARTY_VENDOR_SITES_URL",
		),
		APIKey: envFirst(
			"THIRD_PARTY_VENDOR_API_KEY",
			"THIRD_PARTY_PROXY_API_KEY",
		),
		AuthHeader: envFirst(
			"THIRD_PARTY_VENDOR_AUTH_HEADER",
			"THIRD_PARTY_AUTH_HEADER",
		),
		AuthScheme: envFirst(
			"THIRD_PARTY_VENDOR_AUTH_SCHEME",
			"THIRD_PARTY_AUTH_SCHEME",
		),
		Timeout: parseSecondsEnv(10,
			"THIRD_PARTY_VENDOR_TIMEOUT_SECONDS",
			"THIRD_PARTY_TIMEOUT_SECONDS",
		),
		ValueField: envFirst(
			"THIRD_PARTY_VENDOR_VALUE_FIELD",
			"THIRD_PARTY_VALUE_FIELD",
		),
		LabelField: envFirst(
			"THIRD_PARTY_VENDOR_LABEL_FIELD",
			"THIRD_PARTY_LABEL_FIELD",
		),
	}

	if cfg.URL == "" {
		return cfg, fmt.Errorf("missing THIRD_PARTY_VENDOR_SITES_URL")
	}

	if cfg.AuthHeader == "" {
		cfg.AuthHeader = "Authorization"
	}

	if cfg.AuthScheme == "" {
		cfg.AuthScheme = "Bearer"
	}

	if cfg.ValueField == "" {
		cfg.ValueField = "id"
	}

	if cfg.LabelField == "" {
		cfg.LabelField = "name"
	}

	return cfg, nil
}

type externalDropdownProxyConfig struct {
	AllowedHosts map[string]bool
	APIKey       string
	AuthHeader   string
	AuthScheme   string
	Timeout      time.Duration
	ValueField   string
	LabelField   string
}

func getExternalDropdownProxyConfig() externalDropdownProxyConfig {
	allowedHosts := make(map[string]bool)
	for _, part := range strings.Split(envFirst("THIRD_PARTY_DROPDOWN_ALLOWED_HOSTS", "THIRD_PARTY_ALLOWED_HOSTS"), ",") {
		host := strings.ToLower(strings.TrimSpace(part))
		if host != "" {
			allowedHosts[host] = true
		}
	}

	authHeader := envFirst("THIRD_PARTY_DROPDOWN_PROXY_AUTH_HEADER", "THIRD_PARTY_AUTH_HEADER")
	if authHeader == "" {
		authHeader = "Authorization"
	}

	authScheme := envFirst("THIRD_PARTY_DROPDOWN_PROXY_AUTH_SCHEME", "THIRD_PARTY_AUTH_SCHEME")
	if authScheme == "" {
		authScheme = "Bearer"
	}

	valueField := envFirst("THIRD_PARTY_DROPDOWN_PROXY_VALUE_FIELD", "THIRD_PARTY_VALUE_FIELD")
	if valueField == "" {
		valueField = "id"
	}

	labelField := envFirst("THIRD_PARTY_DROPDOWN_PROXY_LABEL_FIELD", "THIRD_PARTY_LABEL_FIELD")
	if labelField == "" {
		labelField = "name"
	}

	return externalDropdownProxyConfig{
		AllowedHosts: allowedHosts,
		APIKey:       envFirst("THIRD_PARTY_DROPDOWN_PROXY_API_KEY", "THIRD_PARTY_PROXY_API_KEY"),
		AuthHeader:   authHeader,
		AuthScheme:   authScheme,
		Timeout: parseSecondsEnv(10,
			"THIRD_PARTY_DROPDOWN_PROXY_TIMEOUT_SECONDS",
			"THIRD_PARTY_TIMEOUT_SECONDS",
		),
		ValueField: valueField,
		LabelField: labelField,
	}
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func parseSecondsEnv(defaultSeconds int, keys ...string) time.Duration {
	raw := envFirst(keys...)
	if raw == "" {
		return time.Duration(defaultSeconds) * time.Second
	}
	parsed, err := time.ParseDuration(raw + "s")
	if err != nil || parsed <= 0 {
		return time.Duration(defaultSeconds) * time.Second
	}
	return parsed
}

func parseSafeDropdownTarget(raw string) (*url.URL, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return nil, fmt.Errorf("target query parameter is required")
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL")
	}

	if !parsed.IsAbs() || parsed.Host == "" {
		return nil, fmt.Errorf("target must be an absolute URL")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && !(scheme == "http" && isLocalDropdownHost(parsed.Hostname())) {
		return nil, fmt.Errorf("target must use https")
	}

	parsed.User = nil
	parsed.Fragment = ""

	return parsed, nil
}

func isAllowedDropdownHost(host string, allowedHosts map[string]bool) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	if normalized == "" {
		return false
	}
	if isLocalDropdownHost(normalized) {
		return true
	}
	return allowedHosts[normalized]
}

func isLocalDropdownHost(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	return normalized == "localhost" || normalized == "127.0.0.1" || normalized == "::1"
}

func normalizeVendorDropdownOptions(payload interface{}, cfg vendorSitesProxyConfig) []integrationDropdownOption {
	items := extractCollection(payload)
	options := make([]integrationDropdownOption, 0, len(items))

	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			value := pickString(typed, []string{cfg.ValueField, "id", "value", "uuid", "site_id", "code"})
			label := pickString(typed, []string{cfg.LabelField, "name", "label", "title", "display_name", "site_name"})
			if value == "" {
				continue
			}
			if label == "" {
				label = value
			}
			options = append(options, integrationDropdownOption{Label: label, Value: value})
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
			value := strings.TrimSpace(typed)
			options = append(options, integrationDropdownOption{Label: value, Value: value})
		}
	}

	sort.SliceStable(options, func(i, j int) bool {
		return strings.ToLower(options[i].Label) < strings.ToLower(options[j].Label)
	})

	return options
}

func extractCollection(payload interface{}) []interface{} {
	if list, ok := payload.([]interface{}); ok {
		return list
	}

	if m, ok := payload.(map[string]interface{}); ok {
		candidateKeys := []string{"options", "sites", "items", "results", "records", "data"}
		for _, key := range candidateKeys {
			if v, exists := m[key]; exists {
				if list, ok := v.([]interface{}); ok {
					return list
				}
				if nestedMap, ok := v.(map[string]interface{}); ok {
					if nested := extractCollection(nestedMap); len(nested) > 0 {
						return nested
					}
				}
			}
		}

		// Fallback: treat top-level object as one option item.
		return []interface{}{m}
	}

	return []interface{}{}
}

func pickString(data map[string]interface{}, keys []string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if raw, ok := data[key]; ok {
			value := strings.TrimSpace(fmt.Sprint(raw))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func allowedThirdPartyFormCodes() map[string]bool {
	allowed := make(map[string]bool)
	raw := strings.TrimSpace(os.Getenv("THIRD_PARTY_EXPOSED_FORM_CODES"))
	if raw == "" {
		return allowed
	}

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		code := strings.ToLower(strings.TrimSpace(part))
		if code != "" {
			allowed[code] = true
		}
	}

	// Keep deterministic behavior for debugging if needed.
	if len(allowed) > 0 {
		keys := make([]string, 0, len(allowed))
		for key := range allowed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
	}

	return allowed
}

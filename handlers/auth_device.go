package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

const offlineRevalidationDays = 30

type registerTrustedDeviceRequest struct {
	InstallID  *string `json:"installId"`
	Platform   *string `json:"platform"`
	DeviceName *string `json:"deviceName"`
	AppVersion *string `json:"appVersion"`
}

type trustedDeviceStatusResponse struct {
	Trusted          bool       `json:"trusted"`
	OfflineAllowed   bool       `json:"offlineAllowed"`
	ClientID         string     `json:"clientId"`
	InstallID        *string    `json:"installId,omitempty"`
	Platform         *string    `json:"platform,omitempty"`
	AppVersion       *string    `json:"appVersion,omitempty"`
	LastSeenAt       *time.Time `json:"lastSeenAt,omitempty"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	RevocationReason *string    `json:"revocationReason,omitempty"`
	RevalidateBy     *time.Time `json:"revalidateBy,omitempty"`
}

type offlineBootstrapSite struct {
	SiteID               uuid.UUID `json:"siteId"`
	BusinessVerticalID   uuid.UUID `json:"businessVerticalId"`
	BusinessVerticalName string    `json:"businessVerticalName"`
	BusinessVerticalCode string    `json:"businessVerticalCode"`
	Name                 string    `json:"name"`
	Code                 string    `json:"code"`
	CanRead              bool      `json:"canRead"`
	CanCreate            bool      `json:"canCreate"`
	CanUpdate            bool      `json:"canUpdate"`
	CanDelete            bool      `json:"canDelete"`
}

type revokeTrustedDeviceRequest struct {
	Reason *string `json:"reason"`
}

type trustedDeviceRecordResponse struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"userId"`
	ClientID         string     `json:"clientId"`
	InstallID        *string    `json:"installId,omitempty"`
	Platform         *string    `json:"platform,omitempty"`
	DeviceName       *string    `json:"deviceName,omitempty"`
	AppVersion       *string    `json:"appVersion,omitempty"`
	OfflineAllowed   bool       `json:"offlineAllowed"`
	Trusted          bool       `json:"trusted"`
	LastSeenAt       time.Time  `json:"lastSeenAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	RevocationReason *string    `json:"revocationReason,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	RevalidateBy     time.Time  `json:"revalidateBy"`
}

func RegisterCurrentTrustedDevice(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req registerTrustedDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	clientID := strings.TrimSpace(r.Header.Get("X-Client-ID"))
	if clientID == "" {
		http.Error(w, "x-client-id header is required", http.StatusBadRequest)
		return
	}

	device, err := UpsertTrustedDeviceBinding(userID, clientID, req.InstallID, req.Platform, req.DeviceName, req.AppVersion)
	if err != nil {
		http.Error(w, "failed to register trusted device", http.StatusInternalServerError)
		return
	}

	revalidateBy := device.LastSeenAt.Add(offlineRevalidationDays * 24 * time.Hour)
	respondJSON(w, http.StatusOK, trustedDeviceStatusResponse{
		Trusted:          device.RevokedAt == nil,
		OfflineAllowed:   device.OfflineAllowed,
		ClientID:         device.ClientID,
		InstallID:        device.InstallID,
		Platform:         device.Platform,
		AppVersion:       device.AppVersion,
		LastSeenAt:       &device.LastSeenAt,
		RevokedAt:        device.RevokedAt,
		RevocationReason: device.RevocationReason,
		RevalidateBy:     &revalidateBy,
	})
}

func GetCurrentTrustedDeviceStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	clientID := strings.TrimSpace(r.Header.Get("X-Client-ID"))
	if clientID == "" {
		http.Error(w, "x-client-id header is required", http.StatusBadRequest)
		return
	}

	var device models.TrustedDevice
	if err := config.DB.Where("user_id = ? AND client_id = ?", userID, clientID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondJSON(w, http.StatusOK, trustedDeviceStatusResponse{
				Trusted:        false,
				OfflineAllowed: false,
				ClientID:       clientID,
			})
			return
		}
		http.Error(w, "failed to load trusted device status", http.StatusInternalServerError)
		return
	}

	trusted := device.RevokedAt == nil && device.OfflineAllowed
	revalidateBy := device.LastSeenAt.Add(offlineRevalidationDays * 24 * time.Hour)

	respondJSON(w, http.StatusOK, trustedDeviceStatusResponse{
		Trusted:          trusted,
		OfflineAllowed:   device.OfflineAllowed,
		ClientID:         device.ClientID,
		InstallID:        device.InstallID,
		Platform:         device.Platform,
		AppVersion:       device.AppVersion,
		LastSeenAt:       &device.LastSeenAt,
		RevokedAt:        device.RevokedAt,
		RevocationReason: device.RevocationReason,
		RevalidateBy:     &revalidateBy,
	})
}

func GetOfflineBootstrap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	authService := middleware.NewAuthService()
	userCtx, err := authService.LoadUserContext(r)
	if err != nil || userCtx == nil || userCtx.User == nil {
		http.Error(w, "failed to load user context", http.StatusUnauthorized)
		return
	}

	permissions := middleware.GetEffectivePermissions(r)
	businessRoles := make([]map[string]interface{}, 0)
	for _, ubr := range userCtx.User.UserBusinessRoles {
		if !ubr.IsActive || ubr.BusinessRole.ID == uuid.Nil {
			continue
		}
		businessRoles = append(businessRoles, map[string]interface{}{
			"role_id":       ubr.BusinessRole.ID,
			"role_name":     ubr.BusinessRole.Name,
			"display_name":  ubr.BusinessRole.DisplayName,
			"vertical_id":   ubr.BusinessRole.BusinessVerticalID,
			"vertical_name": ubr.BusinessRole.BusinessVertical.Name,
			"vertical_code": ubr.BusinessRole.BusinessVertical.Code,
			"level":         ubr.BusinessRole.Level,
		})
	}

	sites, err := loadOfflineSites(userCtx.User.ID, userCtx.IsSuperAdmin)
	if err != nil {
		http.Error(w, "failed to load site access", http.StatusInternalServerError)
		return
	}

	clientID := strings.TrimSpace(r.Header.Get("X-Client-ID"))
	var deviceStatus trustedDeviceStatusResponse
	if clientID != "" {
		var device models.TrustedDevice
		if err := config.DB.Where("user_id = ? AND client_id = ?", userCtx.User.ID, clientID).First(&device).Error; err == nil {
			revalidateBy := device.LastSeenAt.Add(offlineRevalidationDays * 24 * time.Hour)
			deviceStatus = trustedDeviceStatusResponse{
				Trusted:          device.RevokedAt == nil && device.OfflineAllowed,
				OfflineAllowed:   device.OfflineAllowed,
				ClientID:         clientID,
				InstallID:        device.InstallID,
				Platform:         device.Platform,
				AppVersion:       device.AppVersion,
				LastSeenAt:       &device.LastSeenAt,
				RevokedAt:        device.RevokedAt,
				RevocationReason: device.RevocationReason,
				RevalidateBy:     &revalidateBy,
			}
		}
	}

	var globalRole string
	if userCtx.User.RoleModel != nil {
		globalRole = userCtx.User.RoleModel.Name
	}

	now := time.Now().UTC()
	validUntil := now.Add(offlineRevalidationDays * 24 * time.Hour)

	payload := map[string]interface{}{
		"generatedAt": now,
		"policy": map[string]interface{}{
			"requiresLocalAuth":   true,
			"revalidateAfterDays": offlineRevalidationDays,
			"validUntil":          validUntil,
		},
		"user": map[string]interface{}{
			"id":               userCtx.User.ID,
			"name":             userCtx.User.Name,
			"email":            userCtx.User.Email,
			"phone":            userCtx.User.Phone,
			"role":             globalRole,
			"role_id":          userCtx.User.RoleID,
			"is_super_admin":   userCtx.IsSuperAdmin,
			"permissions":      permissions,
			"business_roles":   businessRoles,
			"role_assignments": CurrentUserRBACAssignments(*userCtx.User),
			"accessible_sites": sites,
		},
		"device": deviceStatus,
	}

	respondJSON(w, http.StatusOK, payload)
}

func ListMyTrustedDevices(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r, w)
	if !ok {
		return
	}

	var devices []models.TrustedDevice
	if err := config.DB.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&devices).Error; err != nil {
		http.Error(w, "failed to load trusted devices", http.StatusInternalServerError)
		return
	}

	response := make([]trustedDeviceRecordResponse, 0, len(devices))
	for _, device := range devices {
		response = append(response, toTrustedDeviceRecordResponse(device))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(response),
		"devices": response,
	})
}

func RevokeMyTrustedDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r, w)
	if !ok {
		return
	}

	deviceID, ok := trustedDeviceIDFromRoute(w, r)
	if !ok {
		return
	}

	var req revokeTrustedDeviceRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
	}

	reason := normalizeOptionalHeaderString(req.Reason)
	if reason == nil {
		defaultReason := "revoked_by_user"
		reason = &defaultReason
	}

	device, err := revokeTrustedDeviceByScope(userID, &deviceID, reason)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "trusted device not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to revoke trusted device", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toTrustedDeviceRecordResponse(*device))
}

func AllowMyTrustedDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(r, w)
	if !ok {
		return
	}

	deviceID, ok := trustedDeviceIDFromRoute(w, r)
	if !ok {
		return
	}

	device, err := allowTrustedDeviceByScope(userID, &deviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "trusted device not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to allow trusted device", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toTrustedDeviceRecordResponse(*device))
}

func AdminListUserTrustedDevices(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRoute(w, r)
	if !ok {
		return
	}

	var devices []models.TrustedDevice
	if err := config.DB.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&devices).Error; err != nil {
		http.Error(w, "failed to load trusted devices", http.StatusInternalServerError)
		return
	}

	response := make([]trustedDeviceRecordResponse, 0, len(devices))
	for _, device := range devices {
		response = append(response, toTrustedDeviceRecordResponse(device))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"userId":  userID,
		"count":   len(response),
		"devices": response,
	})
}

func AdminRevokeUserTrustedDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRoute(w, r)
	if !ok {
		return
	}
	deviceID, ok := trustedDeviceIDFromRoute(w, r)
	if !ok {
		return
	}

	var req revokeTrustedDeviceRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
	}

	reason := normalizeOptionalHeaderString(req.Reason)
	if reason == nil {
		defaultReason := "revoked_by_admin"
		reason = &defaultReason
	}

	device, err := revokeTrustedDeviceByScope(userID, &deviceID, reason)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "trusted device not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to revoke trusted device", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toTrustedDeviceRecordResponse(*device))
}

func AdminAllowUserTrustedDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRoute(w, r)
	if !ok {
		return
	}
	deviceID, ok := trustedDeviceIDFromRoute(w, r)
	if !ok {
		return
	}

	device, err := allowTrustedDeviceByScope(userID, &deviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "trusted device not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to allow trusted device", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toTrustedDeviceRecordResponse(*device))
}

func UpsertTrustedDeviceBinding(
	userID uuid.UUID,
	clientID string,
	installID *string,
	platform *string,
	deviceName *string,
	appVersion *string,
) (*models.TrustedDevice, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, gorm.ErrInvalidData
	}

	normalizedInstallID := normalizeOptionalHeaderString(installID)
	normalizedPlatform := normalizeOptionalHeaderString(platform)
	normalizedDeviceName := normalizeOptionalHeaderString(deviceName)
	normalizedAppVersion := normalizeOptionalHeaderString(appVersion)

	var existing models.TrustedDevice
	err := config.DB.Where("user_id = ? AND client_id = ?", userID, clientID).First(&existing).Error
	now := time.Now().UTC()
	if err == nil {
		existing.InstallID = normalizedInstallID
		existing.Platform = normalizedPlatform
		existing.DeviceName = normalizedDeviceName
		existing.AppVersion = normalizedAppVersion
		existing.LastSeenAt = now
		if existing.RevokedAt == nil {
			existing.OfflineAllowed = true
		}
		if saveErr := config.DB.Save(&existing).Error; saveErr != nil {
			return nil, saveErr
		}
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	device := models.TrustedDevice{
		UserID:         userID,
		ClientID:       clientID,
		InstallID:      normalizedInstallID,
		Platform:       normalizedPlatform,
		DeviceName:     normalizedDeviceName,
		AppVersion:     normalizedAppVersion,
		OfflineAllowed: true,
		LastSeenAt:     now,
	}
	if createErr := config.DB.Create(&device).Error; createErr != nil {
		return nil, createErr
	}
	return &device, nil
}

func normalizeOptionalHeaderString(input *string) *string {
	if input == nil {
		return nil
	}
	value := strings.TrimSpace(*input)
	if value == "" {
		return nil
	}
	return &value
}

func authenticatedUserID(r *http.Request, w http.ResponseWriter) (uuid.UUID, bool) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return uuid.Nil, false
	}

	return userID, true
}

func userIDFromRoute(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userIDRaw := strings.TrimSpace(mux.Vars(r)["id"])
	if userIDRaw == "" {
		http.Error(w, "user id is required", http.StatusBadRequest)
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return userID, true
}

func trustedDeviceIDFromRoute(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	deviceIDRaw := strings.TrimSpace(mux.Vars(r)["deviceId"])
	if deviceIDRaw == "" {
		http.Error(w, "device id is required", http.StatusBadRequest)
		return uuid.Nil, false
	}

	deviceID, err := uuid.Parse(deviceIDRaw)
	if err != nil {
		http.Error(w, "invalid device id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return deviceID, true
}

func revokeTrustedDeviceByScope(userID uuid.UUID, deviceID *uuid.UUID, reason *string) (*models.TrustedDevice, error) {
	query := config.DB.Where("user_id = ?", userID)
	if deviceID != nil {
		query = query.Where("id = ?", *deviceID)
	}

	var device models.TrustedDevice
	if err := query.First(&device).Error; err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	device.OfflineAllowed = false
	device.RevokedAt = &now
	device.RevocationReason = reason
	if err := config.DB.Save(&device).Error; err != nil {
		return nil, err
	}

	return &device, nil
}

func allowTrustedDeviceByScope(userID uuid.UUID, deviceID *uuid.UUID) (*models.TrustedDevice, error) {
	query := config.DB.Where("user_id = ?", userID)
	if deviceID != nil {
		query = query.Where("id = ?", *deviceID)
	}

	var device models.TrustedDevice
	if err := query.First(&device).Error; err != nil {
		return nil, err
	}

	device.OfflineAllowed = true
	device.RevokedAt = nil
	device.RevocationReason = nil
	if err := config.DB.Save(&device).Error; err != nil {
		return nil, err
	}

	return &device, nil
}

func toTrustedDeviceRecordResponse(device models.TrustedDevice) trustedDeviceRecordResponse {
	trusted := device.RevokedAt == nil && device.OfflineAllowed
	revalidateBy := device.LastSeenAt.Add(offlineRevalidationDays * 24 * time.Hour)
	return trustedDeviceRecordResponse{
		ID:               device.ID,
		UserID:           device.UserID,
		ClientID:         device.ClientID,
		InstallID:        device.InstallID,
		Platform:         device.Platform,
		DeviceName:       device.DeviceName,
		AppVersion:       device.AppVersion,
		OfflineAllowed:   device.OfflineAllowed,
		Trusted:          trusted,
		LastSeenAt:       device.LastSeenAt,
		RevokedAt:        device.RevokedAt,
		RevocationReason: device.RevocationReason,
		CreatedAt:        device.CreatedAt,
		UpdatedAt:        device.UpdatedAt,
		RevalidateBy:     revalidateBy,
	}
}

func loadOfflineSites(userID uuid.UUID, isSuperAdmin bool) ([]offlineBootstrapSite, error) {
	if isSuperAdmin {
		var sites []struct {
			ID                 uuid.UUID
			BusinessVerticalID uuid.UUID
			Name               string
			Code               string
		}
		if err := config.DB.Table("sites").
			Select("id, business_vertical_id, name, code").
			Where("is_active = ?", true).
			Find(&sites).Error; err != nil {
			return nil, err
		}

		result := make([]offlineBootstrapSite, 0, len(sites))
		for _, site := range sites {
			result = append(result, offlineBootstrapSite{
				SiteID:             site.ID,
				BusinessVerticalID: site.BusinessVerticalID,
				Name:               site.Name,
				Code:               site.Code,
				CanRead:            true,
				CanCreate:          true,
				CanUpdate:          true,
				CanDelete:          true,
			})
		}
		return result, nil
	}

	var rows []struct {
		SiteID               uuid.UUID
		BusinessVerticalID   uuid.UUID
		BusinessVerticalName string
		BusinessVerticalCode string
		Name                 string
		Code                 string
		CanRead              bool
		CanCreate            bool
		CanUpdate            bool
		CanDelete            bool
	}

	err := config.DB.Table("user_site_accesses").
		Select(`user_site_accesses.site_id,
			sites.business_vertical_id,
			bv.name AS business_vertical_name,
			bv.code AS business_vertical_code,
			sites.name,
			sites.code,
			user_site_accesses.can_read,
			user_site_accesses.can_create,
			user_site_accesses.can_update,
			user_site_accesses.can_delete`).
		Joins("JOIN sites ON sites.id = user_site_accesses.site_id").
		Joins("JOIN business_verticals bv ON bv.id = sites.business_vertical_id").
		Where("user_site_accesses.user_id = ? AND sites.is_active = ?", userID, true).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]offlineBootstrapSite, 0, len(rows))
	for _, row := range rows {
		result = append(result, offlineBootstrapSite{
			SiteID:               row.SiteID,
			BusinessVerticalID:   row.BusinessVerticalID,
			BusinessVerticalName: row.BusinessVerticalName,
			BusinessVerticalCode: row.BusinessVerticalCode,
			Name:                 row.Name,
			Code:                 row.Code,
			CanRead:              row.CanRead,
			CanCreate:            row.CanCreate,
			CanUpdate:            row.CanUpdate,
			CanDelete:            row.CanDelete,
		})
	}
	return result, nil
}

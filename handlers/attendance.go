package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

type attendanceCommandRequest struct {
	SessionID        *uuid.UUID             `json:"sessionId,omitempty"`
	SiteID           uuid.UUID              `json:"siteId"`
	Latitude         float64                `json:"latitude"`
	Longitude        float64                `json:"longitude"`
	Accuracy         float64                `json:"accuracy"`
	CapturedAt       *time.Time             `json:"capturedAt,omitempty"`
	DeviceID         string                 `json:"deviceId"`
	AppState         *string                `json:"appState,omitempty"`
	NetworkStatus    *string                `json:"networkStatus,omitempty"`
	BatteryLevel     *float64               `json:"batteryLevel,omitempty"`
	IsMockLocation   bool                   `json:"isMockLocation"`
	IsGPSEnabled     bool                   `json:"isGpsEnabled"`
	ClockSkewSeconds int                    `json:"clockSkewSeconds,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Policy           *attendancePolicyInput `json:"policy,omitempty"`
}

type attendancePolicyInput struct {
	AllowedRadiusMeters float64 `json:"allowedRadiusMeters,omitempty"`
	MaxAccuracyMeters   float64 `json:"maxAccuracyMeters,omitempty"`
	MaxPingIntervalSec  int     `json:"maxPingIntervalSec,omitempty"`
	MaxSpeedKmh         float64 `json:"maxSpeedKmh,omitempty"`
	MaxClockSkewSeconds int     `json:"maxClockSkewSeconds,omitempty"`
	StrictEnforcement   bool    `json:"strictEnforcement,omitempty"`
}

type attendanceCommandResponse struct {
	Session    models.AttendanceSession          `json:"session"`
	Validation *utils.AttendanceValidationResult `json:"validation,omitempty"`
	Event      *models.AttendanceEvent           `json:"event,omitempty"`
	Ping       *models.TrackingPing              `json:"ping,omitempty"`
}

type attendanceHeadcountRow struct {
	SiteID      uuid.UUID `json:"siteId"`
	SiteName    string    `json:"siteName"`
	ActiveCount int64     `json:"activeCount"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

func CheckInAttendance(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	businessID, err := getBusinessIDFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req attendanceCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SiteID == uuid.Nil || req.DeviceID == "" {
		http.Error(w, "siteId and deviceId are required", http.StatusBadRequest)
		return
	}

	if hasActiveSession(user.ID) {
		http.Error(w, "user already has an active attendance session", http.StatusConflict)
		return
	}

	site, err := loadAccessibleSite(r, user, businessID, req.SiteID)
	if err != nil {
		handleAttendanceError(w, err)
		return
	}

	validation, capturedAt, err := validateAttendanceRequest(site, req, nil, nil)
	if err != nil {
		handleAttendanceError(w, err)
		return
	}
	if validation.ValidationStatus == models.AttendanceValidationRejected {
		writeValidationRejection(w, validation)
		return
	}

	anomalyFlags, err := utils.SerializeStringArray(validation.AnomalyFlags)
	if err != nil {
		http.Error(w, "failed to serialize anomalies", http.StatusInternalServerError)
		return
	}
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		http.Error(w, "failed to serialize metadata", http.StatusBadRequest)
		return
	}

	session := models.AttendanceSession{
		UserID:             user.ID,
		SiteID:             site.ID,
		BusinessVerticalID: businessID,
		Status:             models.AttendanceSessionStatusActive,
		CheckInAt:          capturedAt,
		LastSeenAt:         capturedAt,
		CheckInLatitude:    req.Latitude,
		CheckInLongitude:   req.Longitude,
		CheckInAccuracy:    req.Accuracy,
		LastLatitude:       req.Latitude,
		LastLongitude:      req.Longitude,
		LastAccuracy:       req.Accuracy,
		DeviceID:           req.DeviceID,
		ValidationMethod:   validation.ValidationMethod,
		ValidationStatus:   validation.ValidationStatus,
		ValidationReason:   stringPtr(validation.ValidationReason),
		AnomalyFlags:       anomalyFlags,
		Metadata:           metadata,
	}

	event := buildAttendanceEvent(session.ID, user.ID, site.ID, businessID, models.AttendanceEventTypeCheckIn, req, capturedAt, validation, anomalyFlags, metadata)

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		event.SessionID = session.ID
		return tx.Create(&event).Error
	}); err != nil {
		http.Error(w, "failed to create attendance session", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, attendanceCommandResponse{Session: session, Validation: validation, Event: &event})
}

func HeartbeatAttendance(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	businessID, err := getBusinessIDFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req attendanceCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	session, err := loadActiveSession(user.ID, req.SessionID)
	if err != nil {
		handleAttendanceError(w, err)
		return
	}
	if session.BusinessVerticalID != businessID {
		http.Error(w, "attendance session does not belong to current business", http.StatusForbidden)
		return
	}

	if req.SiteID == uuid.Nil {
		req.SiteID = session.SiteID
	}
	if req.SiteID != session.SiteID {
		http.Error(w, "heartbeat site does not match active session", http.StatusBadRequest)
		return
	}

	previous := &utils.LocationSample{Latitude: session.LastLatitude, Longitude: session.LastLongitude, Timestamp: session.LastSeenAt}
	validation, capturedAt, err := validateAttendanceRequest(session.Site, req, previous, &session.LastSeenAt)
	if err != nil {
		handleAttendanceError(w, err)
		return
	}

	anomalyFlags, err := utils.SerializeStringArray(validation.AnomalyFlags)
	if err != nil {
		http.Error(w, "failed to serialize anomalies", http.StatusInternalServerError)
		return
	}
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		http.Error(w, "failed to serialize metadata", http.StatusBadRequest)
		return
	}

	ping := models.TrackingPing{
		SessionID:          session.ID,
		UserID:             user.ID,
		SiteID:             session.SiteID,
		BusinessVerticalID: businessID,
		PingTime:           capturedAt,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		Accuracy:           req.Accuracy,
		DeviceID:           req.DeviceID,
		InsideGeofence:     validation.InsideBoundary,
		DistanceFromSiteM:  validation.DistanceFromSiteM,
		IsMockLocation:     req.IsMockLocation,
		IsGpsEnabled:       req.IsGPSEnabled,
		ClockSkewSeconds:   intPtr(req.ClockSkewSeconds),
		SyncStatus:         "received",
		AnomalyFlags:       anomalyFlags,
		Payload:            metadata,
		ServerReceivedAt:   time.Now().UTC(),
	}

	var event *models.AttendanceEvent
	if len(validation.AnomalyFlags) > 0 {
		e := buildAttendanceEvent(session.ID, user.ID, session.SiteID, businessID, models.AttendanceEventTypeHeartbeat, req, capturedAt, validation, anomalyFlags, metadata)
		event = &e
	}

	session.LastSeenAt = capturedAt
	session.LastLatitude = req.Latitude
	session.LastLongitude = req.Longitude
	session.LastAccuracy = req.Accuracy
	session.ValidationMethod = validation.ValidationMethod
	session.ValidationStatus = validation.ValidationStatus
	session.ValidationReason = stringPtr(validation.ValidationReason)
	session.AnomalyFlags = anomalyFlags

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&ping).Error; err != nil {
			return err
		}
		if event != nil {
			if err := tx.Create(event).Error; err != nil {
				return err
			}
		}
		return tx.Save(&session).Error
	}); err != nil {
		http.Error(w, "failed to persist attendance heartbeat", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, attendanceCommandResponse{Session: session, Validation: validation, Ping: &ping, Event: event})
}

func CheckOutAttendance(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	businessID, err := getBusinessIDFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req attendanceCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	session, err := loadActiveSession(user.ID, req.SessionID)
	if err != nil {
		handleAttendanceError(w, err)
		return
	}
	if session.BusinessVerticalID != businessID {
		http.Error(w, "attendance session does not belong to current business", http.StatusForbidden)
		return
	}

	if req.SiteID == uuid.Nil {
		req.SiteID = session.SiteID
	}
	if req.SiteID != session.SiteID {
		http.Error(w, "check-out site does not match active session", http.StatusBadRequest)
		return
	}

	previous := &utils.LocationSample{Latitude: session.LastLatitude, Longitude: session.LastLongitude, Timestamp: session.LastSeenAt}
	validation, capturedAt, err := validateAttendanceRequest(session.Site, req, previous, &session.LastSeenAt)
	if err != nil {
		handleAttendanceError(w, err)
		return
	}
	if validation.ValidationStatus == models.AttendanceValidationRejected {
		writeValidationRejection(w, validation)
		return
	}

	anomalyFlags, err := utils.SerializeStringArray(validation.AnomalyFlags)
	if err != nil {
		http.Error(w, "failed to serialize anomalies", http.StatusInternalServerError)
		return
	}
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		http.Error(w, "failed to serialize metadata", http.StatusBadRequest)
		return
	}

	event := buildAttendanceEvent(session.ID, user.ID, session.SiteID, businessID, models.AttendanceEventTypeCheckOut, req, capturedAt, validation, anomalyFlags, metadata)
	session.Status = models.AttendanceSessionStatusCompleted
	session.CheckOutAt = &capturedAt
	session.LastSeenAt = capturedAt
	session.LastLatitude = req.Latitude
	session.LastLongitude = req.Longitude
	session.LastAccuracy = req.Accuracy
	session.CheckOutLatitude = floatPtr(req.Latitude)
	session.CheckOutLongitude = floatPtr(req.Longitude)
	session.CheckOutAccuracy = floatPtr(req.Accuracy)
	session.ValidationMethod = validation.ValidationMethod
	session.ValidationStatus = validation.ValidationStatus
	session.ValidationReason = stringPtr(validation.ValidationReason)
	session.AnomalyFlags = anomalyFlags

	if err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		return tx.Save(&session).Error
	}); err != nil {
		http.Error(w, "failed to complete attendance session", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, attendanceCommandResponse{Session: session, Validation: validation, Event: &event})
}

func GetActiveAttendanceSessions(w http.ResponseWriter, r *http.Request) {
	businessID, err := getBusinessIDFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, limit := parsePagination(r)
	query := config.DB.Model(&models.AttendanceSession{}).
		Preload("User").
		Preload("Site").
		Where("business_vertical_id = ? AND status = ?", businessID, models.AttendanceSessionStatusActive)

	if siteID, ok := parseUUIDQuery(r, "siteId"); ok {
		query = query.Where("site_id = ?", siteID)
	}
	if userID, ok := parseUUIDQuery(r, "userId"); ok {
		query = query.Where("user_id = ?", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, "failed to count active sessions", http.StatusInternalServerError)
		return
	}

	var sessions []models.AttendanceSession
	if err := query.Order("last_seen_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&sessions).Error; err != nil {
		http.Error(w, "failed to fetch active sessions", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  sessions,
	})
}

func GetAttendanceLogs(w http.ResponseWriter, r *http.Request) {
	businessID, err := getBusinessIDFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	page, limit := parsePagination(r)
	query := config.DB.Model(&models.AttendanceSession{}).
		Preload("User").
		Preload("Site").
		Where("business_vertical_id = ?", businessID)

	if siteID, ok := parseUUIDQuery(r, "siteId"); ok {
		query = query.Where("site_id = ?", siteID)
	}
	if userID, ok := parseUUIDQuery(r, "userId"); ok {
		query = query.Where("user_id = ?", userID)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if validationStatus := r.URL.Query().Get("validationStatus"); validationStatus != "" {
		query = query.Where("validation_status = ?", validationStatus)
	}
	if from, ok := parseTimeQuery(r, "from"); ok {
		query = query.Where("check_in_at >= ?", from)
	}
	if to, ok := parseTimeQuery(r, "to"); ok {
		query = query.Where("check_in_at <= ?", to)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, "failed to count attendance logs", http.StatusInternalServerError)
		return
	}

	var sessions []models.AttendanceSession
	if err := query.Order("check_in_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&sessions).Error; err != nil {
		http.Error(w, "failed to fetch attendance logs", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"data":  sessions,
	})
}

func GetAttendanceHeadcount(w http.ResponseWriter, r *http.Request) {
	businessID, err := getBusinessIDFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := config.DB.Table("attendance_sessions").
		Select("attendance_sessions.site_id, sites.name as site_name, COUNT(attendance_sessions.id) as active_count, MAX(attendance_sessions.last_seen_at) as last_seen_at").
		Joins("JOIN sites ON sites.id = attendance_sessions.site_id").
		Where("attendance_sessions.business_vertical_id = ? AND attendance_sessions.status = ? AND attendance_sessions.deleted_at IS NULL", businessID, models.AttendanceSessionStatusActive).
		Group("attendance_sessions.site_id, sites.name").
		Order("active_count DESC, sites.name ASC")

	if siteID, ok := parseUUIDQuery(r, "siteId"); ok {
		query = query.Where("attendance_sessions.site_id = ?", siteID)
	}

	var rows []attendanceHeadcountRow
	if err := query.Scan(&rows).Error; err != nil {
		http.Error(w, "failed to fetch headcount", http.StatusInternalServerError)
		return
	}

	var totalActive int64
	for _, row := range rows {
		totalActive += row.ActiveCount
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"totalActive": totalActive,
		"sites":       rows,
	})
}

func GetEmployeeAttendanceTimeline(w http.ResponseWriter, r *http.Request) {
	businessID, err := getBusinessIDFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(mux.Vars(r)["userId"])
	if err != nil {
		http.Error(w, "invalid userId", http.StatusBadRequest)
		return
	}

	query := config.DB.Model(&models.AttendanceSession{}).
		Preload("Site").
		Preload("Events", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("event_time ASC")
		}).
		Preload("Pings", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("ping_time ASC")
		}).
		Where("business_vertical_id = ? AND user_id = ?", businessID, userID)

	if sessionID, ok := parseUUIDQuery(r, "sessionId"); ok {
		query = query.Where("id = ?", sessionID)
	}
	if from, ok := parseTimeQuery(r, "from"); ok {
		query = query.Where("check_in_at >= ?", from)
	}
	if to, ok := parseTimeQuery(r, "to"); ok {
		query = query.Where("check_in_at <= ?", to)
	}

	var sessions []models.AttendanceSession
	if err := query.Order("check_in_at DESC").Find(&sessions).Error; err != nil {
		http.Error(w, "failed to fetch attendance timeline", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": sessions,
	})
}

func getBusinessIDFromContext(r *http.Request) (uuid.UUID, error) {
	businessContext := middleware.GetUserBusinessContext(r)
	if businessContext == nil {
		return uuid.Nil, errors.New("business context not found")
	}
	businessID, ok := businessContext["business_id"].(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid business context")
	}
	return businessID, nil
}

func loadAccessibleSite(r *http.Request, user models.User, businessID uuid.UUID, siteID uuid.UUID) (models.Site, error) {
	var site models.Site
	if err := config.DB.Where("id = ? AND business_vertical_id = ? AND is_active = ?", siteID, businessID, true).First(&site).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return site, errors.New("site not found")
		}
		return site, err
	}

	if user.RoleModel != nil && user.RoleModel.Name == "super_admin" {
		return site, nil
	}

	if middleware.HasBusinessPermissionInContext(r, "site:view") {
		return site, nil
	}

	var count int64
	if err := config.DB.Model(&models.UserSiteAccess{}).
		Where("user_id = ? AND site_id = ? AND can_read = ?", user.ID, siteID, true).
		Count(&count).Error; err != nil {
		return site, err
	}
	if count == 0 {
		return site, errors.New("user does not have access to this site")
	}

	return site, nil
}

func hasActiveSession(userID uuid.UUID) bool {
	var count int64
	config.DB.Model(&models.AttendanceSession{}).
		Where("user_id = ? AND status = ?", userID, models.AttendanceSessionStatusActive).
		Count(&count)
	return count > 0
}

func loadActiveSession(userID uuid.UUID, sessionID *uuid.UUID) (models.AttendanceSession, error) {
	var session models.AttendanceSession
	query := config.DB.Preload("Site").Where("user_id = ? AND status = ?", userID, models.AttendanceSessionStatusActive)
	if sessionID != nil && *sessionID != uuid.Nil {
		query = query.Where("id = ?", *sessionID)
	}
	if err := query.Order("check_in_at DESC").First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return session, errors.New("active attendance session not found")
		}
		return session, err
	}
	return session, nil
}

func validateAttendanceRequest(site models.Site, req attendanceCommandRequest, previous *utils.LocationSample, lastPing *time.Time) (*utils.AttendanceValidationResult, time.Time, error) {
	capturedAt := time.Now().UTC()
	if req.CapturedAt != nil && !req.CapturedAt.IsZero() {
		capturedAt = req.CapturedAt.UTC()
	}

	policy := utils.AttendanceValidationPolicy{}
	if req.Policy != nil {
		policy = utils.AttendanceValidationPolicy{
			AllowedRadiusMeters: req.Policy.AllowedRadiusMeters,
			MaxAccuracyMeters:   req.Policy.MaxAccuracyMeters,
			MaxSpeedKmh:         req.Policy.MaxSpeedKmh,
			MaxClockSkewSeconds: req.Policy.MaxClockSkewSeconds,
			StrictEnforcement:   req.Policy.StrictEnforcement,
		}
		if req.Policy.MaxPingIntervalSec > 0 {
			policy.MaxPingInterval = time.Duration(req.Policy.MaxPingIntervalSec) * time.Second
		}
	}

	validation, err := utils.ValidateAttendanceInput(utils.AttendanceValidationInput{
		Site:           site,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		AccuracyMeters: req.Accuracy,
		CapturedAt:     capturedAt,
		Integrity: utils.DeviceIntegritySnapshot{
			IsMockLocation:   req.IsMockLocation,
			IsGPSEnabled:     req.IsGPSEnabled,
			ClockSkewSeconds: req.ClockSkewSeconds,
		},
		PreviousSample:   previous,
		LastAcceptedPing: lastPing,
		Policy:           policy,
	})
	if err != nil {
		return nil, time.Time{}, err
	}

	return validation, capturedAt, nil
}

func buildAttendanceEvent(sessionID, userID, siteID, businessID uuid.UUID, eventType string, req attendanceCommandRequest, capturedAt time.Time, validation *utils.AttendanceValidationResult, anomalyFlags *string, metadata *string) models.AttendanceEvent {
	return models.AttendanceEvent{
		SessionID:          sessionID,
		UserID:             userID,
		SiteID:             siteID,
		BusinessVerticalID: businessID,
		EventType:          eventType,
		EventTime:          capturedAt,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		Accuracy:           req.Accuracy,
		DeviceID:           req.DeviceID,
		ValidationMethod:   validation.ValidationMethod,
		ValidationStatus:   validation.ValidationStatus,
		ValidationReason:   stringPtr(validation.ValidationReason),
		AnomalyFlags:       anomalyFlags,
		IsMockLocation:     req.IsMockLocation,
		IsGpsEnabled:       req.IsGPSEnabled,
		AppState:           req.AppState,
		NetworkStatus:      req.NetworkStatus,
		BatteryLevel:       req.BatteryLevel,
		Payload:            metadata,
		ServerReceivedAt:   time.Now().UTC(),
	}
}

func marshalMetadata(metadata map[string]interface{}) (*string, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	value := string(payload)
	return &value, nil
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func writeValidationRejection(w http.ResponseWriter, validation *utils.AttendanceValidationResult) {
	respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
		"message":    "attendance validation rejected",
		"validation": validation,
	})
}

func handleAttendanceError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "site not found", "active attendance session not found":
		http.Error(w, err.Error(), http.StatusNotFound)
	case "user does not have access to this site", "attendance session does not belong to current business":
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}

func intPtr(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func parsePagination(r *http.Request) (int, int) {
	page := 1
	limit := 20
	if pageValue := r.URL.Query().Get("page"); pageValue != "" {
		if parsed, err := strconv.Atoi(pageValue); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if limitValue := r.URL.Query().Get("limit"); limitValue != "" {
		if parsed, err := strconv.Atoi(limitValue); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	return page, limit
}

func parseUUIDQuery(r *http.Request, key string) (uuid.UUID, bool) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return uuid.Nil, false
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, false
	}
	return parsed, true
}

func parseTimeQuery(r *http.Request, key string) (time.Time, bool) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

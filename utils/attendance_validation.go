package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"p9e.in/ugcl/models"
)

const (
	AttendanceAnomalyOutsideBoundary = "outside_boundary"
	AttendanceAnomalyPoorAccuracy    = "poor_accuracy"
	AttendanceAnomalyMockLocation    = "mock_location_detected"
	AttendanceAnomalyGpsDisabled     = "gps_disabled"
	AttendanceAnomalyStalePing       = "stale_ping"
	AttendanceAnomalyImpossibleSpeed = "impossible_speed"
	AttendanceAnomalyClockSkew       = "clock_skew"

	DefaultAttendanceRadiusMeters = 150.0
	DefaultMaxAccuracyMeters      = 100.0
	DefaultMaxSpeedKmh            = 140.0
	DefaultMaxClockSkewSeconds    = 300
)

type SiteLocation struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Address string  `json:"address,omitempty"`
}

type AttendanceValidationPolicy struct {
	AllowedRadiusMeters float64
	MaxAccuracyMeters   float64
	MaxPingInterval     time.Duration
	MaxSpeedKmh         float64
	MaxClockSkewSeconds int
	StrictEnforcement   bool
}

type DeviceIntegritySnapshot struct {
	IsMockLocation   bool
	IsGPSEnabled     bool
	ClockSkewSeconds int
}

type LocationSample struct {
	Latitude  float64
	Longitude float64
	Timestamp time.Time
}

type AttendanceValidationInput struct {
	Site             models.Site
	Latitude         float64
	Longitude        float64
	AccuracyMeters   float64
	CapturedAt       time.Time
	Integrity        DeviceIntegritySnapshot
	PreviousSample   *LocationSample
	LastAcceptedPing *time.Time
	Policy           AttendanceValidationPolicy
}

type AttendanceValidationResult struct {
	InsideBoundary       bool
	DistanceFromSiteM    *float64
	ValidationStatus     string
	ValidationReason     string
	ValidationMethod     string
	AnomalyFlags         []string
	SiteLocation         *SiteLocation
	ResolvedGeofence     *Geofence
	NormalizedPolicy     AttendanceValidationPolicy
	ComputedSpeedKmh     *float64
	SecondsSinceLastPing *float64
}

func NormalizeAttendancePolicy(policy AttendanceValidationPolicy) AttendanceValidationPolicy {
	if policy.AllowedRadiusMeters <= 0 {
		policy.AllowedRadiusMeters = DefaultAttendanceRadiusMeters
	}
	if policy.MaxAccuracyMeters <= 0 {
		policy.MaxAccuracyMeters = DefaultMaxAccuracyMeters
	}
	if policy.MaxSpeedKmh <= 0 {
		policy.MaxSpeedKmh = DefaultMaxSpeedKmh
	}
	if policy.MaxClockSkewSeconds <= 0 {
		policy.MaxClockSkewSeconds = DefaultMaxClockSkewSeconds
	}
	return policy
}

func ParseSiteLocation(locationJSON *string) (*SiteLocation, error) {
	if locationJSON == nil || *locationJSON == "" {
		return nil, nil
	}

	var location SiteLocation
	if err := json.Unmarshal([]byte(*locationJSON), &location); err != nil {
		return nil, fmt.Errorf("invalid site location JSON: %w", err)
	}

	if err := validateCoordinate(Coordinate{Lat: location.Lat, Lng: location.Lng}); err != nil {
		return nil, err
	}

	return &location, nil
}

func SerializeStringArray(values []string) (*string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	result := string(payload)
	return &result, nil
}

func ValidateAttendanceInput(input AttendanceValidationInput) (*AttendanceValidationResult, error) {
	policy := NormalizeAttendancePolicy(input.Policy)
	point := Coordinate{Lat: input.Latitude, Lng: input.Longitude}
	if err := validateCoordinate(point); err != nil {
		return nil, err
	}
	if input.CapturedAt.IsZero() {
		return nil, errors.New("captured time is required")
	}

	location, err := ParseSiteLocation(input.Site.Location)
	if err != nil {
		return nil, err
	}

	geofence, err := ParseGeofenceValue(input.Site.Geofence)
	if err != nil {
		return nil, err
	}

	result := &AttendanceValidationResult{
		InsideBoundary:   true,
		ValidationStatus: models.AttendanceValidationAccepted,
		ValidationMethod: "radius",
		SiteLocation:     location,
		ResolvedGeofence: geofence,
		NormalizedPolicy: policy,
	}

	if geofence != nil && len(geofence.Coordinates) >= 3 {
		result.ValidationMethod = "geofence"
		result.InsideBoundary = IsPointInPolygon(point, geofence.Coordinates)
	} else if location != nil {
		distance := HaversineDistanceMeters(location.Lat, location.Lng, input.Latitude, input.Longitude)
		result.DistanceFromSiteM = &distance
		result.InsideBoundary = distance <= policy.AllowedRadiusMeters
	} else {
		return nil, errors.New("site must have a location or geofence for attendance validation")
	}

	if location != nil && result.DistanceFromSiteM == nil {
		distance := HaversineDistanceMeters(location.Lat, location.Lng, input.Latitude, input.Longitude)
		result.DistanceFromSiteM = &distance
	}

	if !result.InsideBoundary {
		result.AnomalyFlags = append(result.AnomalyFlags, AttendanceAnomalyOutsideBoundary)
	}

	if input.AccuracyMeters > policy.MaxAccuracyMeters {
		result.AnomalyFlags = append(result.AnomalyFlags, AttendanceAnomalyPoorAccuracy)
	}
	if input.Integrity.IsMockLocation {
		result.AnomalyFlags = append(result.AnomalyFlags, AttendanceAnomalyMockLocation)
	}
	if !input.Integrity.IsGPSEnabled {
		result.AnomalyFlags = append(result.AnomalyFlags, AttendanceAnomalyGpsDisabled)
	}
	if input.Integrity.ClockSkewSeconds != 0 && absInt(input.Integrity.ClockSkewSeconds) > policy.MaxClockSkewSeconds {
		result.AnomalyFlags = append(result.AnomalyFlags, AttendanceAnomalyClockSkew)
	}

	if input.LastAcceptedPing != nil && policy.MaxPingInterval > 0 {
		seconds := input.CapturedAt.Sub(*input.LastAcceptedPing).Seconds()
		result.SecondsSinceLastPing = &seconds
		if input.CapturedAt.Sub(*input.LastAcceptedPing) > policy.MaxPingInterval {
			result.AnomalyFlags = append(result.AnomalyFlags, AttendanceAnomalyStalePing)
		}
	}

	if input.PreviousSample != nil && input.CapturedAt.After(input.PreviousSample.Timestamp) {
		speed := CalculateTravelSpeedKmh(
			input.PreviousSample.Latitude,
			input.PreviousSample.Longitude,
			input.PreviousSample.Timestamp,
			input.Latitude,
			input.Longitude,
			input.CapturedAt,
		)
		if speed != nil {
			result.ComputedSpeedKmh = speed
			if *speed > policy.MaxSpeedKmh {
				result.AnomalyFlags = append(result.AnomalyFlags, AttendanceAnomalyImpossibleSpeed)
			}
		}
	}

	if len(result.AnomalyFlags) > 0 {
		if policy.StrictEnforcement && containsCriticalAnomaly(result.AnomalyFlags) {
			result.ValidationStatus = models.AttendanceValidationRejected
			result.ValidationReason = buildValidationReason(result.AnomalyFlags)
		} else {
			result.ValidationStatus = models.AttendanceValidationFlagged
			result.ValidationReason = buildValidationReason(result.AnomalyFlags)
		}
	} else {
		result.ValidationReason = "validated"
	}

	return result, nil
}

func ParseGeofenceValue(geofenceJSON *string) (*Geofence, error) {
	if geofenceJSON == nil {
		return nil, nil
	}
	return ParseGeofence(*geofenceJSON)
}

func HaversineDistanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	latRad1 := lat1 * math.Pi / 180
	latRad2 := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(latRad1)*math.Cos(latRad2)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

func CalculateTravelSpeedKmh(lat1, lng1 float64, at1 time.Time, lat2, lng2 float64, at2 time.Time) *float64 {
	if at2.Before(at1) || at2.Equal(at1) {
		return nil
	}
	hours := at2.Sub(at1).Hours()
	if hours <= 0 {
		return nil
	}
	distanceKm := HaversineDistanceMeters(lat1, lng1, lat2, lng2) / 1000
	speed := distanceKm / hours
	return &speed
}

func containsCriticalAnomaly(flags []string) bool {
	critical := map[string]bool{
		AttendanceAnomalyOutsideBoundary: true,
		AttendanceAnomalyMockLocation:    true,
		AttendanceAnomalyGpsDisabled:     true,
		AttendanceAnomalyImpossibleSpeed: true,
	}

	for _, flag := range flags {
		if critical[flag] {
			return true
		}
	}

	return false
}

func buildValidationReason(flags []string) string {
	if len(flags) == 0 {
		return "validated"
	}
	return fmt.Sprintf("validation anomalies: %v", flags)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

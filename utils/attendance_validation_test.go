package utils

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"p9e.in/ugcl/models"
)

func TestValidateAttendanceInputAcceptsInsideGeofence(t *testing.T) {
	location := `{"lat":12.9716,"lng":77.5946,"address":"Site A"}`
	geofence := `{"coordinates":[{"lat":12.9710,"lng":77.5940},{"lat":12.9725,"lng":77.5940},{"lat":12.9725,"lng":77.5955},{"lat":12.9710,"lng":77.5955}]}`

	result, err := ValidateAttendanceInput(AttendanceValidationInput{
		Site:           models.Site{ID: uuid.New(), Location: &location, Geofence: &geofence},
		Latitude:       12.9718,
		Longitude:      77.5948,
		AccuracyMeters: 15,
		CapturedAt:     time.Now().UTC(),
		Integrity: DeviceIntegritySnapshot{
			IsGPSEnabled: true,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.InsideBoundary {
		t.Fatalf("expected point to be inside boundary")
	}
	if result.ValidationStatus != models.AttendanceValidationAccepted {
		t.Fatalf("expected accepted status, got %s", result.ValidationStatus)
	}
	if result.ValidationMethod != "geofence" {
		t.Fatalf("expected geofence validation method, got %s", result.ValidationMethod)
	}
}

func TestValidateAttendanceInputFlagsOutsideBoundaryByDefault(t *testing.T) {
	location := `{"lat":12.9716,"lng":77.5946,"address":"Site A"}`

	result, err := ValidateAttendanceInput(AttendanceValidationInput{
		Site:           models.Site{ID: uuid.New(), Location: &location},
		Latitude:       12.9816,
		Longitude:      77.6046,
		AccuracyMeters: 10,
		CapturedAt:     time.Now().UTC(),
		Integrity: DeviceIntegritySnapshot{
			IsGPSEnabled: true,
		},
		Policy: AttendanceValidationPolicy{AllowedRadiusMeters: 100},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ValidationStatus != models.AttendanceValidationFlagged {
		t.Fatalf("expected flagged status, got %s", result.ValidationStatus)
	}
	if result.DistanceFromSiteM == nil || *result.DistanceFromSiteM <= 100 {
		t.Fatalf("expected distance to exceed configured radius")
	}
	if !containsFlag(result.AnomalyFlags, AttendanceAnomalyOutsideBoundary) {
		t.Fatalf("expected outside boundary anomaly, got %v", result.AnomalyFlags)
	}
}

func TestValidateAttendanceInputFlagsStaleHeartbeat(t *testing.T) {
	location := `{"lat":12.9716,"lng":77.5946,"address":"Site A"}`
	lastPing := time.Now().UTC().Add(-10 * time.Minute)

	result, err := ValidateAttendanceInput(AttendanceValidationInput{
		Site:           models.Site{ID: uuid.New(), Location: &location},
		Latitude:       12.9717,
		Longitude:      77.5947,
		AccuracyMeters: 20,
		CapturedAt:     time.Now().UTC(),
		Integrity: DeviceIntegritySnapshot{
			IsGPSEnabled: true,
		},
		LastAcceptedPing: &lastPing,
		Policy: AttendanceValidationPolicy{
			AllowedRadiusMeters: 150,
			MaxPingInterval:     2 * time.Minute,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ValidationStatus != models.AttendanceValidationFlagged {
		t.Fatalf("expected flagged status, got %s", result.ValidationStatus)
	}
	if !containsFlag(result.AnomalyFlags, AttendanceAnomalyStalePing) {
		t.Fatalf("expected stale ping anomaly, got %v", result.AnomalyFlags)
	}
}

func TestValidateAttendanceInputFlagsImpossibleSpeedByDefault(t *testing.T) {
	location := `{"lat":12.9716,"lng":77.5946,"address":"Site A"}`
	previous := &LocationSample{
		Latitude:  12.9716,
		Longitude: 77.5946,
		Timestamp: time.Now().UTC().Add(-30 * time.Second),
	}

	result, err := ValidateAttendanceInput(AttendanceValidationInput{
		Site:           models.Site{ID: uuid.New(), Location: &location},
		Latitude:       13.0516,
		Longitude:      77.6946,
		AccuracyMeters: 10,
		CapturedAt:     time.Now().UTC(),
		Integrity: DeviceIntegritySnapshot{
			IsGPSEnabled: true,
		},
		PreviousSample: previous,
		Policy:         AttendanceValidationPolicy{AllowedRadiusMeters: 500000, MaxSpeedKmh: 80},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ValidationStatus != models.AttendanceValidationFlagged {
		t.Fatalf("expected flagged status, got %s", result.ValidationStatus)
	}
	if !containsFlag(result.AnomalyFlags, AttendanceAnomalyImpossibleSpeed) {
		t.Fatalf("expected impossible speed anomaly, got %v", result.AnomalyFlags)
	}
	if result.ComputedSpeedKmh == nil || *result.ComputedSpeedKmh <= 80 {
		t.Fatalf("expected computed speed greater than threshold")
	}
}

func containsFlag(flags []string, expected string) bool {
	for _, flag := range flags {
		if flag == expected {
			return true
		}
	}
	return false
}

func TestValidateAttendanceInputRejectsOutsideBoundaryWhenStrict(t *testing.T) {
	location := `{"lat":12.9716,"lng":77.5946,"address":"Site A"}`

	result, err := ValidateAttendanceInput(AttendanceValidationInput{
		Site:           models.Site{ID: uuid.New(), Location: &location},
		Latitude:       12.9816,
		Longitude:      77.6046,
		AccuracyMeters: 10,
		CapturedAt:     time.Now().UTC(),
		Integrity:      DeviceIntegritySnapshot{IsGPSEnabled: true},
		Policy: AttendanceValidationPolicy{
			AllowedRadiusMeters: 100,
			StrictEnforcement:   true,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ValidationStatus != models.AttendanceValidationRejected {
		t.Fatalf("expected rejected status, got %s", result.ValidationStatus)
	}
}

func TestValidateAttendanceInputRejectsImpossibleSpeedWhenStrict(t *testing.T) {
	location := `{"lat":12.9716,"lng":77.5946,"address":"Site A"}`
	previous := &LocationSample{
		Latitude:  12.9716,
		Longitude: 77.5946,
		Timestamp: time.Now().UTC().Add(-30 * time.Second),
	}

	result, err := ValidateAttendanceInput(AttendanceValidationInput{
		Site:           models.Site{ID: uuid.New(), Location: &location},
		Latitude:       13.0516,
		Longitude:      77.6946,
		AccuracyMeters: 10,
		CapturedAt:     time.Now().UTC(),
		Integrity:      DeviceIntegritySnapshot{IsGPSEnabled: true},
		PreviousSample: previous,
		Policy: AttendanceValidationPolicy{
			AllowedRadiusMeters: 500000,
			MaxSpeedKmh:         80,
			StrictEnforcement:   true,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ValidationStatus != models.AttendanceValidationRejected {
		t.Fatalf("expected rejected status, got %s", result.ValidationStatus)
	}
}

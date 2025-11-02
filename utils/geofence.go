package utils

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Coordinate represents a geographic coordinate with latitude and longitude
type Coordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Geofence represents a polygonal geofence boundary
type Geofence struct {
	Coordinates []Coordinate `json:"coordinates"`
	Name        string       `json:"name,omitempty"`        // Optional name for the geofence
	Description string       `json:"description,omitempty"` // Optional description
}

// ValidateGeofence validates geofencing data
func ValidateGeofence(geofenceJSON string) error {
	if geofenceJSON == "" {
		return nil // Geofence is optional
	}

	var geofence Geofence
	if err := json.Unmarshal([]byte(geofenceJSON), &geofence); err != nil {
		return fmt.Errorf("invalid geofence JSON format: %w", err)
	}

	// A valid polygon needs at least 3 points (triangle)
	if len(geofence.Coordinates) < 3 {
		return errors.New("geofence must have at least 3 coordinates to form a polygon")
	}

	// Validate each coordinate
	for i, coord := range geofence.Coordinates {
		if err := validateCoordinate(coord); err != nil {
			return fmt.Errorf("invalid coordinate at index %d: %w", i, err)
		}
	}

	// Check if the polygon is closed (first and last points should be the same)
	// If not, we'll allow it but it's recommended
	first := geofence.Coordinates[0]
	last := geofence.Coordinates[len(geofence.Coordinates)-1]
	if first.Lat != last.Lat || first.Lng != last.Lng {
		// For user convenience, we can auto-close the polygon by not returning an error
		// But you can uncomment the line below if you want to enforce closed polygons
		// return errors.New("geofence polygon must be closed (first and last coordinates must match)")
	}

	return nil
}

// validateCoordinate validates a single coordinate
func validateCoordinate(coord Coordinate) error {
	// Latitude must be between -90 and 90
	if coord.Lat < -90 || coord.Lat > 90 {
		return fmt.Errorf("latitude %.6f is out of valid range [-90, 90]", coord.Lat)
	}

	// Longitude must be between -180 and 180
	if coord.Lng < -180 || coord.Lng > 180 {
		return fmt.Errorf("longitude %.6f is out of valid range [-180, 180]", coord.Lng)
	}

	return nil
}

// IsPointInPolygon checks if a point is inside a polygon using ray casting algorithm
func IsPointInPolygon(point Coordinate, polygon []Coordinate) bool {
	if len(polygon) < 3 {
		return false
	}

	inside := false
	j := len(polygon) - 1

	for i := 0; i < len(polygon); i++ {
		xi, yi := polygon[i].Lng, polygon[i].Lat
		xj, yj := polygon[j].Lng, polygon[j].Lat

		intersect := ((yi > point.Lat) != (yj > point.Lat)) &&
			(point.Lng < (xj-xi)*(point.Lat-yi)/(yj-yi)+xi)

		if intersect {
			inside = !inside
		}
		j = i
	}

	return inside
}

// ParseGeofence parses geofence JSON string to Geofence struct
func ParseGeofence(geofenceJSON string) (*Geofence, error) {
	if geofenceJSON == "" {
		return nil, nil
	}

	var geofence Geofence
	if err := json.Unmarshal([]byte(geofenceJSON), &geofence); err != nil {
		return nil, fmt.Errorf("failed to parse geofence: %w", err)
	}

	return &geofence, nil
}

// CalculatePolygonCenter calculates the centroid of a polygon
func CalculatePolygonCenter(coordinates []Coordinate) Coordinate {
	if len(coordinates) == 0 {
		return Coordinate{}
	}

	var sumLat, sumLng float64
	for _, coord := range coordinates {
		sumLat += coord.Lat
		sumLng += coord.Lng
	}

	return Coordinate{
		Lat: sumLat / float64(len(coordinates)),
		Lng: sumLng / float64(len(coordinates)),
	}
}

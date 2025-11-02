# Site Geofencing Feature

## Overview
The geofencing feature allows you to define geographical boundaries for each site. This enables location-based access control and tracking within defined site perimeters.

## Data Model

### Site Model Updates
The `Site` model now includes a `Geofence` field:

```go
type Site struct {
    // ... existing fields ...
    Geofence *string `gorm:"type:jsonb" json:"geofence,omitempty"`
}
```

### Geofence Structure
The geofence data is stored as JSONB with the following structure:

```json
{
  "coordinates": [
    {"lat": 23.0225, "lng": 72.5714},
    {"lat": 23.0226, "lng": 72.5715},
    {"lat": 23.0227, "lng": 72.5716},
    {"lat": 23.0225, "lng": 72.5714}
  ],
  "name": "Main Site Boundary",
  "description": "Primary perimeter fence"
}
```

**Required fields:**
- `coordinates`: Array of at least 3 coordinate objects (to form a polygon)
  - Each coordinate must have `lat` (-90 to 90) and `lng` (-180 to 180)

**Optional fields:**
- `name`: Name for the geofence
- `description`: Description of the geofence area

## API Endpoints

### 1. Create Site with Geofencing
**POST** `/api/v1/admin/sites`

```json
{
  "name": "Solar Plant Site A",
  "code": "SOLAR_A",
  "description": "Main solar installation",
  "businessVerticalId": "uuid-here",
  "location": "{\"lat\": 23.0225, \"lng\": 72.5714, \"address\": \"...\"}",
  "geofence": "{\"coordinates\": [{\"lat\": 23.0225, \"lng\": 72.5714}, {\"lat\": 23.0226, \"lng\": 72.5715}, {\"lat\": 23.0227, \"lng\": 72.5716}, {\"lat\": 23.0225, \"lng\": 72.5714}], \"name\": \"Site Perimeter\"}"
}
```

**Response:**
```json
{
  "id": "uuid",
  "name": "Solar Plant Site A",
  "code": "SOLAR_A",
  "geofence": "{\"coordinates\": [...], \"name\": \"Site Perimeter\"}",
  "createdAt": "2025-10-30T...",
  ...
}
```

### 2. Update Site Geofencing
**PUT** `/api/v1/admin/sites/{siteId}`

```json
{
  "geofence": "{\"coordinates\": [{\"lat\": 23.0225, \"lng\": 72.5714}, {\"lat\": 23.0226, \"lng\": 72.5715}, {\"lat\": 23.0227, \"lng\": 72.5716}, {\"lat\": 23.0225, \"lng\": 72.5714}], \"name\": \"Updated Boundary\"}"
}
```

### 3. Get Sites (with Geofencing)
All existing GET endpoints now return geofencing data:

- **GET** `/api/v1/admin/sites` - All sites
- **GET** `/api/v1/admin/sites/{siteId}` - Specific site
- **GET** `/api/v1/business/{businessCode}/sites` - Business sites
- **GET** `/api/v1/business/{businessCode}/sites/my-access` - User's accessible sites

## Validation Rules

The geofencing validation includes:

1. **Minimum Coordinates**: At least 3 points required (triangle)
2. **Latitude Range**: -90 to 90 degrees
3. **Longitude Range**: -180 to 180 degrees
4. **Valid JSON**: Properly formatted JSON structure
5. **Polygon Closure**: First and last points should match (recommended but not enforced)

## Helper Functions

### Validate Geofence
```go
err := utils.ValidateGeofence(geofenceJSON)
```

### Check Point in Polygon
```go
point := utils.Coordinate{Lat: 23.0225, Lng: 72.5714}
isInside := utils.IsPointInPolygon(point, geofence.Coordinates)
```

### Calculate Polygon Center
```go
center := utils.CalculatePolygonCenter(geofence.Coordinates)
```

### Parse Geofence
```go
geofence, err := utils.ParseGeofence(geofenceJSON)
```

## Database Migration

To add the geofencing field to existing databases:

```bash
# Apply migration
migrate -path migrations -database "postgresql://..." up

# Rollback if needed
migrate -path migrations -database "postgresql://..." down 1
```

Migration files:
- `000014_add_geofence_to_sites.up.sql`
- `000014_add_geofence_to_sites.down.sql`

## Use Cases

1. **Access Control**: Verify user is within site boundary before allowing operations
2. **Attendance Tracking**: Track when employees enter/exit site boundaries
3. **Asset Management**: Ensure assets remain within designated areas
4. **Compliance**: Meet regulatory requirements for site boundaries
5. **Analytics**: Generate reports on site coverage and boundary violations

## Example Usage in Frontend

```javascript
// Creating a site with geofencing
const siteData = {
  name: "Solar Plant A",
  code: "SOLAR_A",
  businessVerticalId: "uuid",
  geofence: JSON.stringify({
    coordinates: [
      {lat: 23.0225, lng: 72.5714},
      {lat: 23.0226, lng: 72.5715},
      {lat: 23.0227, lng: 72.5716},
      {lat: 23.0225, lng: 72.5714}
    ],
    name: "Main Perimeter"
  })
};

fetch('/api/v1/admin/sites', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify(siteData)
});
```

## Map Integration

The geofencing coordinates can be directly used with popular mapping libraries:

### Google Maps
```javascript
const polygon = new google.maps.Polygon({
  paths: geofence.coordinates.map(c => ({lat: c.lat, lng: c.lng})),
  strokeColor: '#FF0000',
  fillColor: '#FF0000'
});
```

### Leaflet
```javascript
const polygon = L.polygon(
  geofence.coordinates.map(c => [c.lat, c.lng])
).addTo(map);
```

## Security Considerations

1. Only users with `admin_all` permission can create/update geofencing
2. Geofence data is validated before saving to prevent invalid coordinates
3. JSONB storage with indexing for efficient queries
4. Business context ensures users only see relevant site boundaries

## Future Enhancements

- Multiple geofences per site (for complex sites)
- Geofence alerts and notifications
- Historical geofence tracking
- Automatic geofence generation from site location
- Integration with mobile device GPS for real-time tracking

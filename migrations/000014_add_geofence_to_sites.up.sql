-- Add geofence column to sites table
-- Stores polygon coordinates as JSONB array of {lat, lng} objects
ALTER TABLE sites ADD COLUMN IF NOT EXISTS geofence JSONB;

-- Add comment to explain the field
COMMENT ON COLUMN sites.geofence IS 'Geofencing polygon coordinates stored as array of {lat, lng} objects';

-- Create an index on geofence for better query performance
CREATE INDEX IF NOT EXISTS idx_sites_geofence ON sites USING GIN (geofence);

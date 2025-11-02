-- Rollback: Remove geofence column from sites table
DROP INDEX IF EXISTS idx_sites_geofence;
ALTER TABLE sites DROP COLUMN IF EXISTS geofence;

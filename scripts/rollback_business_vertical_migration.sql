-- Rollback script for the failed business_vertical_id migration
-- Run this script to clean up the partial migration, then restart your app

-- Remove the failed migration entry from the migrations table
DELETE FROM migrations WHERE id = '20102025_add_business_vertical_id_to_operational_tables';

-- Drop the business_vertical_id column from all tables (if it exists)
ALTER TABLE waters DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE dpr_sites DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE materials DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE payments DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE wrappings DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE eways DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE dairy_sites DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE diesels DROP COLUMN IF EXISTS business_vertical_id CASCADE;
ALTER TABLE stocks DROP COLUMN IF EXISTS business_vertical_id CASCADE;

-- Now you can restart your Go application and the migration will run fresh

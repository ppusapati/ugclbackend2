-- Migration: Drop old Form tables after migration to AppForm

-- Drop the many-to-many join table first
DROP TABLE IF EXISTS business_vertical_forms CASCADE;

-- Drop the old forms table
DROP TABLE IF EXISTS forms CASCADE;

-- Add comment
COMMENT ON TABLE app_forms IS 'Unified form configuration system (migrated from old forms table)';

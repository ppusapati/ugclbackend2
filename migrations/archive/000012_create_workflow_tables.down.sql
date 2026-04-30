-- Migration rollback: Drop workflow system tables

-- Drop tables in reverse order (respecting foreign key constraints)
DROP TABLE IF EXISTS workflow_transitions CASCADE;
DROP TABLE IF EXISTS form_submissions CASCADE;
DROP TABLE IF EXISTS workflow_definitions CASCADE;

-- Remove workflow_id column from app_forms if it was added
ALTER TABLE app_forms DROP COLUMN IF EXISTS workflow_id CASCADE;

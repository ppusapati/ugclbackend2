-- Migration: Create unified form configuration (inspired by v2 formbuilder)
-- This creates a simpler, more maintainable form system with JSON definitions

-- =============================================================================
-- Modules table - makes modules data-driven instead of hardcoded
-- =============================================================================
CREATE TABLE IF NOT EXISTS modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,           -- 'project', 'hr', 'finance', 'inventory', 'vehicle'
    name VARCHAR(100) NOT NULL,                  -- 'Projects', 'Human Resources'
    description TEXT,
    icon VARCHAR(50),                            -- 'work', 'people', 'attach_money'
    route VARCHAR(200),                          -- '/projects', '/hr'
    display_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- =============================================================================
-- Forms table (unified with vertical access) - inspired by v2 formbuilder
-- =============================================================================
CREATE TABLE IF NOT EXISTS app_forms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Basic info
    code VARCHAR(50) UNIQUE NOT NULL,            -- 'water', 'hr_nmr', 'contractor'
    title VARCHAR(255) NOT NULL,                 -- 'Water Form', 'NMR Form'
    description TEXT,
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',

    -- Module association
    module_id UUID NOT NULL REFERENCES modules(id),

    -- Navigation
    route VARCHAR(200) NOT NULL,                 -- '/projects/water', '/hr/nmr'
    icon VARCHAR(50),                            -- 'water_drop', 'people'
    display_order INT DEFAULT 0,

    -- Access control
    required_permission VARCHAR(100),            -- 'project:create', 'hr:create'
    allowed_roles JSONB DEFAULT '[]'::jsonb,     -- Legacy role support
    accessible_verticals JSONB DEFAULT '[]'::jsonb,  -- ["WATER", "HO"] - Direct vertical access in same table!

    -- Form definition (JSON-based like v2 formbuilder)
    form_schema JSONB DEFAULT '{}'::jsonb,       -- Complete form definition
    steps JSONB DEFAULT '[]'::jsonb,             -- Multi-step forms
    core_fields JSONB DEFAULT '[]'::jsonb,       -- Field definitions
    validations JSONB DEFAULT '{}'::jsonb,       -- Validation rules
    dependencies JSONB DEFAULT '[]'::jsonb,      -- Field dependencies

    -- Workflow integration
    workflow_id UUID,                            -- Link to workflow if needed
    initial_state VARCHAR(100) DEFAULT 'draft',

    -- Database table mapping (for v2 formbuilder integration)
    table_name VARCHAR(255),                     -- If form stores data in dedicated table
    schema_version INT DEFAULT 1,

    -- Metadata
    is_active BOOLEAN DEFAULT true,
    audit BOOLEAN DEFAULT false,
    created_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- =============================================================================
-- Create indexes
-- =============================================================================
CREATE INDEX idx_modules_code ON modules(code);
CREATE INDEX idx_modules_active ON modules(is_active);
CREATE INDEX idx_modules_order ON modules(display_order);

CREATE INDEX idx_app_forms_code ON app_forms(code);
CREATE INDEX idx_app_forms_module_id ON app_forms(module_id);
CREATE INDEX idx_app_forms_active ON app_forms(is_active);
CREATE INDEX idx_app_forms_verticals ON app_forms USING GIN(accessible_verticals);
CREATE INDEX idx_app_forms_module_order ON app_forms(module_id, display_order);

-- =============================================================================
-- Seed modules
-- =============================================================================
INSERT INTO modules (code, name, description, icon, route, display_order) VALUES
('project', 'Projects', 'Project management and execution', 'work', '/projects', 1),
('hr', 'Human Resources', 'HR and workforce management', 'people', '/hr', 2),
('finance', 'Finance', 'Financial operations and payments', 'attach_money', '/finance', 3),
('inventory', 'Inventory', 'Inventory and stock management', 'inventory', '/inventory', 4),
('vehicle', 'Vehicles', 'Vehicle tracking and logs', 'directions_car', '/vehicles', 5)
ON CONFLICT (code) DO NOTHING;

-- =============================================================================
-- Seed forms with vertical access in same table
-- =============================================================================
DO $$
DECLARE
    project_module_id UUID;
    hr_module_id UUID;
    finance_module_id UUID;
    inventory_module_id UUID;
    vehicle_module_id UUID;
BEGIN
    -- Get module IDs
    SELECT id INTO project_module_id FROM modules WHERE code = 'project';
    SELECT id INTO hr_module_id FROM modules WHERE code = 'hr';
    SELECT id INTO finance_module_id FROM modules WHERE code = 'finance';
    SELECT id INTO inventory_module_id FROM modules WHERE code = 'inventory';
    SELECT id INTO vehicle_module_id FROM modules WHERE code = 'vehicle';

    -- Project forms
    INSERT INTO app_forms (code, title, description, module_id, route, icon, required_permission, accessible_verticals, display_order) VALUES
    ('water', 'Water', 'Record daily supply of water', project_module_id, '/projects/water', 'water_drop', 'project:create', '["WATER", "HO"]'::jsonb, 1),
    ('painting', 'Painting', 'Report pipe painting works', project_module_id, '/projects/painting', 'brush', 'project:create', '["WATER", "HO"]'::jsonb, 2),
    ('wrapping', 'Wrapping', 'Record pipe wrapping activity', project_module_id, '/projects/wrapping', 'wrap_text', 'project:create', '["WATER", "HO"]'::jsonb, 3),
    ('contractor', 'Contractor Form', 'Record daily progress of work done', project_module_id, '/projects/contractor', 'construction', 'project:create', '["WATER", "CONTRACTORS", "HO"]'::jsonb, 4),
    ('site_dpr', 'DPR Site', 'Manage your bills', project_module_id, '/projects/site', 'location_on', 'project:create', '["WATER", "CONTRACTORS", "HO"]'::jsonb, 5),
    ('dairy_site', 'Site Dairy', 'Record all activities for the day', project_module_id, '/projects/dairySite', 'task', 'project:create', '["WATER", "HO"]'::jsonb, 6),
    ('tasks', 'Tasks', 'Record daily assigned tasks', project_module_id, '/projects/tasks', 'task_sharp', 'project:assign', '["WATER", "CONTRACTORS", "HO"]'::jsonb, 7)
    ON CONFLICT (code) DO NOTHING;

    -- HR forms (shared between WATER, SOLAR, HO)
    INSERT INTO app_forms (code, title, description, module_id, route, icon, required_permission, accessible_verticals, display_order) VALUES
    ('hr_nmr', 'NMR Form', 'Report daily worker deployment', hr_module_id, '/hr/nmr', 'supervised_user_circle', 'hr:create', '["WATER", "SOLAR", "HO"]'::jsonb, 1)
    ON CONFLICT (code) DO NOTHING;

    -- Finance forms
    INSERT INTO app_forms (code, title, description, module_id, route, icon, required_permission, accessible_verticals, display_order) VALUES
    ('finance_payment', 'Payments', 'Request payment or advance', finance_module_id, '/finance/payments', 'payment', 'finance:create', '["WATER", "HO"]'::jsonb, 1)
    ON CONFLICT (code) DO NOTHING;

    -- Inventory forms
    INSERT INTO app_forms (code, title, description, module_id, route, icon, required_permission, accessible_verticals, display_order) VALUES
    ('inventory_material', 'Material', 'Request materials', inventory_module_id, '/inventory/material', 'receipt', 'inventory:create', '["WATER", "HO"]'::jsonb, 1),
    ('inventory_stock', 'Stock Registry', 'Record stocks', inventory_module_id, '/inventory/stock', 'storage', 'inventory:create', '["WATER", "HO"]'::jsonb, 2),
    ('inventory_diesel', 'Diesel', 'Log diesel usage', inventory_module_id, '/inventory/diesel', 'local_gas_station', 'inventory:create', '["WATER", "HO"]'::jsonb, 3),
    ('inventory_eway', 'Eway-bills', 'Log all stock movement', inventory_module_id, '/inventory/eway', 'receipt_long', 'inventory:create', '["WATER", "HO"]'::jsonb, 4)
    ON CONFLICT (code) DO NOTHING;

    -- Vehicle forms (shared between WATER, SOLAR, HO)
    INSERT INTO app_forms (code, title, description, module_id, route, icon, required_permission, accessible_verticals, display_order) VALUES
    ('vehicle_nmr', 'NMR Vehicle', 'Record daily vehicle deployment', vehicle_module_id, '/vehicles/nmrVehicle', 'car_repair', 'hr:create', '["WATER", "SOLAR", "HO"]'::jsonb, 1),
    ('vehicle_log', 'Vehicle Log', 'Log daily vehicle usage details', vehicle_module_id, '/vehicles/vehicleLog', 'history', 'hr:create', '["WATER", "SOLAR", "HO"]'::jsonb, 2)
    ON CONFLICT (code) DO NOTHING;
END $$;

-- =============================================================================
-- Example: How to add form schema (JSON-based form definition)
-- =============================================================================
-- UPDATE app_forms
-- SET form_schema = '{
--   "type": "form",
--   "fields": [
--     {"id": "site", "type": "dropdown", "label": "Site Name", "required": true, "dataSource": "sites"},
--     {"id": "date", "type": "date", "label": "Date", "required": true},
--     {"id": "quantity", "type": "number", "label": "Quantity", "required": true}
--   ]
-- }'::jsonb
-- WHERE code = 'water';

-- Migration: Create forms configuration tables
-- This enables database-driven form visibility per business vertical

-- 1. Forms master table
CREATE TABLE IF NOT EXISTS forms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,           -- Unique identifier: 'water', 'hr_nmr', 'contractor', etc.
    name VARCHAR(100) NOT NULL,                  -- Display name: 'Water Form', 'NMR Form'
    description TEXT,                            -- Subtitle/description
    module VARCHAR(50) NOT NULL,                 -- Module: 'project', 'hr', 'finance', 'inventory', 'vehicle'
    route VARCHAR(200) NOT NULL,                 -- App route: '/projects/water', '/hr/nmr'
    icon VARCHAR(50),                            -- Icon name: 'water_drop', 'people'
    required_permission VARCHAR(100),            -- Permission required: 'project:create', 'hr:create'
    is_active BOOLEAN DEFAULT true,
    display_order INT DEFAULT 0,                 -- Order in which to display forms
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 2. Business vertical to form mapping (many-to-many)
CREATE TABLE IF NOT EXISTS business_vertical_forms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_vertical_id UUID NOT NULL REFERENCES business_verticals(id) ON DELETE CASCADE,
    form_id UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(business_vertical_id, form_id)
);

-- Create indexes for performance
CREATE INDEX idx_forms_module ON forms(module);
CREATE INDEX idx_forms_code ON forms(code);
CREATE INDEX idx_business_vertical_forms_vertical ON business_vertical_forms(business_vertical_id);
CREATE INDEX idx_business_vertical_forms_form ON business_vertical_forms(form_id);

-- Seed forms data
INSERT INTO forms (code, name, description, module, route, icon, required_permission, display_order) VALUES
-- Project forms
('water', 'Water', 'Record daily supply of water', 'project', '/projects/water', 'water_drop', 'project:create', 1),
('painting', 'Painting', 'Report pipe painting works', 'project', '/projects/painting', 'brush', 'project:create', 2),
('wrapping', 'Wrapping', 'Record pipe wrapping activity', 'project', '/projects/wrapping', 'wrap_text', 'project:create', 3),
('contractor', 'Contractor Form', 'Record daily progress of work done', 'project', '/projects/contractor', 'construction', 'project:create', 4),
('site_dpr', 'DPR Site', 'Manage your bills', 'project', '/projects/site', 'location_on', 'project:create', 5),
('dairy_site', 'Site Dairy', 'Record all the activities for the day', 'project', '/projects/dairySite', 'task', 'project:create', 6),
('tasks', 'Tasks', 'Record daily assigned tasks', 'project', '/projects/tasks', 'task_sharp', 'project:assign', 7),

-- HR forms
('hr_nmr', 'NMR Form', 'Report daily worker deployment', 'hr', '/hr/nmr', 'supervised_user_circle', 'hr:create', 1),

-- Finance forms
('finance_payment', 'Payments', 'Request Payment or advance', 'finance', '/finance/payments', 'payment', 'finance:create', 1),

-- Inventory forms
('inventory_material', 'Material', 'Request Materials', 'inventory', '/inventory/material', 'receipt', 'inventory:create', 1),
('inventory_stock', 'Stock Registry', 'Record stocks', 'inventory', '/inventory/stock', 'storage', 'inventory:create', 2),
('inventory_diesel', 'Diesel', 'Log diesel usage', 'inventory', '/inventory/diesel', 'local_gas_station', 'inventory:create', 3),
('inventory_eway', 'Eway-bills', 'Log all stock movement', 'inventory', '/inventory/eway', 'receipt_long', 'inventory:create', 4),

-- Vehicle forms
('vehicle_nmr', 'NMR Vehicle', 'Record daily vehicle deployment', 'vehicle', '/vehicles/nmrVehicle', 'car_repair', 'hr:create', 1),
('vehicle_log', 'Vehicle Log', 'Log daily Vehicle usage details', 'vehicle', '/vehicles/vehicleLog', 'history', 'hr:create', 2)
ON CONFLICT (code) DO NOTHING;

-- Map forms to business verticals
-- Note: Run this after business verticals are created
DO $$
DECLARE
    water_id UUID;
    solar_id UUID;
    ho_id UUID;
    contractors_id UUID;
BEGIN
    -- Get business vertical IDs
    SELECT id INTO water_id FROM business_verticals WHERE code = 'WATER';
    SELECT id INTO solar_id FROM business_verticals WHERE code = 'SOLAR';
    SELECT id INTO ho_id FROM business_verticals WHERE code = 'HO';
    SELECT id INTO contractors_id FROM business_verticals WHERE code = 'CONTRACTORS';

    -- WATER: All forms
    INSERT INTO business_vertical_forms (business_vertical_id, form_id)
    SELECT water_id, id FROM forms
    ON CONFLICT (business_vertical_id, form_id) DO NOTHING;

    -- SOLAR: Only hr_nmr, vehicle_nmr, vehicle_log
    INSERT INTO business_vertical_forms (business_vertical_id, form_id)
    SELECT solar_id, id FROM forms WHERE code IN ('hr_nmr', 'vehicle_nmr', 'vehicle_log')
    ON CONFLICT (business_vertical_id, form_id) DO NOTHING;

    -- CONTRACTORS: Only contractor, site_dpr, tasks
    INSERT INTO business_vertical_forms (business_vertical_id, form_id)
    SELECT contractors_id, id FROM forms WHERE code IN ('contractor', 'site_dpr', 'tasks')
    ON CONFLICT (business_vertical_id, form_id) DO NOTHING;

    -- HEAD_OFFICE: All forms
    INSERT INTO business_vertical_forms (business_vertical_id, form_id)
    SELECT ho_id, id FROM forms
    ON CONFLICT (business_vertical_id, form_id) DO NOTHING;
END $$;

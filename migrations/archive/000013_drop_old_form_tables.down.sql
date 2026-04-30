-- Migration rollback: Recreate old Form tables
-- WARNING: This will recreate empty tables - data cannot be automatically restored

-- Recreate forms table
CREATE TABLE IF NOT EXISTS forms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    module VARCHAR(50) NOT NULL,
    route VARCHAR(200) NOT NULL,
    icon VARCHAR(50),
    required_permission VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Recreate business_vertical_forms join table
CREATE TABLE IF NOT EXISTS business_vertical_forms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_vertical_id UUID NOT NULL REFERENCES business_verticals(id) ON DELETE CASCADE,
    form_id UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(business_vertical_id, form_id)
);

-- Recreate indexes
CREATE INDEX idx_forms_code ON forms(code);
CREATE INDEX idx_forms_module ON forms(module);
CREATE INDEX idx_forms_is_active ON forms(is_active);
CREATE INDEX idx_business_vertical_forms_vertical ON business_vertical_forms(business_vertical_id);
CREATE INDEX idx_business_vertical_forms_form ON business_vertical_forms(form_id);

-- Phase 1 Project Management Controls
-- Adds WBS, task dependencies, BOQ, MB, and RA billing tables.

CREATE TABLE IF NOT EXISTS wbs_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES wbs_nodes(id) ON DELETE SET NULL,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    node_type VARCHAR(32) NOT NULL DEFAULT 'activity',
    sort_order INTEGER NOT NULL DEFAULT 0,
    planned_start_date TIMESTAMP,
    planned_end_date TIMESTAMP,
    actual_start_date TIMESTAMP,
    actual_end_date TIMESTAMP,
    progress DECIMAL(5,2) DEFAULT 0,
    weightage DECIMAL(5,2) DEFAULT 0,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT chk_wbs_node_type_phase1_sql CHECK (node_type IN ('package', 'activity', 'milestone'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wbs_nodes_project_code ON wbs_nodes(project_id, code);
CREATE INDEX IF NOT EXISTS idx_wbs_nodes_parent ON wbs_nodes(parent_id);
CREATE INDEX IF NOT EXISTS idx_wbs_nodes_project_sort ON wbs_nodes(project_id, sort_order);

CREATE TABLE IF NOT EXISTS task_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    predecessor_task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    successor_task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    dependency_type VARCHAR(8) NOT NULL DEFAULT 'FS',
    lag_days INTEGER NOT NULL DEFAULT 0,
    notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_task_dep_type_phase1_sql CHECK (dependency_type IN ('FS', 'SS', 'FF', 'SF')),
    CONSTRAINT chk_task_dep_not_self_sql CHECK (predecessor_task_id <> successor_task_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_task_dependencies_unique_pair ON task_dependencies(project_id, predecessor_task_id, successor_task_id);
CREATE INDEX IF NOT EXISTS idx_task_dependencies_successor ON task_dependencies(successor_task_id);

CREATE TABLE IF NOT EXISTS boq_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    wbs_node_id UUID REFERENCES wbs_nodes(id) ON DELETE SET NULL,
    code VARCHAR(64) NOT NULL,
    description TEXT NOT NULL,
    uom VARCHAR(32) NOT NULL,
    planned_quantity DECIMAL(15,4) NOT NULL DEFAULT 0,
    executed_quantity DECIMAL(15,4) NOT NULL DEFAULT 0,
    billed_quantity DECIMAL(15,4) NOT NULL DEFAULT 0,
    unit_rate DECIMAL(15,2) NOT NULL DEFAULT 0,
    planned_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'planned',
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT chk_boq_status_phase1_sql CHECK (status IN ('planned', 'in-progress', 'completed', 'cancelled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_boq_items_project_code ON boq_items(project_id, code);
CREATE INDEX IF NOT EXISTS idx_boq_items_status ON boq_items(status);

CREATE TABLE IF NOT EXISTS mb_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    boq_item_id UUID NOT NULL REFERENCES boq_items(id) ON DELETE RESTRICT,
    entry_number VARCHAR(64) NOT NULL,
    measurement_date TIMESTAMP NOT NULL,
    measured_qty DECIMAL(15,4) NOT NULL,
    rate DECIMAL(15,2) NOT NULL DEFAULT 0,
    amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    location_ref VARCHAR(255),
    remarks TEXT,
    recorded_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mb_entries_project_entry_number ON mb_entries(project_id, entry_number);
CREATE INDEX IF NOT EXISTS idx_mb_entries_boq_item ON mb_entries(boq_item_id);

CREATE TABLE IF NOT EXISTS ra_bills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    bill_number VARCHAR(64) NOT NULL,
    period_start TIMESTAMP,
    period_end TIMESTAMP,
    gross_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    deductions_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    retention_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    net_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    submitted_by VARCHAR(255),
    submitted_at TIMESTAMP,
    approved_by VARCHAR(255),
    approved_at TIMESTAMP,
    payment_reference VARCHAR(255),
    notes TEXT,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT chk_ra_bill_status_phase1_sql CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'paid'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ra_bills_project_bill_number ON ra_bills(project_id, bill_number);
CREATE INDEX IF NOT EXISTS idx_ra_bills_status ON ra_bills(status);

CREATE TABLE IF NOT EXISTS ra_bill_lines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ra_bill_id UUID NOT NULL REFERENCES ra_bills(id) ON DELETE CASCADE,
    boq_item_id UUID NOT NULL REFERENCES boq_items(id) ON DELETE RESTRICT,
    mb_entry_id UUID REFERENCES mb_entries(id) ON DELETE SET NULL,
    quantity DECIMAL(15,4) NOT NULL,
    rate DECIMAL(15,2) NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    line_remark TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ra_bill_lines_bill_id ON ra_bill_lines(ra_bill_id);
CREATE INDEX IF NOT EXISTS idx_ra_bill_lines_boq_item_id ON ra_bill_lines(boq_item_id);

-- Migration: Create Project Management Tables with PostGIS Support
-- Description: Creates tables for project management with KMZ file handling, zones, nodes, tasks, and budget tracking
-- Author: Claude Code
-- Date: 2025-10-25

-- Enable PostGIS extension if not already enabled
CREATE EXTENSION IF NOT EXISTS postgis;

-- =====================================================
-- Project Roles Table
-- =====================================================
CREATE TABLE IF NOT EXISTS project_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    permissions JSONB DEFAULT '[]'::jsonb,
    level INTEGER DEFAULT 0,
    parent_role_id UUID REFERENCES project_roles(id) ON DELETE SET NULL,
    is_active BOOLEAN DEFAULT true,
    is_system_role BOOLEAN DEFAULT false,
    created_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_project_roles_code ON project_roles(code);
CREATE INDEX idx_project_roles_is_active ON project_roles(is_active);
CREATE INDEX idx_project_roles_parent ON project_roles(parent_role_id);

-- =====================================================
-- Projects Table
-- =====================================================
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    business_vertical_id UUID NOT NULL REFERENCES business_verticals(id) ON DELETE RESTRICT,

    -- KMZ File information
    kmz_file_name VARCHAR(255),
    kmz_file_path VARCHAR(500),
    kmz_uploaded_at TIMESTAMP,

    -- GeoJSON data
    geojson_data JSONB DEFAULT '{}'::jsonb,

    -- Timeline
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    actual_start_date TIMESTAMP,
    actual_end_date TIMESTAMP,

    -- Budget
    total_budget DECIMAL(15,2) DEFAULT 0,
    allocated_budget DECIMAL(15,2) DEFAULT 0,
    spent_budget DECIMAL(15,2) DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'INR',

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    progress DECIMAL(5,2) DEFAULT 0,

    -- Workflow
    workflow_id UUID REFERENCES workflow_definitions(id) ON DELETE SET NULL,

    -- Metadata
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT chk_project_status CHECK (status IN ('draft', 'active', 'on-hold', 'completed', 'cancelled')),
    CONSTRAINT chk_project_progress CHECK (progress >= 0 AND progress <= 100)
);

CREATE INDEX idx_projects_code ON projects(code);
CREATE INDEX idx_projects_business_vertical ON projects(business_vertical_id);
CREATE INDEX idx_projects_status ON projects(status);
CREATE INDEX idx_projects_deleted_at ON projects(deleted_at);
CREATE INDEX idx_projects_workflow ON projects(workflow_id);

-- =====================================================
-- Zones Table (Geographic zones within projects)
-- =====================================================
CREATE TABLE IF NOT EXISTS zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    description TEXT,
    label VARCHAR(255),

    -- PostGIS geometry columns
    geometry GEOMETRY(Geometry, 4326),
    centroid GEOMETRY(Point, 4326),
    area DECIMAL(15,2), -- in square meters

    -- GeoJSON representation
    geojson JSONB DEFAULT '{}'::jsonb,

    -- Additional properties from KMZ
    properties JSONB DEFAULT '{}'::jsonb,

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_zones_project ON zones(project_id);
CREATE INDEX idx_zones_deleted_at ON zones(deleted_at);
CREATE INDEX idx_zones_geometry ON zones USING GIST(geometry);
CREATE INDEX idx_zones_centroid ON zones USING GIST(centroid);

-- =====================================================
-- Nodes Table (Points/nodes within zones)
-- =====================================================
CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id UUID NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    description TEXT,
    label VARCHAR(255),
    node_type VARCHAR(50) NOT NULL,

    -- PostGIS location
    location GEOMETRY(Point, 4326) NOT NULL,
    latitude DECIMAL(10,8),
    longitude DECIMAL(11,8),
    elevation DECIMAL(10,2),

    -- GeoJSON representation
    geojson JSONB DEFAULT '{}'::jsonb,

    -- Additional properties
    properties JSONB DEFAULT '{}'::jsonb,

    -- Status
    status VARCHAR(50) DEFAULT 'available',

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT chk_node_type CHECK (node_type IN ('start', 'stop', 'waypoint')),
    CONSTRAINT chk_node_status CHECK (status IN ('available', 'allocated', 'in-progress', 'completed'))
);

CREATE INDEX idx_nodes_zone ON nodes(zone_id);
CREATE INDEX idx_nodes_project ON nodes(project_id);
CREATE INDEX idx_nodes_type ON nodes(node_type);
CREATE INDEX idx_nodes_status ON nodes(status);
CREATE INDEX idx_nodes_deleted_at ON nodes(deleted_at);
CREATE INDEX idx_nodes_location ON nodes USING GIST(location);

-- =====================================================
-- Tasks Table
-- =====================================================
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,

    -- Project context
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    zone_id UUID REFERENCES zones(id) ON DELETE SET NULL,

    -- Node references
    start_node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
    stop_node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,

    -- Timeline
    planned_start_date TIMESTAMP,
    planned_end_date TIMESTAMP,
    actual_start_date TIMESTAMP,
    actual_end_date TIMESTAMP,

    -- Budget
    allocated_budget DECIMAL(15,2) DEFAULT 0,
    labor_cost DECIMAL(15,2) DEFAULT 0,
    material_cost DECIMAL(15,2) DEFAULT 0,
    equipment_cost DECIMAL(15,2) DEFAULT 0,
    other_cost DECIMAL(15,2) DEFAULT 0,
    total_cost DECIMAL(15,2) DEFAULT 0,

    -- Status and progress
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    progress DECIMAL(5,2) DEFAULT 0,
    priority VARCHAR(20) DEFAULT 'medium',

    -- Workflow integration
    workflow_id UUID REFERENCES workflow_definitions(id) ON DELETE SET NULL,
    current_state VARCHAR(50),

    -- Form submission
    form_submission_id UUID REFERENCES form_submissions(id) ON DELETE SET NULL,

    -- Additional data
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Metadata
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT chk_task_status CHECK (status IN ('pending', 'assigned', 'in-progress', 'on-hold', 'completed', 'cancelled')),
    CONSTRAINT chk_task_priority CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT chk_task_progress CHECK (progress >= 0 AND progress <= 100)
);

CREATE INDEX idx_tasks_code ON tasks(code);
CREATE INDEX idx_tasks_project ON tasks(project_id);
CREATE INDEX idx_tasks_zone ON tasks(zone_id);
CREATE INDEX idx_tasks_start_node ON tasks(start_node_id);
CREATE INDEX idx_tasks_stop_node ON tasks(stop_node_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_priority ON tasks(priority);
CREATE INDEX idx_tasks_current_state ON tasks(current_state);
CREATE INDEX idx_tasks_deleted_at ON tasks(deleted_at);

-- =====================================================
-- Task Assignments Table
-- =====================================================
CREATE TABLE IF NOT EXISTS task_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    -- User assignment
    user_id VARCHAR(255) NOT NULL,
    user_name VARCHAR(255),
    user_type VARCHAR(50) NOT NULL,
    role VARCHAR(50) NOT NULL,

    -- Assignment details
    assigned_by VARCHAR(255) NOT NULL,
    assigned_at TIMESTAMP NOT NULL,
    start_date TIMESTAMP,
    end_date TIMESTAMP,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    is_active BOOLEAN DEFAULT true,

    -- Permissions
    can_edit BOOLEAN DEFAULT false,
    can_approve BOOLEAN DEFAULT false,

    -- Metadata
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT chk_assignment_user_type CHECK (user_type IN ('employee', 'contractor', 'supervisor')),
    CONSTRAINT chk_assignment_role CHECK (role IN ('worker', 'supervisor', 'manager', 'approver')),
    CONSTRAINT chk_assignment_status CHECK (status IN ('active', 'inactive', 'completed'))
);

CREATE INDEX idx_task_assignments_task ON task_assignments(task_id);
CREATE INDEX idx_task_assignments_user ON task_assignments(user_id);
CREATE INDEX idx_task_assignments_user_type ON task_assignments(user_type);
CREATE INDEX idx_task_assignments_status ON task_assignments(status);
CREATE INDEX idx_task_assignments_deleted_at ON task_assignments(deleted_at);

-- =====================================================
-- Budget Allocations Table
-- =====================================================
CREATE TABLE IF NOT EXISTS budget_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Project or Task reference
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,

    -- Budget details
    category VARCHAR(50) NOT NULL,
    description TEXT,
    planned_amount DECIMAL(15,2) NOT NULL,
    actual_amount DECIMAL(15,2) DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'INR',

    -- Timeline
    allocation_date TIMESTAMP NOT NULL,
    start_date TIMESTAMP,
    end_date TIMESTAMP,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'allocated',

    -- Approval
    approved_by VARCHAR(255),
    approved_at TIMESTAMP,

    -- Metadata
    notes TEXT,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT chk_budget_category CHECK (category IN ('labor', 'material', 'equipment', 'overhead', 'contingency')),
    CONSTRAINT chk_budget_status CHECK (status IN ('allocated', 'in-use', 'spent', 'cancelled')),
    CONSTRAINT chk_budget_reference CHECK (
        (project_id IS NOT NULL AND task_id IS NULL) OR
        (project_id IS NULL AND task_id IS NOT NULL)
    )
);

CREATE INDEX idx_budget_allocations_project ON budget_allocations(project_id);
CREATE INDEX idx_budget_allocations_task ON budget_allocations(task_id);
CREATE INDEX idx_budget_allocations_category ON budget_allocations(category);
CREATE INDEX idx_budget_allocations_status ON budget_allocations(status);
CREATE INDEX idx_budget_allocations_deleted_at ON budget_allocations(deleted_at);

-- =====================================================
-- Task Audit Logs Table
-- =====================================================
CREATE TABLE IF NOT EXISTS task_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    -- Change details
    action VARCHAR(50) NOT NULL,
    field VARCHAR(100),
    old_value TEXT,
    new_value TEXT,

    -- Actor information
    performed_by VARCHAR(255) NOT NULL,
    performed_by_name VARCHAR(255),
    role VARCHAR(100),

    -- Additional context
    comment TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    ip_address VARCHAR(50),
    user_agent VARCHAR(500),

    -- Timestamp
    performed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_audit_logs_task ON task_audit_logs(task_id);
CREATE INDEX idx_task_audit_logs_action ON task_audit_logs(action);
CREATE INDEX idx_task_audit_logs_performed_at ON task_audit_logs(performed_at);
CREATE INDEX idx_task_audit_logs_performed_by ON task_audit_logs(performed_by);

-- =====================================================
-- Task Comments Table
-- =====================================================
CREATE TABLE IF NOT EXISTS task_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    -- Comment details
    comment TEXT NOT NULL,
    comment_type VARCHAR(50) DEFAULT 'general',

    -- Author
    author_id VARCHAR(255) NOT NULL,
    author_name VARCHAR(255),

    -- Parent comment (for replies)
    parent_id UUID REFERENCES task_comments(id) ON DELETE CASCADE,

    -- Metadata
    is_edited BOOLEAN DEFAULT false,
    edited_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT chk_comment_type CHECK (comment_type IN ('general', 'update', 'issue', 'resolution'))
);

CREATE INDEX idx_task_comments_task ON task_comments(task_id);
CREATE INDEX idx_task_comments_author ON task_comments(author_id);
CREATE INDEX idx_task_comments_parent ON task_comments(parent_id);
CREATE INDEX idx_task_comments_type ON task_comments(comment_type);
CREATE INDEX idx_task_comments_deleted_at ON task_comments(deleted_at);

-- =====================================================
-- Task Attachments Table
-- =====================================================
CREATE TABLE IF NOT EXISTS task_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    -- File details
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT,
    file_type VARCHAR(100),
    mime_type VARCHAR(100),

    -- Attachment metadata
    attachment_type VARCHAR(50) DEFAULT 'document',
    description TEXT,

    -- Uploader
    uploaded_by VARCHAR(255) NOT NULL,
    uploaded_by_name VARCHAR(255),

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,

    CONSTRAINT chk_attachment_type CHECK (attachment_type IN ('document', 'image', 'video', 'other'))
);

CREATE INDEX idx_task_attachments_task ON task_attachments(task_id);
CREATE INDEX idx_task_attachments_type ON task_attachments(attachment_type);
CREATE INDEX idx_task_attachments_uploaded_by ON task_attachments(uploaded_by);
CREATE INDEX idx_task_attachments_deleted_at ON task_attachments(deleted_at);

-- =====================================================
-- User Project Roles Table
-- =====================================================
CREATE TABLE IF NOT EXISTS user_project_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id VARCHAR(255) NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES project_roles(id) ON DELETE RESTRICT,

    -- Assignment details
    assigned_by VARCHAR(255) NOT NULL,
    assigned_at TIMESTAMP NOT NULL,
    valid_from TIMESTAMP,
    valid_until TIMESTAMP,

    -- Status
    is_active BOOLEAN DEFAULT true,

    -- Metadata
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_project_roles_user ON user_project_roles(user_id);
CREATE INDEX idx_user_project_roles_project ON user_project_roles(project_id);
CREATE INDEX idx_user_project_roles_role ON user_project_roles(role_id);
CREATE INDEX idx_user_project_roles_is_active ON user_project_roles(is_active);
CREATE UNIQUE INDEX idx_user_project_roles_unique ON user_project_roles(user_id, project_id) WHERE is_active = true;

-- =====================================================
-- Seed Default Project Roles
-- =====================================================
INSERT INTO project_roles (code, name, description, permissions, level, is_system_role) VALUES
('project_admin', 'Project Administrator', 'Full control over projects', '["project:create", "project:read", "project:update", "project:delete", "task:create", "task:read", "task:update", "task:delete", "task:assign", "budget:manage", "user:assign"]'::jsonb, 100, true),
('project_manager', 'Project Manager', 'Manage projects and tasks', '["project:read", "project:update", "task:create", "task:read", "task:update", "task:assign", "budget:view", "budget:allocate"]'::jsonb, 80, true),
('supervisor', 'Supervisor', 'Supervise tasks and workers', '["project:read", "task:read", "task:update", "task:assign:limited", "budget:view"]'::jsonb, 60, true),
('worker', 'Worker', 'Execute assigned tasks', '["task:read", "task:update:own", "task:comment"]'::jsonb, 40, true),
('viewer', 'Viewer', 'View-only access to projects', '["project:read", "task:read"]'::jsonb, 20, true)
ON CONFLICT (code) DO NOTHING;

-- =====================================================
-- Create triggers for updated_at
-- =====================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_zones_updated_at BEFORE UPDATE ON zones
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_nodes_updated_at BEFORE UPDATE ON nodes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tasks_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_assignments_updated_at BEFORE UPDATE ON task_assignments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_budget_allocations_updated_at BEFORE UPDATE ON budget_allocations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_comments_updated_at BEFORE UPDATE ON task_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_project_roles_updated_at BEFORE UPDATE ON project_roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_project_roles_updated_at BEFORE UPDATE ON user_project_roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- Comments
-- =====================================================
COMMENT ON TABLE projects IS 'Main projects table storing project information and KMZ data';
COMMENT ON TABLE zones IS 'Geographic zones within projects extracted from KMZ files';
COMMENT ON TABLE nodes IS 'Points/nodes within zones (start, stop, waypoint)';
COMMENT ON TABLE tasks IS 'Work tasks allocated between start and stop nodes';
COMMENT ON TABLE task_assignments IS 'User assignments to tasks with roles';
COMMENT ON TABLE budget_allocations IS 'Budget allocations at project and task level';
COMMENT ON TABLE task_audit_logs IS 'Audit trail for task changes';
COMMENT ON TABLE task_comments IS 'Comments and discussions on tasks';
COMMENT ON TABLE task_attachments IS 'File attachments for tasks';
COMMENT ON TABLE project_roles IS 'Project-specific roles and permissions';
COMMENT ON TABLE user_project_roles IS 'User-to-project role assignments';

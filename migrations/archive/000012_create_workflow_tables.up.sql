-- Migration: Create workflow system tables
-- This migration creates the complete workflow infrastructure

-- ============================================================================
-- 1. Workflow Definitions Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS workflow_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    initial_state VARCHAR(50) NOT NULL DEFAULT 'draft',

    -- Workflow configuration (JSONB)
    states JSONB NOT NULL DEFAULT '[]',
    transitions JSONB NOT NULL DEFAULT '[]',

    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for workflow_definitions
CREATE INDEX idx_workflow_definitions_code ON workflow_definitions(code);
CREATE INDEX idx_workflow_definitions_is_active ON workflow_definitions(is_active);

-- ============================================================================
-- 2. Form Submissions Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS form_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Form reference
    form_code VARCHAR(50) NOT NULL,
    form_id UUID NOT NULL REFERENCES app_forms(id) ON DELETE CASCADE,

    -- Business context
    business_vertical_id UUID NOT NULL REFERENCES business_verticals(id) ON DELETE CASCADE,

    -- Site context (optional)
    site_id UUID REFERENCES sites(id) ON DELETE SET NULL,

    -- Workflow state
    workflow_id UUID REFERENCES workflow_definitions(id) ON DELETE SET NULL,
    current_state VARCHAR(50) NOT NULL DEFAULT 'draft',

    -- Form data (JSONB)
    form_data JSONB NOT NULL DEFAULT '{}',

    -- Metadata
    version INTEGER DEFAULT 1,
    submitted_by VARCHAR(255) NOT NULL,
    submitted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_modified_by VARCHAR(255),
    last_modified_at TIMESTAMP,

    -- Audit trail
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Indexes for form_submissions
CREATE INDEX idx_form_submissions_form_code ON form_submissions(form_code);
CREATE INDEX idx_form_submissions_form_id ON form_submissions(form_id);
CREATE INDEX idx_form_submissions_business_vertical_id ON form_submissions(business_vertical_id);
CREATE INDEX idx_form_submissions_site_id ON form_submissions(site_id) WHERE site_id IS NOT NULL;
CREATE INDEX idx_form_submissions_workflow_id ON form_submissions(workflow_id) WHERE workflow_id IS NOT NULL;
CREATE INDEX idx_form_submissions_current_state ON form_submissions(current_state);
CREATE INDEX idx_form_submissions_submitted_by ON form_submissions(submitted_by);
CREATE INDEX idx_form_submissions_submitted_at ON form_submissions(submitted_at DESC);
CREATE INDEX idx_form_submissions_deleted_at ON form_submissions(deleted_at) WHERE deleted_at IS NOT NULL;

-- Composite indexes for common queries
CREATE INDEX idx_form_submissions_form_business ON form_submissions(form_code, business_vertical_id);
CREATE INDEX idx_form_submissions_form_state ON form_submissions(form_code, current_state);
CREATE INDEX idx_form_submissions_business_state ON form_submissions(business_vertical_id, current_state);

-- ============================================================================
-- 3. Workflow Transitions Table (Audit Trail)
-- ============================================================================
CREATE TABLE IF NOT EXISTS workflow_transitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Submission reference
    submission_id UUID NOT NULL REFERENCES form_submissions(id) ON DELETE CASCADE,

    -- Transition details
    from_state VARCHAR(50) NOT NULL,
    to_state VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,

    -- Actor information
    actor_id VARCHAR(255) NOT NULL,
    actor_name VARCHAR(255),
    actor_role VARCHAR(100),

    -- Additional context
    comment TEXT,
    metadata JSONB DEFAULT '{}',

    -- Timestamp
    transitioned_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for workflow_transitions
CREATE INDEX idx_workflow_transitions_submission_id ON workflow_transitions(submission_id);
CREATE INDEX idx_workflow_transitions_transitioned_at ON workflow_transitions(transitioned_at DESC);
CREATE INDEX idx_workflow_transitions_actor_id ON workflow_transitions(actor_id);
CREATE INDEX idx_workflow_transitions_action ON workflow_transitions(action);

-- Composite index for history queries
CREATE INDEX idx_workflow_transitions_submission_time ON workflow_transitions(submission_id, transitioned_at DESC);

-- ============================================================================
-- 4. Add Comments
-- ============================================================================
COMMENT ON TABLE workflow_definitions IS 'Stores workflow configuration including states and transitions';
COMMENT ON TABLE form_submissions IS 'Stores form submission instances with workflow state tracking';
COMMENT ON TABLE workflow_transitions IS 'Audit trail for all workflow state transitions';

COMMENT ON COLUMN form_submissions.form_data IS 'JSON object containing all submitted form field values';
COMMENT ON COLUMN form_submissions.current_state IS 'Current workflow state (e.g., draft, submitted, approved, rejected)';
COMMENT ON COLUMN workflow_transitions.metadata IS 'Additional context data for the transition';

-- ============================================================================
-- 5. Sample Workflow Definition
-- ============================================================================
-- Insert a standard approval workflow
INSERT INTO workflow_definitions (code, name, description, initial_state, states, transitions, is_active)
VALUES (
    'standard_approval',
    'Standard Approval Workflow',
    'Basic workflow with draft, submission, approval, and rejection states',
    'draft',
    '[
        {"code": "draft", "name": "Draft", "description": "Initial draft state", "color": "#gray", "icon": "edit"},
        {"code": "submitted", "name": "Submitted", "description": "Awaiting approval", "color": "#blue", "icon": "send"},
        {"code": "approved", "name": "Approved", "description": "Approved by reviewer", "color": "#green", "icon": "check", "is_final": true},
        {"code": "rejected", "name": "Rejected", "description": "Rejected by reviewer", "color": "#red", "icon": "close", "is_final": true}
    ]'::jsonb,
    '[
        {"from": "draft", "to": "submitted", "action": "submit", "label": "Submit for Approval", "permission": "project:create"},
        {"from": "submitted", "to": "approved", "action": "approve", "label": "Approve", "permission": "project:approve", "requires_comment": false},
        {"from": "submitted", "to": "rejected", "action": "reject", "label": "Reject", "permission": "project:approve", "requires_comment": true},
        {"from": "submitted", "to": "draft", "action": "recall", "label": "Recall Submission", "permission": "project:create"},
        {"from": "rejected", "to": "draft", "action": "revise", "label": "Revise and Resubmit", "permission": "project:create"}
    ]'::jsonb,
    true
) ON CONFLICT (code) DO NOTHING;

-- ============================================================================
-- 6. Update app_forms table to ensure workflow_id column exists
-- ============================================================================
-- This ensures the workflow_id column exists in app_forms (it should already exist from the model)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'app_forms' AND column_name = 'workflow_id'
    ) THEN
        ALTER TABLE app_forms ADD COLUMN workflow_id UUID REFERENCES workflow_definitions(id) ON DELETE SET NULL;
        CREATE INDEX idx_app_forms_workflow_id ON app_forms(workflow_id) WHERE workflow_id IS NOT NULL;
    END IF;
END $$;

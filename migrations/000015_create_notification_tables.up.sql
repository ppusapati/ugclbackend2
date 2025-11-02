-- Migration: Create notification system tables
-- This migration creates the complete notification infrastructure

-- ============================================================================
-- 1. Notification Rules Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS notification_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,

    -- Workflow integration
    workflow_id UUID REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    trigger_on_states JSONB DEFAULT '[]',
    trigger_on_actions JSONB DEFAULT '[]',

    -- Notification content
    priority VARCHAR(20) DEFAULT 'normal',
    channels JSONB DEFAULT '["in_app"]',
    title_template VARCHAR(500) NOT NULL,
    body_template TEXT NOT NULL,
    action_url VARCHAR(500),

    -- Email specific
    email_subject VARCHAR(500),
    email_template TEXT,

    -- SMS specific
    sms_template VARCHAR(500),

    -- Conditions
    conditions JSONB,

    -- Settings
    is_active BOOLEAN DEFAULT true,
    batch_interval_minutes INTEGER DEFAULT 0,
    deduplicate_key VARCHAR(200),

    -- Metadata
    created_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for notification_rules
CREATE INDEX idx_notification_rules_workflow_id ON notification_rules(workflow_id) WHERE workflow_id IS NOT NULL;
CREATE INDEX idx_notification_rules_is_active ON notification_rules(is_active);
CREATE INDEX idx_notification_rules_code ON notification_rules(code);

-- ============================================================================
-- 2. Notification Recipients Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS notification_recipients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_rule_id UUID NOT NULL REFERENCES notification_rules(id) ON DELETE CASCADE,

    -- Multi-level targeting
    user_id VARCHAR(255),
    role_id UUID,
    business_role_id UUID,
    permission_code VARCHAR(100),
    attribute_condition JSONB,
    policy_id UUID,

    -- Dynamic recipient resolution
    recipient_type VARCHAR(50) NOT NULL, -- user, role, business_role, permission, attribute, policy, submitter, approver

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for notification_recipients
CREATE INDEX idx_notification_recipients_rule_id ON notification_recipients(notification_rule_id);
CREATE INDEX idx_notification_recipients_user_id ON notification_recipients(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_notification_recipients_role_id ON notification_recipients(role_id) WHERE role_id IS NOT NULL;
CREATE INDEX idx_notification_recipients_business_role_id ON notification_recipients(business_role_id) WHERE business_role_id IS NOT NULL;
CREATE INDEX idx_notification_recipients_type ON notification_recipients(recipient_type);

-- ============================================================================
-- 3. Notifications Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Rule reference
    notification_rule_id UUID REFERENCES notification_rules(id) ON DELETE SET NULL,

    -- Recipient
    user_id VARCHAR(255) NOT NULL,

    -- Content
    type VARCHAR(50) NOT NULL,
    priority VARCHAR(20) DEFAULT 'normal',
    title VARCHAR(500) NOT NULL,
    body TEXT NOT NULL,
    action_url VARCHAR(500),

    -- Context (what triggered this notification)
    submission_id UUID REFERENCES form_submissions(id) ON DELETE CASCADE,
    workflow_id UUID REFERENCES workflow_definitions(id) ON DELETE SET NULL,
    transition_id UUID REFERENCES workflow_transitions(id) ON DELETE SET NULL,
    form_code VARCHAR(50),
    business_vertical_id UUID REFERENCES business_verticals(id) ON DELETE SET NULL,

    -- Additional context data
    metadata JSONB,

    -- Delivery status
    status VARCHAR(20) DEFAULT 'pending',
    channel VARCHAR(20) DEFAULT 'in_app',
    sent_at TIMESTAMP,
    read_at TIMESTAMP,
    archived_at TIMESTAMP,
    failed_reason TEXT,

    -- Grouping (for batching similar notifications)
    group_key VARCHAR(200),

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for notifications
CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_type ON notifications(type);
CREATE INDEX idx_notifications_submission_id ON notifications(submission_id) WHERE submission_id IS NOT NULL;
CREATE INDEX idx_notifications_workflow_id ON notifications(workflow_id) WHERE workflow_id IS NOT NULL;
CREATE INDEX idx_notifications_created_at ON notifications(created_at DESC);
CREATE INDEX idx_notifications_read_at ON notifications(read_at) WHERE read_at IS NOT NULL;
CREATE INDEX idx_notifications_group_key ON notifications(group_key) WHERE group_key IS NOT NULL;

-- Composite indexes for common queries
CREATE INDEX idx_notifications_user_status ON notifications(user_id, status);
CREATE INDEX idx_notifications_user_unread ON notifications(user_id, created_at DESC) WHERE read_at IS NULL;
CREATE INDEX idx_notifications_user_type ON notifications(user_id, type);

-- ============================================================================
-- 4. Notification Preferences Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) UNIQUE NOT NULL,

    -- Channel preferences
    enable_in_app BOOLEAN DEFAULT true,
    enable_email BOOLEAN DEFAULT true,
    enable_sms BOOLEAN DEFAULT false,
    enable_web_push BOOLEAN DEFAULT true,

    -- Type preferences (can disable specific types)
    disabled_types JSONB DEFAULT '[]',

    -- Quiet hours
    quiet_hours_enabled BOOLEAN DEFAULT false,
    quiet_hours_start VARCHAR(5), -- HH:MM format
    quiet_hours_end VARCHAR(5),   -- HH:MM format

    -- Digest settings
    digest_enabled BOOLEAN DEFAULT false,
    digest_frequency VARCHAR(20), -- daily, weekly

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for notification_preferences
CREATE INDEX idx_notification_preferences_user_id ON notification_preferences(user_id);

-- ============================================================================
-- 5. Add Comments
-- ============================================================================
COMMENT ON TABLE notification_rules IS 'Defines notification rules triggered by workflow events';
COMMENT ON TABLE notification_recipients IS 'Defines who should receive notifications based on various targeting strategies';
COMMENT ON TABLE notifications IS 'Stores actual notification instances sent to users';
COMMENT ON TABLE notification_preferences IS 'Stores user notification delivery preferences';

COMMENT ON COLUMN notification_rules.trigger_on_states IS 'JSON array of workflow states that trigger this notification';
COMMENT ON COLUMN notification_rules.trigger_on_actions IS 'JSON array of workflow actions that trigger this notification';
COMMENT ON COLUMN notification_recipients.recipient_type IS 'Type of recipient: user, role, business_role, permission, attribute, policy, submitter, approver';
COMMENT ON COLUMN notifications.metadata IS 'Additional context data for the notification';
COMMENT ON COLUMN notifications.group_key IS 'Key for grouping similar notifications for batching';

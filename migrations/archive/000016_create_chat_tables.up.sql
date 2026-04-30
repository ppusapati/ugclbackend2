-- Chat Module Database Schema
-- Migration: 000016_create_chat_tables.up.sql

-- ============================================================================
-- ENUM TYPES
-- ============================================================================

-- Conversation type enum
DO $$ BEGIN
    CREATE TYPE conversation_type AS ENUM ('direct', 'group', 'channel');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Message type enum
DO $$ BEGIN
    CREATE TYPE message_type AS ENUM ('text', 'image', 'file', 'video', 'audio', 'location', 'system');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Message status enum
DO $$ BEGIN
    CREATE TYPE message_status AS ENUM ('sending', 'sent', 'delivered', 'read', 'failed', 'deleted');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Participant role enum
DO $$ BEGIN
    CREATE TYPE participant_role AS ENUM ('owner', 'admin', 'moderator', 'member');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- ============================================================================
-- TABLES
-- ============================================================================

-- Conversations table
CREATE TABLE IF NOT EXISTS chat_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type conversation_type NOT NULL DEFAULT 'direct',
    title VARCHAR(255),
    description TEXT,
    avatar_url VARCHAR(500),
    metadata JSONB DEFAULT '{}',
    last_message_id UUID,
    last_message_at TIMESTAMP WITH TIME ZONE,
    is_muted BOOLEAN DEFAULT false,
    is_archived BOOLEAN DEFAULT false,
    max_participants INTEGER DEFAULT 100,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Messages table
CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    sender_id VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    message_type message_type NOT NULL DEFAULT 'text',
    status message_status NOT NULL DEFAULT 'sent',
    reply_to_id UUID REFERENCES chat_messages(id) ON DELETE SET NULL,
    metadata JSONB DEFAULT '{}',
    sent_at TIMESTAMP WITH TIME ZONE,
    delivered_at TIMESTAMP WITH TIME ZONE,
    is_edited BOOLEAN DEFAULT false,
    edited_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Participants table
CREATE TABLE IF NOT EXISTS chat_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    role participant_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    left_at TIMESTAMP WITH TIME ZONE,
    last_read_message_id UUID REFERENCES chat_messages(id) ON DELETE SET NULL,
    last_read_at TIMESTAMP WITH TIME ZONE,
    notifications_enabled BOOLEAN DEFAULT true,
    mention_notifications_only BOOLEAN DEFAULT false,
    is_muted BOOLEAN DEFAULT false,
    muted_until TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_participant UNIQUE (conversation_id, user_id)
);

-- Attachments table
CREATE TABLE IF NOT EXISTS chat_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    dms_file_id VARCHAR(255),
    dms_file_url VARCHAR(1000),
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    thumbnail_url VARCHAR(1000),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Typing indicators table (ephemeral data)
CREATE TABLE IF NOT EXISTS chat_typing_indicators (
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (conversation_id, user_id)
);

-- Read receipts table
CREATE TABLE IF NOT EXISTS chat_read_receipts (
    message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    read_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

-- Reactions table
CREATE TABLE IF NOT EXISTS chat_reactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    reaction VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_reaction UNIQUE (message_id, user_id, reaction)
);

-- ============================================================================
-- INDEXES
-- ============================================================================

-- Conversation indexes
CREATE INDEX IF NOT EXISTS idx_chat_conversations_created_by ON chat_conversations(created_by);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_type ON chat_conversations(type);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_deleted_at ON chat_conversations(deleted_at);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_last_message_at ON chat_conversations(last_message_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_is_archived ON chat_conversations(is_archived) WHERE deleted_at IS NULL;

-- Message indexes
CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation_id ON chat_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_sender_id ON chat_messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation_created ON chat_messages(conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_deleted_at ON chat_messages(deleted_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_reply_to_id ON chat_messages(reply_to_id) WHERE reply_to_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_chat_messages_status ON chat_messages(status) WHERE deleted_at IS NULL;

-- Full-text search index on message content
CREATE INDEX IF NOT EXISTS idx_chat_messages_content_search ON chat_messages USING gin(to_tsvector('english', content));

-- Participant indexes
CREATE INDEX IF NOT EXISTS idx_chat_participants_user_id ON chat_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_participants_conversation_id ON chat_participants(conversation_id);
CREATE INDEX IF NOT EXISTS idx_chat_participants_active ON chat_participants(user_id, conversation_id) WHERE left_at IS NULL;

-- Attachment indexes
CREATE INDEX IF NOT EXISTS idx_chat_attachments_message_id ON chat_attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_chat_attachments_dms_file_id ON chat_attachments(dms_file_id) WHERE dms_file_id IS NOT NULL;

-- Typing indicator indexes
CREATE INDEX IF NOT EXISTS idx_chat_typing_indicators_expires ON chat_typing_indicators(expires_at);

-- Read receipt indexes
CREATE INDEX IF NOT EXISTS idx_chat_read_receipts_message_id ON chat_read_receipts(message_id);
CREATE INDEX IF NOT EXISTS idx_chat_read_receipts_user_id ON chat_read_receipts(user_id);

-- Reaction indexes
CREATE INDEX IF NOT EXISTS idx_chat_reactions_message_id ON chat_reactions(message_id);
CREATE INDEX IF NOT EXISTS idx_chat_reactions_user_id ON chat_reactions(user_id);

-- ============================================================================
-- VIEWS
-- ============================================================================

-- View for unread message counts per user per conversation
CREATE OR REPLACE VIEW chat_unread_counts AS
SELECT
    p.conversation_id,
    p.user_id,
    COUNT(m.id) AS unread_count
FROM chat_participants p
LEFT JOIN chat_messages m ON m.conversation_id = p.conversation_id
    AND m.deleted_at IS NULL
    AND m.sender_id != p.user_id
    AND (p.last_read_at IS NULL OR m.created_at > p.last_read_at)
WHERE p.left_at IS NULL
GROUP BY p.conversation_id, p.user_id;

-- View for user conversations with last message and unread count
CREATE OR REPLACE VIEW chat_user_conversations AS
SELECT
    c.id AS conversation_id,
    c.type,
    c.title,
    c.description,
    c.avatar_url,
    c.metadata,
    c.is_muted,
    c.is_archived,
    c.max_participants,
    c.created_by,
    c.created_at,
    c.updated_at,
    p.user_id,
    p.role,
    p.joined_at,
    p.notifications_enabled,
    p.mention_notifications_only,
    p.is_muted AS participant_muted,
    p.muted_until,
    m.id AS last_message_id,
    m.content AS last_message_content,
    m.sender_id AS last_message_sender_id,
    m.message_type AS last_message_type,
    m.created_at AS last_message_at,
    COALESCE(uc.unread_count, 0) AS unread_count
FROM chat_conversations c
JOIN chat_participants p ON p.conversation_id = c.id AND p.left_at IS NULL
LEFT JOIN chat_messages m ON m.id = c.last_message_id
LEFT JOIN chat_unread_counts uc ON uc.conversation_id = c.id AND uc.user_id = p.user_id
WHERE c.deleted_at IS NULL;

-- View for message details with attachment and reaction counts
CREATE OR REPLACE VIEW chat_message_details AS
SELECT
    m.id,
    m.conversation_id,
    m.sender_id,
    m.content,
    m.message_type,
    m.status,
    m.reply_to_id,
    m.metadata,
    m.sent_at,
    m.delivered_at,
    m.is_edited,
    m.edited_at,
    m.created_at,
    m.updated_at,
    COALESCE(att.attachment_count, 0) AS attachment_count,
    COALESCE(react.reaction_count, 0) AS reaction_count,
    COALESCE(rr.read_count, 0) AS read_count
FROM chat_messages m
LEFT JOIN (
    SELECT message_id, COUNT(*) AS attachment_count
    FROM chat_attachments
    GROUP BY message_id
) att ON att.message_id = m.id
LEFT JOIN (
    SELECT message_id, COUNT(*) AS reaction_count
    FROM chat_reactions
    GROUP BY message_id
) react ON react.message_id = m.id
LEFT JOIN (
    SELECT message_id, COUNT(*) AS read_count
    FROM chat_read_receipts
    GROUP BY message_id
) rr ON rr.message_id = m.id
WHERE m.deleted_at IS NULL;

-- ============================================================================
-- FUNCTIONS & TRIGGERS
-- ============================================================================

-- Function to update conversation's last message
CREATE OR REPLACE FUNCTION update_conversation_last_message()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE chat_conversations
    SET last_message_id = NEW.id,
        last_message_at = NEW.created_at,
        updated_at = NOW()
    WHERE id = NEW.conversation_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update conversation last message on new message
DROP TRIGGER IF EXISTS trg_update_conversation_last_message ON chat_messages;
CREATE TRIGGER trg_update_conversation_last_message
    AFTER INSERT ON chat_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_conversation_last_message();

-- Function to update timestamps
CREATE OR REPLACE FUNCTION update_chat_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for updated_at
DROP TRIGGER IF EXISTS trg_chat_conversations_updated_at ON chat_conversations;
CREATE TRIGGER trg_chat_conversations_updated_at
    BEFORE UPDATE ON chat_conversations
    FOR EACH ROW
    EXECUTE FUNCTION update_chat_updated_at();

DROP TRIGGER IF EXISTS trg_chat_messages_updated_at ON chat_messages;
CREATE TRIGGER trg_chat_messages_updated_at
    BEFORE UPDATE ON chat_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_chat_updated_at();

DROP TRIGGER IF EXISTS trg_chat_participants_updated_at ON chat_participants;
CREATE TRIGGER trg_chat_participants_updated_at
    BEFORE UPDATE ON chat_participants
    FOR EACH ROW
    EXECUTE FUNCTION update_chat_updated_at();

-- Function to cleanup expired typing indicators
CREATE OR REPLACE FUNCTION cleanup_expired_typing_indicators()
RETURNS void AS $$
BEGIN
    DELETE FROM chat_typing_indicators WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Function to update participant's last read
CREATE OR REPLACE FUNCTION update_participant_last_read()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE chat_participants
    SET last_read_message_id = NEW.message_id,
        last_read_at = NEW.read_at,
        updated_at = NOW()
    WHERE conversation_id = (SELECT conversation_id FROM chat_messages WHERE id = NEW.message_id)
      AND user_id = NEW.user_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update participant last read on read receipt
DROP TRIGGER IF EXISTS trg_update_participant_last_read ON chat_read_receipts;
CREATE TRIGGER trg_update_participant_last_read
    AFTER INSERT ON chat_read_receipts
    FOR EACH ROW
    EXECUTE FUNCTION update_participant_last_read();

-- ============================================================================
-- COMMENTS
-- ============================================================================

COMMENT ON TABLE chat_conversations IS 'Stores chat conversations (direct, group, channel)';
COMMENT ON TABLE chat_messages IS 'Stores individual messages within conversations';
COMMENT ON TABLE chat_participants IS 'Stores participants in each conversation with their roles';
COMMENT ON TABLE chat_attachments IS 'Stores file attachments linked to messages';
COMMENT ON TABLE chat_typing_indicators IS 'Ephemeral storage for typing indicators';
COMMENT ON TABLE chat_read_receipts IS 'Tracks which users have read which messages';
COMMENT ON TABLE chat_reactions IS 'Stores emoji reactions to messages';

COMMENT ON VIEW chat_unread_counts IS 'Aggregated unread message counts per user per conversation';
COMMENT ON VIEW chat_user_conversations IS 'User conversations with last message info and unread count';
COMMENT ON VIEW chat_message_details IS 'Messages with attachment/reaction counts';

-- Chat Module Database Schema - Rollback
-- Migration: 000016_create_chat_tables.down.sql

-- Drop triggers
DROP TRIGGER IF EXISTS trg_update_participant_last_read ON chat_read_receipts;
DROP TRIGGER IF EXISTS trg_chat_participants_updated_at ON chat_participants;
DROP TRIGGER IF EXISTS trg_chat_messages_updated_at ON chat_messages;
DROP TRIGGER IF EXISTS trg_chat_conversations_updated_at ON chat_conversations;
DROP TRIGGER IF EXISTS trg_update_conversation_last_message ON chat_messages;

-- Drop functions
DROP FUNCTION IF EXISTS update_participant_last_read();
DROP FUNCTION IF EXISTS cleanup_expired_typing_indicators();
DROP FUNCTION IF EXISTS update_chat_updated_at();
DROP FUNCTION IF EXISTS update_conversation_last_message();

-- Drop views
DROP VIEW IF EXISTS chat_message_details;
DROP VIEW IF EXISTS chat_user_conversations;
DROP VIEW IF EXISTS chat_unread_counts;

-- Drop tables (in order of dependencies)
DROP TABLE IF EXISTS chat_reactions;
DROP TABLE IF EXISTS chat_read_receipts;
DROP TABLE IF EXISTS chat_typing_indicators;
DROP TABLE IF EXISTS chat_attachments;
DROP TABLE IF EXISTS chat_participants;
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_conversations;

-- Drop enum types
DROP TYPE IF EXISTS participant_role;
DROP TYPE IF EXISTS message_status;
DROP TYPE IF EXISTS message_type;
DROP TYPE IF EXISTS conversation_type;

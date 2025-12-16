package handlers

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// ChatService handles chat business logic
type ChatService struct {
	db *gorm.DB
}

// NewChatService creates a new ChatService instance
func NewChatService() *ChatService {
	return &ChatService{
		db: config.DB,
	}
}

// ============================================================================
// Conversation Operations
// ============================================================================

// CreateConversation creates a new conversation
func (s *ChatService) CreateConversation(creatorID string, req models.CreateConversationRequest) (*models.Conversation, error) {
	// For direct conversations, check if one already exists between the two users
	if req.Type == models.ConversationTypeDirect {
		if len(req.GetParticipantIDs()) != 1 {
			return nil, errors.New("direct conversation must have exactly one other participant")
		}

		existingConv, err := s.GetDirectConversation(creatorID, req.GetParticipantIDs()[0])
		if err == nil && existingConv != nil {
			return existingConv, nil
		}
	}

	// Set default max participants
	maxParticipants := req.MaxParticipants
	if maxParticipants == 0 {
		switch req.Type {
		case models.ConversationTypeDirect:
			maxParticipants = 2
		case models.ConversationTypeGroup:
			maxParticipants = 100
		case models.ConversationTypeChannel:
			maxParticipants = 10000
		}
	}

	// Create conversation
	conversation := &models.Conversation{
		Type:            req.Type,
		Title:           req.Title,
		Description:     req.Description,
		AvatarURL:       req.AvatarURL,
		Metadata:        req.Metadata,
		MaxParticipants: maxParticipants,
		CreatedBy:       creatorID,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create conversation
		if err := tx.Create(conversation).Error; err != nil {
			return fmt.Errorf("failed to create conversation: %w", err)
		}

		// Add creator as owner
		creatorParticipant := &models.ChatParticipant{
			ConversationID:       conversation.ID,
			UserID:               creatorID,
			Role:                 models.ParticipantRoleOwner,
			JoinedAt:             time.Now(),
			NotificationsEnabled: true,
		}
		if err := tx.Create(creatorParticipant).Error; err != nil {
			return fmt.Errorf("failed to add creator as participant: %w", err)
		}

		// Add other participants
		participantIDs := req.GetParticipantIDs()
		for _, participantID := range participantIDs {
			if participantID == creatorID {
				continue // Skip creator, already added
			}

			participant := &models.ChatParticipant{
				ConversationID:       conversation.ID,
				UserID:               participantID,
				Role:                 models.ParticipantRoleMember,
				JoinedAt:             time.Now(),
				NotificationsEnabled: true,
			}
			if err := tx.Create(participant).Error; err != nil {
				return fmt.Errorf("failed to add participant %s: %w", participantID, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Reload with participants
	if err := s.db.Preload("Participants").Preload("Participants.User").First(conversation, conversation.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload conversation: %w", err)
	}

	log.Printf("✅ Created conversation %s (type: %s) by user %s", conversation.ID, conversation.Type, creatorID)
	return conversation, nil
}

// CreateGroup creates a new group (admin only)
// The creator becomes the owner, all members are added with 'member' role
func (s *ChatService) CreateGroup(creatorID string, req models.CreateGroupRequest) (*models.Conversation, error) {
	if len(req.MemberIDs) == 0 {
		return nil, errors.New("at least one member is required")
	}

	// Set default max participants for groups
	maxParticipants := req.MaxParticipants
	if maxParticipants == 0 {
		maxParticipants = 100
	}

	// Create conversation
	conversation := &models.Conversation{
		Type:            models.ConversationTypeGroup,
		Title:           &req.Title,
		Description:     req.Description,
		AvatarURL:       req.AvatarURL,
		Metadata:        req.Metadata,
		MaxParticipants: maxParticipants,
		CreatedBy:       creatorID,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create conversation
		if err := tx.Create(conversation).Error; err != nil {
			return fmt.Errorf("failed to create group: %w", err)
		}

		// Add creator as owner
		creatorParticipant := &models.ChatParticipant{
			ConversationID:       conversation.ID,
			UserID:               creatorID,
			Role:                 models.ParticipantRoleOwner,
			JoinedAt:             time.Now(),
			NotificationsEnabled: true,
		}
		if err := tx.Create(creatorParticipant).Error; err != nil {
			return fmt.Errorf("failed to add creator as participant: %w", err)
		}

		// Add members
		for _, memberID := range req.MemberIDs {
			if memberID == creatorID {
				continue // Skip creator, already added as owner
			}

			participant := &models.ChatParticipant{
				ConversationID:       conversation.ID,
				UserID:               memberID,
				Role:                 models.ParticipantRoleMember,
				JoinedAt:             time.Now(),
				NotificationsEnabled: true,
			}
			if err := tx.Create(participant).Error; err != nil {
				return fmt.Errorf("failed to add member %s: %w", memberID, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Reload with participants
	if err := s.db.Preload("Participants").Preload("Participants.User").First(conversation, conversation.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload group: %w", err)
	}

	log.Printf("✅ Created group %s ('%s') by admin %s with %d members", conversation.ID, req.Title, creatorID, len(req.MemberIDs))
	return conversation, nil
}

// GetConversation retrieves a conversation by ID
func (s *ChatService) GetConversation(conversationID uuid.UUID, userID string) (*models.Conversation, error) {
	var conversation models.Conversation
	err := s.db.
		Preload("Participants").
		Preload("Participants.User").
		Where("id = ? AND deleted_at IS NULL", conversationID).
		First(&conversation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("conversation not found")
		}
		return nil, err
	}

	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return nil, errors.New("user is not a participant in this conversation")
	}

	// Manually load LastMessage (since it's not a GORM relation)
	if conversation.LastMessageID != nil {
		var lastMsg models.ChatMessage
		if err := s.db.First(&lastMsg, "id = ?", conversation.LastMessageID).Error; err == nil {
			conversation.LastMessage = &lastMsg
		}
	}

	return &conversation, nil
}

// GetDirectConversation finds an existing direct conversation between two users
func (s *ChatService) GetDirectConversation(userID1, userID2 string) (*models.Conversation, error) {
	var conversation models.Conversation

	// Find a direct conversation where both users are participants
	err := s.db.
		Joins("JOIN chat_participants p1 ON p1.conversation_id = chat_conversations.id AND p1.user_id = ? AND p1.left_at IS NULL", userID1).
		Joins("JOIN chat_participants p2 ON p2.conversation_id = chat_conversations.id AND p2.user_id = ? AND p2.left_at IS NULL", userID2).
		Where("chat_conversations.type = ? AND chat_conversations.deleted_at IS NULL", models.ConversationTypeDirect).
		Preload("Participants").
		First(&conversation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &conversation, nil
}

// ListUserConversations lists conversations for a user with pagination
func (s *ChatService) ListUserConversations(userID string, page, pageSize int, includeArchived bool, convType *models.ConversationType) ([]models.Conversation, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var conversations []models.Conversation
	var totalCount int64

	query := s.db.Model(&models.Conversation{}).
		Joins("JOIN chat_participants ON chat_participants.conversation_id = chat_conversations.id").
		Where("chat_participants.user_id = ? AND chat_participants.left_at IS NULL", userID).
		Where("chat_conversations.deleted_at IS NULL")

	if !includeArchived {
		query = query.Where("chat_conversations.is_archived = false")
	}

	if convType != nil {
		query = query.Where("chat_conversations.type = ?", *convType)
	}

	// Get total count
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	err := query.
		Preload("Participants").
		Preload("Participants.User").
		Order("chat_conversations.last_message_at DESC NULLS LAST, chat_conversations.created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&conversations).Error

	if err != nil {
		return nil, 0, err
	}

	// Manually load LastMessage for each conversation (since it's not a GORM relation)
	for i := range conversations {
		if conversations[i].LastMessageID != nil {
			var lastMsg models.ChatMessage
			if err := config.DB.First(&lastMsg, "id = ?", conversations[i].LastMessageID).Error; err == nil {
				conversations[i].LastMessage = &lastMsg
			}
		}
	}

	return conversations, totalCount, nil
}

// UpdateConversation updates a conversation
func (s *ChatService) UpdateConversation(conversationID uuid.UUID, userID string, req models.UpdateConversationRequest) (*models.Conversation, error) {
	conversation, err := s.GetConversation(conversationID, userID)
	if err != nil {
		return nil, err
	}

	// Check if user has permission to update (owner or admin)
	role, err := s.GetParticipantRole(conversationID, userID)
	if err != nil {
		return nil, err
	}
	if role != models.ParticipantRoleOwner && role != models.ParticipantRoleAdmin {
		return nil, errors.New("only owner or admin can update conversation")
	}

	// Update fields
	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = req.Title
	}
	if req.Description != nil {
		updates["description"] = req.Description
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.Metadata != nil {
		updates["metadata"] = req.Metadata
	}
	if req.MaxParticipants != nil {
		updates["max_participants"] = *req.MaxParticipants
	}

	if err := s.db.Model(conversation).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update conversation: %w", err)
	}

	log.Printf("✅ Updated conversation %s by user %s", conversationID, userID)
	return conversation, nil
}

// DeleteConversation soft deletes a conversation
func (s *ChatService) DeleteConversation(conversationID uuid.UUID, userID string) error {
	conversation, err := s.GetConversation(conversationID, userID)
	if err != nil {
		return err
	}

	// Check if user is owner
	role, err := s.GetParticipantRole(conversationID, userID)
	if err != nil {
		return err
	}
	if role != models.ParticipantRoleOwner {
		return errors.New("only owner can delete conversation")
	}

	now := time.Now()
	if err := s.db.Model(conversation).Update("deleted_at", now).Error; err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	log.Printf("✅ Deleted conversation %s by user %s", conversationID, userID)
	return nil
}

// ArchiveConversation archives or unarchives a conversation for a user
func (s *ChatService) ArchiveConversation(conversationID uuid.UUID, userID string, archive bool) (*models.Conversation, error) {
	conversation, err := s.GetConversation(conversationID, userID)
	if err != nil {
		return nil, err
	}

	if err := s.db.Model(conversation).Update("is_archived", archive).Error; err != nil {
		return nil, fmt.Errorf("failed to archive conversation: %w", err)
	}

	action := "archived"
	if !archive {
		action = "unarchived"
	}
	log.Printf("✅ %s conversation %s by user %s", action, conversationID, userID)
	return conversation, nil
}

// ============================================================================
// Message Operations
// ============================================================================

// SendMessage sends a new message to a conversation
func (s *ChatService) SendMessage(conversationID uuid.UUID, senderID string, req models.SendMessageRequest) (*models.ChatMessage, error) {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, senderID) {
		return nil, errors.New("user is not a participant in this conversation")
	}

	// Set default message type
	messageType := req.MessageType
	if messageType == "" {
		messageType = models.MessageTypeText
	}

	now := time.Now()
	message := &models.ChatMessage{
		ConversationID: conversationID,
		SenderID:       senderID,
		Content:        req.Content,
		MessageType:    messageType,
		Status:         models.MessageStatusSent,
		ReplyToID:      req.ReplyToID,
		Metadata:       req.Metadata,
		SentAt:         &now,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create message
		if err := tx.Create(message).Error; err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		// Update conversation's last message
		if err := tx.Model(&models.Conversation{}).
			Where("id = ?", conversationID).
			Updates(map[string]interface{}{
				"last_message_id": message.ID,
				"last_message_at": now,
			}).Error; err != nil {
			return fmt.Errorf("failed to update conversation: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Printf("✅ Message %s sent to conversation %s by user %s", message.ID, conversationID, senderID)
	return message, nil
}

// GetMessage retrieves a message by ID
func (s *ChatService) GetMessage(messageID uuid.UUID, userID string) (*models.ChatMessage, error) {
	var message models.ChatMessage
	err := s.db.
		Preload("Sender").
		Preload("Attachments").
		Preload("Reactions").
		Preload("ReadReceipts").
		Where("id = ? AND deleted_at IS NULL", messageID).
		First(&message).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("message not found")
		}
		return nil, err
	}

	// Verify user is a participant in the conversation
	if !s.IsParticipant(message.ConversationID, userID) {
		return nil, errors.New("user is not a participant in this conversation")
	}

	return &message, nil
}

// ListMessages lists messages in a conversation with pagination
func (s *ChatService) ListMessages(conversationID uuid.UUID, userID string, page, pageSize int, beforeMessageID, afterMessageID *uuid.UUID) ([]models.ChatMessage, int64, bool, error) {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return nil, 0, false, errors.New("user is not a participant in this conversation")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	var messages []models.ChatMessage
	var totalCount int64

	query := s.db.Model(&models.ChatMessage{}).
		Where("conversation_id = ? AND deleted_at IS NULL", conversationID)

	if beforeMessageID != nil {
		var beforeMsg models.ChatMessage
		if err := s.db.Select("created_at").Where("id = ?", *beforeMessageID).First(&beforeMsg).Error; err == nil {
			query = query.Where("created_at < ?", beforeMsg.CreatedAt)
		}
	}

	if afterMessageID != nil {
		var afterMsg models.ChatMessage
		if err := s.db.Select("created_at").Where("id = ?", *afterMessageID).First(&afterMsg).Error; err == nil {
			query = query.Where("created_at > ?", afterMsg.CreatedAt)
		}
	}

	// Get total count
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, false, err
	}

	// Get paginated results (newest first)
	offset := (page - 1) * pageSize
	err := query.
		Preload("Sender").
		Preload("Attachments").
		Preload("Reactions").
		Preload("ReadReceipts").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // Fetch one extra to check if there are more
		Find(&messages).Error

	if err != nil {
		return nil, 0, false, err
	}

	hasMore := len(messages) > pageSize
	if hasMore {
		messages = messages[:pageSize]
	}

	return messages, totalCount, hasMore, nil
}

// UpdateMessage updates a message content
func (s *ChatService) UpdateMessage(messageID uuid.UUID, userID string, req models.UpdateMessageRequest) (*models.ChatMessage, error) {
	message, err := s.GetMessage(messageID, userID)
	if err != nil {
		return nil, err
	}

	// Only sender can edit their message
	if message.SenderID != userID {
		return nil, errors.New("only the sender can edit this message")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"content":   req.Content,
		"is_edited": true,
		"edited_at": now,
	}

	if err := s.db.Model(message).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	log.Printf("✅ Message %s updated by user %s", messageID, userID)
	return message, nil
}

// DeleteMessage soft deletes a message
func (s *ChatService) DeleteMessage(messageID uuid.UUID, userID string) error {
	message, err := s.GetMessage(messageID, userID)
	if err != nil {
		return err
	}

	// Check if user can delete (sender, or admin/owner of conversation)
	canDelete := message.SenderID == userID
	if !canDelete {
		role, err := s.GetParticipantRole(message.ConversationID, userID)
		if err == nil && (role == models.ParticipantRoleOwner || role == models.ParticipantRoleAdmin || role == models.ParticipantRoleModerator) {
			canDelete = true
		}
	}

	if !canDelete {
		return errors.New("you don't have permission to delete this message")
	}

	now := time.Now()
	if err := s.db.Model(message).Updates(map[string]interface{}{
		"deleted_at": now,
		"status":     models.MessageStatusDeleted,
	}).Error; err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	log.Printf("✅ Message %s deleted by user %s", messageID, userID)
	return nil
}

// SearchMessages searches messages in a conversation
func (s *ChatService) SearchMessages(conversationID uuid.UUID, userID, query string, page, pageSize int) ([]models.ChatMessage, int64, error) {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return nil, 0, errors.New("user is not a participant in this conversation")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var messages []models.ChatMessage
	var totalCount int64

	searchQuery := s.db.Model(&models.ChatMessage{}).
		Where("conversation_id = ? AND deleted_at IS NULL", conversationID).
		Where("content ILIKE ?", "%"+query+"%")

	// Get total count
	if err := searchQuery.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	err := searchQuery.
		Preload("Sender").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&messages).Error

	if err != nil {
		return nil, 0, err
	}

	return messages, totalCount, nil
}

// ============================================================================
// Participant Operations
// ============================================================================

// AddParticipant adds a participant to a conversation
func (s *ChatService) AddParticipant(conversationID uuid.UUID, userID string, req models.AddParticipantRequest) (*models.ChatParticipant, error) {
	// Verify requester is a participant with appropriate role
	role, err := s.GetParticipantRole(conversationID, userID)
	if err != nil {
		return nil, errors.New("you are not a participant in this conversation")
	}
	if role != models.ParticipantRoleOwner && role != models.ParticipantRoleAdmin {
		return nil, errors.New("only owner or admin can add participants")
	}

	// Check if already a participant
	if s.IsParticipant(conversationID, req.UserID) {
		return nil, errors.New("user is already a participant")
	}

	// Check max participants
	var conv models.Conversation
	if err := s.db.Select("max_participants").Where("id = ?", conversationID).First(&conv).Error; err != nil {
		return nil, err
	}

	var currentCount int64
	s.db.Model(&models.ChatParticipant{}).
		Where("conversation_id = ? AND left_at IS NULL", conversationID).
		Count(&currentCount)

	if int(currentCount) >= conv.MaxParticipants {
		return nil, errors.New("conversation has reached maximum participants")
	}

	// Set default role
	participantRole := req.Role
	if participantRole == "" {
		participantRole = models.ParticipantRoleMember
	}

	participant := &models.ChatParticipant{
		ConversationID:       conversationID,
		UserID:               req.UserID,
		Role:                 participantRole,
		JoinedAt:             time.Now(),
		NotificationsEnabled: true,
	}

	if err := s.db.Create(participant).Error; err != nil {
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}

	// Reload with user
	if err := s.db.Preload("User").First(participant, participant.ID).Error; err != nil {
		return nil, err
	}

	log.Printf("✅ Added participant %s to conversation %s by user %s", req.UserID, conversationID, userID)
	return participant, nil
}

// RemoveParticipant removes a participant from a conversation
func (s *ChatService) RemoveParticipant(conversationID uuid.UUID, userID, targetUserID string) error {
	// User can remove themselves, or owner/admin can remove others
	if userID != targetUserID {
		role, err := s.GetParticipantRole(conversationID, userID)
		if err != nil {
			return errors.New("you are not a participant in this conversation")
		}
		if role != models.ParticipantRoleOwner && role != models.ParticipantRoleAdmin {
			return errors.New("only owner or admin can remove other participants")
		}

		// Cannot remove owner
		targetRole, _ := s.GetParticipantRole(conversationID, targetUserID)
		if targetRole == models.ParticipantRoleOwner {
			return errors.New("cannot remove the owner")
		}
	}

	now := time.Now()
	result := s.db.Model(&models.ChatParticipant{}).
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, targetUserID).
		Update("left_at", now)

	if result.Error != nil {
		return fmt.Errorf("failed to remove participant: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("participant not found")
	}

	log.Printf("✅ Removed participant %s from conversation %s by user %s", targetUserID, conversationID, userID)
	return nil
}

// ListParticipants lists participants in a conversation
func (s *ChatService) ListParticipants(conversationID uuid.UUID, userID string, page, pageSize int) ([]models.ChatParticipant, int64, error) {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return nil, 0, errors.New("user is not a participant in this conversation")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	var participants []models.ChatParticipant
	var totalCount int64

	query := s.db.Model(&models.ChatParticipant{}).
		Where("conversation_id = ? AND left_at IS NULL", conversationID)

	// Get total count
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	err := query.
		Preload("User").
		Order("joined_at ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&participants).Error

	if err != nil {
		return nil, 0, err
	}

	return participants, totalCount, nil
}

// UpdateParticipantRole updates a participant's role
func (s *ChatService) UpdateParticipantRole(conversationID uuid.UUID, userID, targetUserID string, req models.UpdateParticipantRoleRequest) (*models.ChatParticipant, error) {
	// Only owner can change roles
	role, err := s.GetParticipantRole(conversationID, userID)
	if err != nil {
		return nil, errors.New("you are not a participant in this conversation")
	}
	if role != models.ParticipantRoleOwner {
		return nil, errors.New("only owner can change roles")
	}

	// Cannot change own role
	if userID == targetUserID {
		return nil, errors.New("cannot change your own role")
	}

	var participant models.ChatParticipant
	if err := s.db.
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, targetUserID).
		First(&participant).Error; err != nil {
		return nil, errors.New("participant not found")
	}

	if err := s.db.Model(&participant).Update("role", req.Role).Error; err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	// Reload with user
	if err := s.db.Preload("User").First(&participant, participant.ID).Error; err != nil {
		return nil, err
	}

	log.Printf("✅ Updated role of %s to %s in conversation %s by user %s", targetUserID, req.Role, conversationID, userID)
	return &participant, nil
}

// IsParticipant checks if a user is a participant in a conversation
func (s *ChatService) IsParticipant(conversationID uuid.UUID, userID string) bool {
	var count int64
	s.db.Model(&models.ChatParticipant{}).
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, userID).
		Count(&count)
	return count > 0
}

// GetParticipantRole gets a user's role in a conversation
func (s *ChatService) GetParticipantRole(conversationID uuid.UUID, userID string) (models.ParticipantRole, error) {
	var participant models.ChatParticipant
	err := s.db.
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, userID).
		First(&participant).Error
	if err != nil {
		return "", err
	}
	return participant.Role, nil
}

// ============================================================================
// Read Receipts & Typing Indicators
// ============================================================================

// MarkAsRead marks messages as read up to a specific message
func (s *ChatService) MarkAsRead(conversationID, messageID uuid.UUID, userID string) error {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return errors.New("user is not a participant in this conversation")
	}

	now := time.Now()

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create read receipt
		readReceipt := &models.ChatReadReceipt{
			MessageID: messageID,
			UserID:    userID,
			ReadAt:    now,
		}

		// Upsert read receipt
		if err := tx.
			Where(models.ChatReadReceipt{MessageID: messageID, UserID: userID}).
			Assign(models.ChatReadReceipt{ReadAt: now}).
			FirstOrCreate(readReceipt).Error; err != nil {
			return err
		}

		// Update participant's last read
		if err := tx.Model(&models.ChatParticipant{}).
			Where("conversation_id = ? AND user_id = ?", conversationID, userID).
			Updates(map[string]interface{}{
				"last_read_message_id": messageID,
				"last_read_at":         now,
			}).Error; err != nil {
			return err
		}

		return nil
	})

	return err
}

// SendTypingIndicator sends a typing indicator
func (s *ChatService) SendTypingIndicator(conversationID uuid.UUID, userID string) error {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return errors.New("user is not a participant in this conversation")
	}

	indicator := &models.ChatTypingIndicator{
		ConversationID: conversationID,
		UserID:         userID,
		ExpiresAt:      time.Now().Add(5 * time.Second),
	}

	// Upsert typing indicator
	if err := s.db.
		Where(models.ChatTypingIndicator{ConversationID: conversationID, UserID: userID}).
		Assign(models.ChatTypingIndicator{ExpiresAt: indicator.ExpiresAt}).
		FirstOrCreate(indicator).Error; err != nil {
		return err
	}

	return nil
}

// GetTypingUsers gets users currently typing in a conversation
func (s *ChatService) GetTypingUsers(conversationID uuid.UUID, userID string) ([]string, error) {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return nil, errors.New("user is not a participant in this conversation")
	}

	var indicators []models.ChatTypingIndicator
	err := s.db.
		Where("conversation_id = ? AND expires_at > ? AND user_id != ?", conversationID, time.Now(), userID).
		Find(&indicators).Error

	if err != nil {
		return nil, err
	}

	userIDs := make([]string, len(indicators))
	for i, ind := range indicators {
		userIDs[i] = ind.UserID
	}

	return userIDs, nil
}

// ============================================================================
// Reactions
// ============================================================================

// AddReaction adds a reaction to a message
func (s *ChatService) AddReaction(messageID uuid.UUID, userID string, req models.AddReactionRequest) (*models.ChatReaction, error) {
	// Get message to verify access
	message, err := s.GetMessage(messageID, userID)
	if err != nil {
		return nil, err
	}

	reaction := &models.ChatReaction{
		MessageID: message.ID,
		UserID:    userID,
		Reaction:  req.Reaction,
	}

	// Check if reaction already exists
	var existing models.ChatReaction
	err = s.db.
		Where("message_id = ? AND user_id = ? AND reaction = ?", messageID, userID, req.Reaction).
		First(&existing).Error

	if err == nil {
		// Already exists
		return &existing, nil
	}

	if err := s.db.Create(reaction).Error; err != nil {
		return nil, fmt.Errorf("failed to add reaction: %w", err)
	}

	log.Printf("✅ Reaction '%s' added to message %s by user %s", req.Reaction, messageID, userID)
	return reaction, nil
}

// RemoveReaction removes a reaction from a message
func (s *ChatService) RemoveReaction(messageID uuid.UUID, userID, reaction string) error {
	// Verify user has access to the message's conversation
	var message models.ChatMessage
	if err := s.db.Select("conversation_id").Where("id = ?", messageID).First(&message).Error; err != nil {
		return errors.New("message not found")
	}

	if !s.IsParticipant(message.ConversationID, userID) {
		return errors.New("user is not a participant in this conversation")
	}

	result := s.db.
		Where("message_id = ? AND user_id = ? AND reaction = ?", messageID, userID, reaction).
		Delete(&models.ChatReaction{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove reaction: %w", result.Error)
	}

	log.Printf("✅ Reaction '%s' removed from message %s by user %s", reaction, messageID, userID)
	return nil
}

// ListReactions lists reactions for a message
func (s *ChatService) ListReactions(messageID uuid.UUID, userID string) ([]models.ReactionSummaryDTO, error) {
	// Verify user has access to the message's conversation
	var message models.ChatMessage
	if err := s.db.Select("conversation_id").Where("id = ?", messageID).First(&message).Error; err != nil {
		return nil, errors.New("message not found")
	}

	if !s.IsParticipant(message.ConversationID, userID) {
		return nil, errors.New("user is not a participant in this conversation")
	}

	var reactions []models.ChatReaction
	if err := s.db.Where("message_id = ?", messageID).Find(&reactions).Error; err != nil {
		return nil, err
	}

	// Group by reaction emoji
	reactionMap := make(map[string][]string)
	for _, r := range reactions {
		reactionMap[r.Reaction] = append(reactionMap[r.Reaction], r.UserID)
	}

	summaries := make([]models.ReactionSummaryDTO, 0, len(reactionMap))
	for emoji, userIDs := range reactionMap {
		summaries = append(summaries, models.ReactionSummaryDTO{
			Reaction: emoji,
			Count:    len(userIDs),
			UserIDs:  userIDs,
		})
	}

	return summaries, nil
}

// ============================================================================
// Attachments
// ============================================================================

// SendAttachment sends an attachment to a message
func (s *ChatService) SendAttachment(conversationID, messageID uuid.UUID, userID string, req models.SendAttachmentRequest) (*models.ChatAttachment, error) {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return nil, errors.New("user is not a participant in this conversation")
	}

	// Verify message belongs to conversation
	var message models.ChatMessage
	if err := s.db.Where("id = ? AND conversation_id = ?", messageID, conversationID).First(&message).Error; err != nil {
		return nil, errors.New("message not found in conversation")
	}

	attachment := &models.ChatAttachment{
		MessageID:    messageID,
		DMSFileID:    req.DMSFileID,
		DMSFileURL:   req.DMSFileURL,
		FileName:     req.FileName,
		FileSize:     req.FileSize,
		MimeType:     req.MimeType,
		ThumbnailURL: req.ThumbnailURL,
		Metadata:     req.Metadata,
	}

	if err := s.db.Create(attachment).Error; err != nil {
		return nil, fmt.Errorf("failed to create attachment: %w", err)
	}

	log.Printf("✅ Attachment %s added to message %s by user %s", attachment.ID, messageID, userID)
	return attachment, nil
}

// ListAttachments lists attachments in a conversation
func (s *ChatService) ListAttachments(conversationID uuid.UUID, userID string, page, pageSize int) ([]models.ChatAttachment, int64, error) {
	// Verify user is a participant
	if !s.IsParticipant(conversationID, userID) {
		return nil, 0, errors.New("user is not a participant in this conversation")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var attachments []models.ChatAttachment
	var totalCount int64

	query := s.db.Model(&models.ChatAttachment{}).
		Joins("JOIN chat_messages ON chat_messages.id = chat_attachments.message_id").
		Where("chat_messages.conversation_id = ? AND chat_messages.deleted_at IS NULL", conversationID)

	// Get total count
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	err := query.
		Order("chat_attachments.created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&attachments).Error

	if err != nil {
		return nil, 0, err
	}

	return attachments, totalCount, nil
}

// ============================================================================
// Utility Functions
// ============================================================================

// GetUnreadCount gets the unread message count for a user in a conversation
func (s *ChatService) GetUnreadCount(conversationID uuid.UUID, userID string) (int64, error) {
	var participant models.ChatParticipant
	if err := s.db.
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, userID).
		First(&participant).Error; err != nil {
		return 0, err
	}

	var count int64
	query := s.db.Model(&models.ChatMessage{}).
		Where("conversation_id = ? AND deleted_at IS NULL AND sender_id != ?", conversationID, userID)

	if participant.LastReadAt != nil {
		query = query.Where("created_at > ?", *participant.LastReadAt)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// CleanupExpiredTypingIndicators removes expired typing indicators
func (s *ChatService) CleanupExpiredTypingIndicators() error {
	result := s.db.Where("expires_at < ?", time.Now()).Delete(&models.ChatTypingIndicator{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("✅ Cleaned up %d expired typing indicators", result.RowsAffected)
	}
	return nil
}

// ============================================================================
// Chat Notifications
// ============================================================================

// SendChatNotifications sends notifications to all participants (except sender) when a message is sent
func (s *ChatService) SendChatNotifications(message *models.ChatMessage, senderName string) error {
	// Get all participants in the conversation except the sender
	var participants []models.ChatParticipant
	if err := s.db.
		Preload("User").
		Where("conversation_id = ? AND user_id != ? AND left_at IS NULL AND notifications_enabled = true", message.ConversationID, message.SenderID).
		Find(&participants).Error; err != nil {
		return fmt.Errorf("failed to get participants: %w", err)
	}

	if len(participants) == 0 {
		return nil
	}

	// Get conversation details for notification title
	var conversation models.Conversation
	if err := s.db.First(&conversation, message.ConversationID).Error; err != nil {
		return fmt.Errorf("failed to get conversation: %w", err)
	}

	// Build notification title
	title := senderName
	if conversation.Type == models.ConversationTypeGroup && conversation.Title != nil && *conversation.Title != "" {
		title = fmt.Sprintf("%s in %s", senderName, *conversation.Title)
	}

	// Truncate message content for notification body
	body := message.Content
	if len(body) > 100 {
		body = body[:100] + "..."
	}

	// Create notifications for each participant
	now := time.Now()
	for _, participant := range participants {
		// Check if user has muted this conversation
		if participant.IsMuted {
			if participant.MutedUntil == nil || participant.MutedUntil.After(now) {
				continue // Skip muted participants
			}
		}

		notification := &models.Notification{
			UserID:         participant.UserID,
			Type:           models.NotificationTypeChatMessage,
			Priority:       models.NotificationPriorityNormal,
			Title:          title,
			Body:           body,
			ConversationID: &message.ConversationID,
			MessageID:      &message.ID,
			Status:         models.NotificationStatusSent,
			Channel:        models.NotificationChannelInApp,
			SentAt:         &now,
			ActionURL:      fmt.Sprintf("/chat/conversations/%s", message.ConversationID),
			Metadata: models.JSONMap{
				"sender_id":       message.SenderID,
				"sender_name":     senderName,
				"message_type":    string(message.MessageType),
				"conversation_id": message.ConversationID.String(),
			},
		}

		if err := s.db.Create(notification).Error; err != nil {
			log.Printf("⚠️ Failed to create chat notification for user %s: %v", participant.UserID, err)
			continue
		}
	}

	log.Printf("✅ Sent chat notifications for message %s to %d participants", message.ID, len(participants))
	return nil
}

// ============================================================================
// User List for Chat
// ============================================================================

// ChatUserDTO represents a user for chat user selection
type ChatUserDTO struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Email                string  `json:"email,omitempty"`
	Phone                string  `json:"phone,omitempty"`
	AvatarURL            string  `json:"avatar_url,omitempty"`
	Role                 string  `json:"role,omitempty"`
	BusinessVerticalID   string  `json:"business_vertical_id,omitempty"`
	BusinessVerticalName string  `json:"business_vertical_name,omitempty"`
	BusinessVerticalCode string  `json:"business_vertical_code,omitempty"`
	IsOnline             bool    `json:"is_online"`
}

// ListUsersForChat returns users for chat selection, sorted by business vertical
func (s *ChatService) ListUsersForChat(currentUserID string, search string, page, pageSize int) ([]ChatUserDTO, int64, error) {
	var users []models.User
	var totalCount int64

	// Use qualified column names to avoid ambiguity when joining tables
	query := s.db.Model(&models.User{}).
		Preload("BusinessVertical").
		Preload("RoleModel").
		Where("users.is_active = ?", true).
		Where("users.id != ?", currentUserID) // Exclude current user

	// Apply search filter with qualified column names
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("users.name ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", searchPattern, searchPattern, searchPattern)
	}

	// Get total count first (before join to avoid issues)
	countQuery := s.db.Model(&models.User{}).
		Where("users.is_active = ?", true).
		Where("users.id != ?", currentUserID)
	if search != "" {
		searchPattern := "%" + search + "%"
		countQuery = countQuery.Where("users.name ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", searchPattern, searchPattern, searchPattern)
	}
	if err := countQuery.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Order by business vertical name (nulls last), then by name
	offset := (page - 1) * pageSize
	if err := query.
		Joins("LEFT JOIN business_verticals ON business_verticals.id = users.business_vertical_id").
		Order("business_verticals.name NULLS LAST, users.name ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	// Convert to DTOs
	dtos := make([]ChatUserDTO, len(users))
	for i, u := range users {
		dto := ChatUserDTO{
			ID:       u.ID.String(),
			Name:     u.Name,
			Email:    u.Email,
			Phone:    u.Phone,
			IsOnline: false, // TODO: Implement online status
		}

		if u.RoleModel != nil {
			dto.Role = u.RoleModel.Name
		}

		if u.BusinessVertical != nil {
			dto.BusinessVerticalID = u.BusinessVertical.ID.String()
			dto.BusinessVerticalName = u.BusinessVertical.Name
			dto.BusinessVerticalCode = u.BusinessVertical.Code
		}

		dtos[i] = dto
	}

	return dtos, totalCount, nil
}

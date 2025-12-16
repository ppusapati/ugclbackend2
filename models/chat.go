package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ConversationType defines the type of conversation
type ConversationType string

const (
	ConversationTypeDirect  ConversationType = "direct"
	ConversationTypeGroup   ConversationType = "group"
	ConversationTypeChannel ConversationType = "channel"
)

// MessageType defines the type of message
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeImage    MessageType = "image"
	MessageTypeFile     MessageType = "file"
	MessageTypeVideo    MessageType = "video"
	MessageTypeAudio    MessageType = "audio"
	MessageTypeLocation MessageType = "location"
	MessageTypeSystem   MessageType = "system"
)

// MessageStatus defines the delivery status of a message
type MessageStatus string

const (
	MessageStatusSending   MessageStatus = "sending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
	MessageStatusDeleted   MessageStatus = "deleted"
)

// ParticipantRole defines the role of a participant in a conversation
type ParticipantRole string

const (
	ParticipantRoleOwner     ParticipantRole = "owner"
	ParticipantRoleAdmin     ParticipantRole = "admin"
	ParticipantRoleModerator ParticipantRole = "moderator"
	ParticipantRoleMember    ParticipantRole = "member"
)

// Conversation represents a chat conversation
type Conversation struct {
	ID              uuid.UUID        `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Type            ConversationType `gorm:"size:20;not null;default:'direct'" json:"type"`
	Title           *string          `gorm:"size:255" json:"title,omitempty"`
	Description     *string          `gorm:"type:text" json:"description,omitempty"`
	AvatarURL       *string          `gorm:"size:500" json:"avatar_url,omitempty"`
	Metadata        JSONMap          `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	LastMessageID   *uuid.UUID       `gorm:"type:uuid;index" json:"last_message_id,omitempty"`
	LastMessageAt   *time.Time       `json:"last_message_at,omitempty"`
	IsMuted         bool             `gorm:"default:false" json:"is_muted"`
	IsArchived      bool             `gorm:"default:false" json:"is_archived"`
	MaxParticipants int              `gorm:"default:100" json:"max_participants"`
	CreatedBy       string           `gorm:"size:255;not null" json:"created_by"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	DeletedAt       *time.Time       `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships (no FK constraint on LastMessage to avoid circular dependency)
	Participants []ChatParticipant `gorm:"foreignKey:ConversationID" json:"participants,omitempty"`
	Messages     []ChatMessage     `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
	LastMessage  *ChatMessage      `gorm:"-" json:"last_message,omitempty"` // Manual join, no FK
}

// TableName specifies the table name
func (Conversation) TableName() string {
	return "chat_conversations"
}

// ChatMessage represents a message in a conversation
type ChatMessage struct {
	ID             uuid.UUID     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConversationID uuid.UUID     `gorm:"type:uuid;not null;index" json:"conversation_id"`
	SenderID       string        `gorm:"size:255;not null;index" json:"sender_id"`
	Content        string        `gorm:"type:text;not null" json:"content"`
	MessageType    MessageType   `gorm:"size:20;not null;default:'text'" json:"message_type"`
	Status         MessageStatus `gorm:"size:20;not null;default:'sent'" json:"status"`
	ReplyToID      *uuid.UUID    `gorm:"type:uuid;index" json:"reply_to_id,omitempty"`
	Metadata       JSONMap       `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	SentAt         *time.Time    `json:"sent_at,omitempty"`
	DeliveredAt    *time.Time    `json:"delivered_at,omitempty"`
	IsEdited       bool          `gorm:"default:false" json:"is_edited"`
	EditedAt       *time.Time    `json:"edited_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	DeletedAt      *time.Time    `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Conversation *Conversation     `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
	Sender       *User             `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	ReplyTo      *ChatMessage      `gorm:"foreignKey:ReplyToID" json:"reply_to,omitempty"`
	Attachments  []ChatAttachment  `gorm:"foreignKey:MessageID" json:"attachments,omitempty"`
	Reactions    []ChatReaction    `gorm:"foreignKey:MessageID" json:"reactions,omitempty"`
	ReadReceipts []ChatReadReceipt `gorm:"foreignKey:MessageID" json:"read_receipts,omitempty"`
}

// TableName specifies the table name
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// ChatParticipant represents a participant in a conversation
type ChatParticipant struct {
	ID                       uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConversationID           uuid.UUID       `gorm:"type:uuid;not null;index:idx_participant_conv_user,unique" json:"conversation_id"`
	UserID                   string          `gorm:"size:255;not null;index:idx_participant_conv_user,unique;index" json:"user_id"`
	Role                     ParticipantRole `gorm:"size:20;not null;default:'member'" json:"role"`
	JoinedAt                 time.Time       `json:"joined_at"`
	LeftAt                   *time.Time      `json:"left_at,omitempty"`
	LastReadMessageID        *uuid.UUID      `gorm:"type:uuid" json:"last_read_message_id,omitempty"`
	LastReadAt               *time.Time      `json:"last_read_at,omitempty"`
	NotificationsEnabled     bool            `gorm:"default:true" json:"notifications_enabled"`
	MentionNotificationsOnly bool            `gorm:"default:false" json:"mention_notifications_only"`
	IsMuted                  bool            `gorm:"default:false" json:"is_muted"`
	MutedUntil               *time.Time      `json:"muted_until,omitempty"`
	Metadata                 JSONMap         `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`

	// Relationships
	Conversation *Conversation `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
	User         *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name
func (ChatParticipant) TableName() string {
	return "chat_participants"
}

// ChatAttachment represents a file attachment in a message
type ChatAttachment struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	MessageID    uuid.UUID `gorm:"type:uuid;not null;index" json:"message_id"`
	DMSFileID    *string   `gorm:"size:255" json:"dms_file_id,omitempty"`
	DMSFileURL   *string   `gorm:"size:1000" json:"dms_file_url,omitempty"`
	FileName     string    `gorm:"size:255;not null" json:"file_name"`
	FileSize     int64     `gorm:"not null" json:"file_size"`
	MimeType     string    `gorm:"size:100;not null" json:"mime_type"`
	ThumbnailURL *string   `gorm:"size:1000" json:"thumbnail_url,omitempty"`
	Metadata     JSONMap   `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at"`

	// Relationships
	Message *ChatMessage `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName specifies the table name
func (ChatAttachment) TableName() string {
	return "chat_attachments"
}

// ChatTypingIndicator represents a typing indicator
type ChatTypingIndicator struct {
	ConversationID uuid.UUID `gorm:"type:uuid;primaryKey" json:"conversation_id"`
	UserID         string    `gorm:"size:255;primaryKey" json:"user_id"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// TableName specifies the table name
func (ChatTypingIndicator) TableName() string {
	return "chat_typing_indicators"
}

// ChatReadReceipt represents a read receipt
type ChatReadReceipt struct {
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey" json:"message_id"`
	UserID    string    `gorm:"size:255;primaryKey" json:"user_id"`
	ReadAt    time.Time `json:"read_at"`

	// Relationships
	Message *ChatMessage `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName specifies the table name
func (ChatReadReceipt) TableName() string {
	return "chat_read_receipts"
}

// ChatReaction represents a reaction to a message
type ChatReaction struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	MessageID uuid.UUID `gorm:"type:uuid;not null;index:idx_reaction_message_user_emoji,unique" json:"message_id"`
	UserID    string    `gorm:"size:255;not null;index:idx_reaction_message_user_emoji,unique" json:"user_id"`
	Reaction  string    `gorm:"size:50;not null;index:idx_reaction_message_user_emoji,unique" json:"reaction"`
	CreatedAt time.Time `json:"created_at"`

	// Relationships
	Message *ChatMessage `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName specifies the table name
func (ChatReaction) TableName() string {
	return "chat_reactions"
}

// ============================================================================
// DTOs (Data Transfer Objects)
// ============================================================================

// ConversationDTO represents the API response format for a conversation
type ConversationDTO struct {
	ID               uuid.UUID              `json:"id"`
	Type             ConversationType       `json:"type"`
	Title            *string                `json:"title,omitempty"`
	Description      *string                `json:"description,omitempty"`
	AvatarURL        *string                `json:"avatar_url,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	LastMessageID    *uuid.UUID             `json:"last_message_id,omitempty"`
	LastMessageAt    *time.Time             `json:"last_message_at,omitempty"`
	IsMuted          bool                   `json:"is_muted"`
	IsArchived       bool                   `json:"is_archived"`
	MaxParticipants  int                    `json:"max_participants"`
	CreatedBy        string                 `json:"created_by"`
	CreatedAt        time.Time              `json:"created_at"`
	UnreadCount      int                    `json:"unread_count,omitempty"`
	LastMessage      *MessageDTO            `json:"last_message,omitempty"`
	Participants     []ParticipantDTO       `json:"participants,omitempty"`
	OtherParticipant *ParticipantDTO        `json:"other_participant,omitempty"` // For direct conversations - the other user
}

// ToDTO converts Conversation to ConversationDTO
func (c *Conversation) ToDTO() ConversationDTO {
	dto := ConversationDTO{
		ID:              c.ID,
		Type:            c.Type,
		Title:           c.Title,
		Description:     c.Description,
		AvatarURL:       c.AvatarURL,
		Metadata:        c.Metadata,
		LastMessageID:   c.LastMessageID,
		LastMessageAt:   c.LastMessageAt,
		IsMuted:         c.IsMuted,
		IsArchived:      c.IsArchived,
		MaxParticipants: c.MaxParticipants,
		CreatedBy:       c.CreatedBy,
		CreatedAt:       c.CreatedAt,
	}

	if c.LastMessage != nil {
		lastMsgDTO := c.LastMessage.ToDTO()
		dto.LastMessage = &lastMsgDTO
	}

	if len(c.Participants) > 0 {
		dto.Participants = make([]ParticipantDTO, len(c.Participants))
		for i, p := range c.Participants {
			dto.Participants[i] = p.ToDTO()
		}
	}

	return dto
}

// ToDTOForUser converts Conversation to ConversationDTO with user context
// For direct conversations, sets OtherParticipant to the other user (not the current user)
func (c *Conversation) ToDTOForUser(currentUserID string) ConversationDTO {
	dto := c.ToDTO()

	// For direct conversations, find and set the other participant
	if c.Type == ConversationTypeDirect && len(c.Participants) > 0 {
		for _, p := range c.Participants {
			if p.UserID != currentUserID && p.LeftAt == nil {
				pDTO := p.ToDTO()
				dto.OtherParticipant = &pDTO
				// For direct chats, use other participant's name as title if no title set
				if dto.Title == nil || *dto.Title == "" {
					if p.User != nil && p.User.Name != "" {
						dto.Title = &p.User.Name
					}
				}
				break
			}
		}
	}

	return dto
}

// MessageDTO represents the API response format for a message
type MessageDTO struct {
	ID              uuid.UUID              `json:"id"`
	ConversationID  uuid.UUID              `json:"conversation_id"`
	SenderID        string                 `json:"sender_id"`
	SenderName      string                 `json:"sender_name,omitempty"`
	SenderAvatarURL *string                `json:"sender_avatar_url,omitempty"`
	Content         string                 `json:"content"`
	MessageType     MessageType            `json:"message_type"`
	Status          MessageStatus          `json:"status"`
	ReplyToID       *uuid.UUID             `json:"reply_to_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	SentAt          *time.Time             `json:"sent_at,omitempty"`
	DeliveredAt     *time.Time             `json:"delivered_at,omitempty"`
	IsEdited        bool                   `json:"is_edited"`
	EditedAt        *time.Time             `json:"edited_at,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	Attachments     []AttachmentDTO        `json:"attachments,omitempty"`
	Reactions       []ReactionSummaryDTO   `json:"reactions,omitempty"`
	ReadCount       int                    `json:"read_count,omitempty"`
}

// ToDTO converts ChatMessage to MessageDTO
func (m *ChatMessage) ToDTO() MessageDTO {
	dto := MessageDTO{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderID:       m.SenderID,
		Content:        m.Content,
		MessageType:    m.MessageType,
		Status:         m.Status,
		ReplyToID:      m.ReplyToID,
		Metadata:       m.Metadata,
		SentAt:         m.SentAt,
		DeliveredAt:    m.DeliveredAt,
		IsEdited:       m.IsEdited,
		EditedAt:       m.EditedAt,
		CreatedAt:      m.CreatedAt,
	}

	// Populate sender info if available
	if m.Sender != nil {
		dto.SenderName = m.Sender.Name
		// Note: User model doesn't have AvatarURL field yet, add if needed
	}

	if len(m.Attachments) > 0 {
		dto.Attachments = make([]AttachmentDTO, len(m.Attachments))
		for i, a := range m.Attachments {
			dto.Attachments[i] = a.ToDTO()
		}
	}

	// Group reactions by emoji
	if len(m.Reactions) > 0 {
		reactionMap := make(map[string][]string)
		for _, r := range m.Reactions {
			reactionMap[r.Reaction] = append(reactionMap[r.Reaction], r.UserID)
		}
		for emoji, userIDs := range reactionMap {
			dto.Reactions = append(dto.Reactions, ReactionSummaryDTO{
				Reaction: emoji,
				Count:    len(userIDs),
				UserIDs:  userIDs,
			})
		}
	}

	dto.ReadCount = len(m.ReadReceipts)

	return dto
}

// ParticipantDTO represents the API response format for a participant
type ParticipantDTO struct {
	UserID                   string          `json:"user_id"`
	Role                     ParticipantRole `json:"role"`
	JoinedAt                 time.Time       `json:"joined_at"`
	LeftAt                   *time.Time      `json:"left_at,omitempty"`
	LastReadMessageID        *uuid.UUID      `json:"last_read_message_id,omitempty"`
	LastReadAt               *time.Time      `json:"last_read_at,omitempty"`
	NotificationsEnabled     bool            `json:"notifications_enabled"`
	MentionNotificationsOnly bool            `json:"mention_notifications_only"`
	IsMuted                  bool            `json:"is_muted"`
	MutedUntil               *time.Time      `json:"muted_until,omitempty"`
	UserName                 string          `json:"user_name,omitempty"`
	UserEmail                string          `json:"user_email,omitempty"`
}

// ToDTO converts ChatParticipant to ParticipantDTO
func (p *ChatParticipant) ToDTO() ParticipantDTO {
	dto := ParticipantDTO{
		UserID:                   p.UserID,
		Role:                     p.Role,
		JoinedAt:                 p.JoinedAt,
		LeftAt:                   p.LeftAt,
		LastReadMessageID:        p.LastReadMessageID,
		LastReadAt:               p.LastReadAt,
		NotificationsEnabled:     p.NotificationsEnabled,
		MentionNotificationsOnly: p.MentionNotificationsOnly,
		IsMuted:                  p.IsMuted,
		MutedUntil:               p.MutedUntil,
	}

	if p.User != nil {
		dto.UserName = p.User.Name
		dto.UserEmail = p.User.Email
	}

	return dto
}

// AttachmentDTO represents the API response format for an attachment
type AttachmentDTO struct {
	ID           uuid.UUID              `json:"id"`
	MessageID    uuid.UUID              `json:"message_id"`
	DMSFileID    *string                `json:"dms_file_id,omitempty"`
	DMSFileURL   *string                `json:"dms_file_url,omitempty"`
	FileName     string                 `json:"file_name"`
	FileSize     int64                  `json:"file_size"`
	MimeType     string                 `json:"mime_type"`
	ThumbnailURL *string                `json:"thumbnail_url,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// ToDTO converts ChatAttachment to AttachmentDTO
func (a *ChatAttachment) ToDTO() AttachmentDTO {
	return AttachmentDTO{
		ID:           a.ID,
		MessageID:    a.MessageID,
		DMSFileID:    a.DMSFileID,
		DMSFileURL:   a.DMSFileURL,
		FileName:     a.FileName,
		FileSize:     a.FileSize,
		MimeType:     a.MimeType,
		ThumbnailURL: a.ThumbnailURL,
		Metadata:     a.Metadata,
		CreatedAt:    a.CreatedAt,
	}
}

// ReactionSummaryDTO represents a summary of reactions for an emoji
type ReactionSummaryDTO struct {
	Reaction string   `json:"reaction"`
	Count    int      `json:"count"`
	UserIDs  []string `json:"user_ids"`
}

// ReactionDTO represents the API response format for a reaction
type ReactionDTO struct {
	ID        uuid.UUID `json:"id"`
	MessageID uuid.UUID `json:"message_id"`
	UserID    string    `json:"user_id"`
	Reaction  string    `json:"reaction"`
	CreatedAt time.Time `json:"created_at"`
}

// ToDTO converts ChatReaction to ReactionDTO
func (r *ChatReaction) ToDTO() ReactionDTO {
	return ReactionDTO{
		ID:        r.ID,
		MessageID: r.MessageID,
		UserID:    r.UserID,
		Reaction:  r.Reaction,
		CreatedAt: r.CreatedAt,
	}
}

// ============================================================================
// Request Types
// ============================================================================

// CreateConversationRequest represents the request to create a conversation
type CreateConversationRequest struct {
	Type               ConversationType       `json:"type" validate:"required,oneof=direct group channel"`
	Title              *string                `json:"title,omitempty"`
	Description        *string                `json:"description,omitempty"`
	AvatarURL          *string                `json:"avatar_url,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	ParticipantIDs     []string               `json:"participant_ids"`
	ParticipantUserIDs []string               `json:"participant_user_ids"` // Alias for participant_ids (mobile app compatibility)
	MaxParticipants    int                    `json:"max_participants,omitempty"`
}

// GetParticipantIDs returns participant IDs from either field
func (r *CreateConversationRequest) GetParticipantIDs() []string {
	if len(r.ParticipantIDs) > 0 {
		return r.ParticipantIDs
	}
	return r.ParticipantUserIDs
}

// CreateGroupRequest represents the request to create a group (admin only)
type CreateGroupRequest struct {
	Title           string                 `json:"title" validate:"required"`
	Description     *string                `json:"description,omitempty"`
	AvatarURL       *string                `json:"avatar_url,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	MemberIDs       []string               `json:"member_ids" validate:"required,min=1"`
	MaxParticipants int                    `json:"max_participants,omitempty"`
}

// SendMessageRequest represents the request to send a message
type SendMessageRequest struct {
	Content     string                 `json:"content" validate:"required"`
	MessageType MessageType            `json:"message_type,omitempty"`
	ReplyToID   *uuid.UUID             `json:"reply_to_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateMessageRequest represents the request to update a message
type UpdateMessageRequest struct {
	Content string `json:"content" validate:"required"`
}

// UpdateConversationRequest represents the request to update a conversation
type UpdateConversationRequest struct {
	Title           *string                `json:"title,omitempty"`
	Description     *string                `json:"description,omitempty"`
	AvatarURL       *string                `json:"avatar_url,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	MaxParticipants *int                   `json:"max_participants,omitempty"`
}

// AddParticipantRequest represents the request to add a participant
type AddParticipantRequest struct {
	UserID string          `json:"user_id" validate:"required"`
	Role   ParticipantRole `json:"role,omitempty"`
}

// UpdateParticipantRoleRequest represents the request to update a participant's role
type UpdateParticipantRoleRequest struct {
	Role ParticipantRole `json:"role" validate:"required,oneof=owner admin moderator member"`
}

// AddReactionRequest represents the request to add a reaction
type AddReactionRequest struct {
	Reaction string `json:"reaction" validate:"required,max=50"`
}

// SendAttachmentRequest represents the request to send an attachment
type SendAttachmentRequest struct {
	DMSFileID    *string                `json:"dms_file_id,omitempty"`
	DMSFileURL   *string                `json:"dms_file_url,omitempty"`
	FileName     string                 `json:"file_name" validate:"required"`
	FileSize     int64                  `json:"file_size" validate:"required"`
	MimeType     string                 `json:"mime_type" validate:"required"`
	ThumbnailURL *string                `json:"thumbnail_url,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ============================================================================
// Custom Types for GORM
// ============================================================================

// ConversationTypeArray is a custom type for conversation types
type ConversationTypeArray []ConversationType

// Scan implements the sql.Scanner interface
func (c *ConversationTypeArray) Scan(value interface{}) error {
	if value == nil {
		*c = []ConversationType{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*c = []ConversationType{}
		return nil
	}

	var types []string
	if err := json.Unmarshal(bytes, &types); err != nil {
		return err
	}

	result := make([]ConversationType, len(types))
	for i, t := range types {
		result[i] = ConversationType(t)
	}
	*c = result
	return nil
}

// Value implements the driver.Valuer interface
func (c ConversationTypeArray) Value() (driver.Value, error) {
	if c == nil {
		c = []ConversationType{}
	}

	types := make([]string, len(c))
	for i, t := range c {
		types[i] = string(t)
	}
	return json.Marshal(types)
}

// GormDataType defines the data type for GORM
func (ConversationTypeArray) GormDataType() string {
	return "jsonb"
}

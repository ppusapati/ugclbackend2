package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// ChatHandler handles chat HTTP endpoints
type ChatHandler struct{}

var chatServiceInstance *ChatService

func getChatService() *ChatService {
	if chatServiceInstance == nil {
		chatServiceInstance = NewChatService()
	}
	return chatServiceInstance
}

// ============================================================================
// Conversation Handlers
// ============================================================================

// CreateConversation creates a new conversation
// POST /api/v1/chat/conversations
func (h *ChatHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Type == "" {
		req.Type = models.ConversationTypeDirect
	}
	// Use helper method that checks both participant_ids and participant_user_ids
	if len(req.GetParticipantIDs()) == 0 {
		http.Error(w, "participant_ids or participant_user_ids is required", http.StatusBadRequest)
		return
	}

	conversation, err := getChatService().CreateConversation(claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error creating conversation: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "conversation created successfully",
		"conversation": conversation.ToDTOForUser(claims.UserID),
	})
}

// CreateGroup creates a new group (admin only)
// POST /api/v1/chat/groups
func (h *ChatHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if len(req.MemberIDs) == 0 {
		http.Error(w, "member_ids is required", http.StatusBadRequest)
		return
	}

	group, err := getChatService().CreateGroup(claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error creating group: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "group created successfully",
		"group":   group.ToDTOForUser(claims.UserID),
	})
}

// GetConversation retrieves a conversation by ID
// GET /api/v1/chat/conversations/{id}
func (h *ChatHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	conversation, err := getChatService().GetConversation(conversationID, claims.UserID)
	if err != nil {
		log.Printf("❌ Error getting conversation: %v", err)
		if err.Error() == "conversation not found" || err.Error() == "user is not a participant in this conversation" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get conversation", http.StatusInternalServerError)
		return
	}

	// Get unread count
	unreadCount, _ := getChatService().GetUnreadCount(conversationID, claims.UserID)

	dto := conversation.ToDTOForUser(claims.UserID)
	dto.UnreadCount = int(unreadCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conversation": dto,
	})
}

// ListConversations lists conversations for the current user
// GET /api/v1/chat/conversations
func (h *ChatHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	includeArchived := r.URL.Query().Get("include_archived") == "true"

	var convType *models.ConversationType
	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		ct := models.ConversationType(typeParam)
		convType = &ct
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	conversations, totalCount, err := getChatService().ListUserConversations(claims.UserID, page, pageSize, includeArchived, convType)
	if err != nil {
		log.Printf("❌ Error listing conversations: %v", err)
		http.Error(w, "failed to list conversations", http.StatusInternalServerError)
		return
	}

	// Convert to DTOs and add unread counts
	dtos := make([]models.ConversationDTO, len(conversations))
	for i, conv := range conversations {
		dtos[i] = conv.ToDTOForUser(claims.UserID)
		unreadCount, _ := getChatService().GetUnreadCount(conv.ID, claims.UserID)
		dtos[i].UnreadCount = int(unreadCount)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conversations": dtos,
		"total_count":   totalCount,
		"page":          page,
		"page_size":     pageSize,
		"has_more":      int64(page*pageSize) < totalCount,
	})
}

// UpdateConversation updates a conversation
// PUT /api/v1/chat/conversations/{id}
func (h *ChatHandler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	conversation, err := getChatService().UpdateConversation(conversationID, claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error updating conversation: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "conversation updated successfully",
		"conversation": conversation.ToDTOForUser(claims.UserID),
	})
}

// DeleteConversation deletes a conversation
// DELETE /api/v1/chat/conversations/{id}
func (h *ChatHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	if err := getChatService().DeleteConversation(conversationID, claims.UserID); err != nil {
		log.Printf("❌ Error deleting conversation: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ArchiveConversation archives or unarchives a conversation
// PATCH /api/v1/chat/conversations/{id}/archive
func (h *ChatHandler) ArchiveConversation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Archive bool `json:"archive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	conversation, err := getChatService().ArchiveConversation(conversationID, claims.UserID, req.Archive)
	if err != nil {
		log.Printf("❌ Error archiving conversation: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	action := "archived"
	if !req.Archive {
		action = "unarchived"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "conversation " + action + " successfully",
		"conversation": conversation.ToDTOForUser(claims.UserID),
	})
}

// ============================================================================
// Message Handlers
// ============================================================================

// SendMessage sends a message to a conversation
// POST /api/v1/chat/conversations/{id}/messages
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	var req models.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	message, err := getChatService().SendMessage(conversationID, claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error sending message: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Send notifications to other participants (async, don't block response)
	go func() {
		if err := getChatService().SendChatNotifications(message, claims.Name); err != nil {
			log.Printf("⚠️ Error sending chat notifications: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": message.ToDTO(),
	})
}

// GetMessage retrieves a message by ID
// GET /api/v1/chat/messages/{id}
func (h *ChatHandler) GetMessage(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	messageID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	message, err := getChatService().GetMessage(messageID, claims.UserID)
	if err != nil {
		log.Printf("❌ Error getting message: %v", err)
		if err.Error() == "message not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": message.ToDTO(),
	})
}

// ListMessages lists messages in a conversation
// GET /api/v1/chat/conversations/{id}/messages
func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	var beforeMessageID, afterMessageID *uuid.UUID
	if beforeID := r.URL.Query().Get("before"); beforeID != "" {
		if id, err := uuid.Parse(beforeID); err == nil {
			beforeMessageID = &id
		}
	}
	if afterID := r.URL.Query().Get("after"); afterID != "" {
		if id, err := uuid.Parse(afterID); err == nil {
			afterMessageID = &id
		}
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	messages, totalCount, hasMore, err := getChatService().ListMessages(conversationID, claims.UserID, page, pageSize, beforeMessageID, afterMessageID)
	if err != nil {
		log.Printf("❌ Error listing messages: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert to DTOs
	dtos := make([]models.MessageDTO, len(messages))
	for i, msg := range messages {
		dtos[i] = msg.ToDTO()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages":    dtos,
		"total_count": totalCount,
		"has_more":    hasMore,
	})
}

// UpdateMessage updates a message
// PUT /api/v1/chat/messages/{id}
func (h *ChatHandler) UpdateMessage(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	messageID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	message, err := getChatService().UpdateMessage(messageID, claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error updating message: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": message.ToDTO(),
	})
}

// DeleteMessage deletes a message
// DELETE /api/v1/chat/messages/{id}
func (h *ChatHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	messageID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	if err := getChatService().DeleteMessage(messageID, claims.UserID); err != nil {
		log.Printf("❌ Error deleting message: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SearchMessages searches messages in a conversation
// GET /api/v1/chat/conversations/{id}/messages/search
func (h *ChatHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "search query is required", http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	messages, totalCount, err := getChatService().SearchMessages(conversationID, claims.UserID, query, page, pageSize)
	if err != nil {
		log.Printf("❌ Error searching messages: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert to DTOs
	dtos := make([]models.MessageDTO, len(messages))
	for i, msg := range messages {
		dtos[i] = msg.ToDTO()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages":    dtos,
		"total_count": totalCount,
	})
}

// ============================================================================
// Participant Handlers
// ============================================================================

// AddParticipant adds a participant to a conversation
// POST /api/v1/chat/conversations/{id}/participants
func (h *ChatHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	var req models.AddParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	participant, err := getChatService().AddParticipant(conversationID, claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error adding participant: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "participant added successfully",
		"participant": participant.ToDTO(),
	})
}

// RemoveParticipant removes a participant from a conversation
// DELETE /api/v1/chat/conversations/{id}/participants/{userId}
func (h *ChatHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	targetUserID := vars["userId"]
	if targetUserID == "" {
		http.Error(w, "user ID is required", http.StatusBadRequest)
		return
	}

	if err := getChatService().RemoveParticipant(conversationID, claims.UserID, targetUserID); err != nil {
		log.Printf("❌ Error removing participant: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListParticipants lists participants in a conversation
// GET /api/v1/chat/conversations/{id}/participants
func (h *ChatHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	participants, totalCount, err := getChatService().ListParticipants(conversationID, claims.UserID, page, pageSize)
	if err != nil {
		log.Printf("❌ Error listing participants: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert to DTOs
	dtos := make([]models.ParticipantDTO, len(participants))
	for i, p := range participants {
		dtos[i] = p.ToDTO()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"participants": dtos,
		"total_count":  totalCount,
	})
}

// UpdateParticipantRole updates a participant's role
// PATCH /api/v1/chat/conversations/{id}/participants/{userId}/role
func (h *ChatHandler) UpdateParticipantRole(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	targetUserID := vars["userId"]
	if targetUserID == "" {
		http.Error(w, "user ID is required", http.StatusBadRequest)
		return
	}

	var req models.UpdateParticipantRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		http.Error(w, "role is required", http.StatusBadRequest)
		return
	}

	participant, err := getChatService().UpdateParticipantRole(conversationID, claims.UserID, targetUserID, req)
	if err != nil {
		log.Printf("❌ Error updating participant role: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "role updated successfully",
		"participant": participant.ToDTO(),
	})
}

// ============================================================================
// Read Receipts & Typing Indicators
// ============================================================================

// MarkAsRead marks messages as read
// POST /api/v1/chat/conversations/{id}/read
func (h *ChatHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	messageID, err := uuid.Parse(req.MessageID)
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	if err := getChatService().MarkAsRead(conversationID, messageID, claims.UserID); err != nil {
		log.Printf("❌ Error marking as read: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// SendTypingIndicator sends a typing indicator
// POST /api/v1/chat/conversations/{id}/typing
func (h *ChatHandler) SendTypingIndicator(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	if err := getChatService().SendTypingIndicator(conversationID, claims.UserID); err != nil {
		log.Printf("❌ Error sending typing indicator: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// GetTypingUsers gets users currently typing
// GET /api/v1/chat/conversations/{id}/typing
func (h *ChatHandler) GetTypingUsers(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	userIDs, err := getChatService().GetTypingUsers(conversationID, claims.UserID)
	if err != nil {
		log.Printf("❌ Error getting typing users: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_ids": userIDs,
	})
}

// ============================================================================
// Reactions
// ============================================================================

// AddReaction adds a reaction to a message
// POST /api/v1/chat/messages/{id}/reactions
func (h *ChatHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	messageID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	var req models.AddReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Reaction == "" {
		http.Error(w, "reaction is required", http.StatusBadRequest)
		return
	}

	reaction, err := getChatService().AddReaction(messageID, claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error adding reaction: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"reaction": reaction.ToDTO(),
	})
}

// RemoveReaction removes a reaction from a message
// DELETE /api/v1/chat/messages/{id}/reactions/{reaction}
func (h *ChatHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	messageID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	reaction := vars["reaction"]
	if reaction == "" {
		http.Error(w, "reaction is required", http.StatusBadRequest)
		return
	}

	if err := getChatService().RemoveReaction(messageID, claims.UserID, reaction); err != nil {
		log.Printf("❌ Error removing reaction: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListReactions lists reactions for a message
// GET /api/v1/chat/messages/{id}/reactions
func (h *ChatHandler) ListReactions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	messageID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	reactions, err := getChatService().ListReactions(messageID, claims.UserID)
	if err != nil {
		log.Printf("❌ Error listing reactions: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"reactions": reactions,
	})
}

// ============================================================================
// Attachments
// ============================================================================

// SendAttachment sends an attachment
// POST /api/v1/chat/conversations/{id}/messages/{messageId}/attachments
func (h *ChatHandler) SendAttachment(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	messageID, err := uuid.Parse(vars["messageId"])
	if err != nil {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	var req models.SendAttachmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.FileName == "" || req.MimeType == "" {
		http.Error(w, "file_name and mime_type are required", http.StatusBadRequest)
		return
	}

	attachment, err := getChatService().SendAttachment(conversationID, messageID, claims.UserID, req)
	if err != nil {
		log.Printf("❌ Error sending attachment: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"attachment": attachment.ToDTO(),
	})
}

// ListAttachments lists attachments in a conversation
// GET /api/v1/chat/conversations/{id}/attachments
func (h *ChatHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	conversationID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid conversation ID", http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	attachments, totalCount, err := getChatService().ListAttachments(conversationID, claims.UserID, page, pageSize)
	if err != nil {
		log.Printf("❌ Error listing attachments: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert to DTOs
	dtos := make([]models.AttachmentDTO, len(attachments))
	for i, a := range attachments {
		dtos[i] = a.ToDTO()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"attachments": dtos,
		"total_count": totalCount,
	})
}

// ============================================================================
// User List for Chat
// ============================================================================

// ListUsersForChat returns all users grouped by business vertical for starting conversations
// GET /api/v1/chat/users
func (h *ChatHandler) ListUsersForChat(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	search := r.URL.Query().Get("search")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 100
	}

	users, totalCount, err := getChatService().ListUsersForChat(claims.UserID, search, page, pageSize)
	if err != nil {
		log.Printf("❌ Error listing users for chat: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users":       users,
		"total_count": totalCount,
		"page":        page,
		"page_size":   pageSize,
	})
}

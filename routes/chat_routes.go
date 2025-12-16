package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterChatRoutes registers all chat-related routes
// Note: Most chat endpoints only require authentication, not specific permissions.
// The service layer checks if the user is a participant in the conversation.
// Admin-only operations (like creating groups) still require specific permissions.
func RegisterChatRoutes(api *mux.Router) {
	chatHandler := &handlers.ChatHandler{}

	// Chat routes - all require authentication
	// Base path: /api/v1/chat

	chat := api.PathPrefix("/chat").Subrouter()

	// ============================================================================
	// User list for starting conversations
	// ============================================================================

	// List users for chat (sorted by business vertical)
	// GET /api/v1/chat/users
	chat.HandleFunc("/users", chatHandler.ListUsersForChat).Methods("GET")

	// ============================================================================
	// Conversation endpoints
	// ============================================================================

	// Create a new direct conversation (any authenticated user can create)
	// POST /api/v1/chat/conversations
	chat.HandleFunc("/conversations", chatHandler.CreateConversation).Methods("POST")

	// Create a new group (admin only - requires special permission)
	// POST /api/v1/chat/groups
	chat.Handle("/groups", middleware.RequirePermission("chat:group:create")(
		http.HandlerFunc(chatHandler.CreateGroup))).Methods("POST")

	// List user's conversations (only returns conversations where user is participant)
	// GET /api/v1/chat/conversations
	chat.HandleFunc("/conversations", chatHandler.ListConversations).Methods("GET")

	// Get a specific conversation (service checks if user is participant)
	// GET /api/v1/chat/conversations/{id}
	chat.HandleFunc("/conversations/{id}", chatHandler.GetConversation).Methods("GET")

	// Update a conversation (service checks if user is owner/admin)
	// PUT /api/v1/chat/conversations/{id}
	chat.HandleFunc("/conversations/{id}", chatHandler.UpdateConversation).Methods("PUT")

	// Delete a conversation (service checks if user is owner)
	// DELETE /api/v1/chat/conversations/{id}
	chat.HandleFunc("/conversations/{id}", chatHandler.DeleteConversation).Methods("DELETE")

	// Archive/unarchive a conversation (service checks if user is participant)
	// PATCH /api/v1/chat/conversations/{id}/archive
	chat.HandleFunc("/conversations/{id}/archive", chatHandler.ArchiveConversation).Methods("PATCH")

	// ============================================================================
	// Message endpoints
	// ============================================================================

	// Send a message to a conversation (service checks if user is participant)
	// POST /api/v1/chat/conversations/{id}/messages
	chat.HandleFunc("/conversations/{id}/messages", chatHandler.SendMessage).Methods("POST")

	// List messages in a conversation (service checks if user is participant)
	// GET /api/v1/chat/conversations/{id}/messages
	chat.HandleFunc("/conversations/{id}/messages", chatHandler.ListMessages).Methods("GET")

	// Search messages in a conversation (service checks if user is participant)
	// GET /api/v1/chat/conversations/{id}/messages/search
	chat.HandleFunc("/conversations/{id}/messages/search", chatHandler.SearchMessages).Methods("GET")

	// Get a specific message (service checks if user is participant in conversation)
	// GET /api/v1/chat/messages/{id}
	chat.HandleFunc("/messages/{id}", chatHandler.GetMessage).Methods("GET")

	// Update a message (service checks if user is the sender)
	// PUT /api/v1/chat/messages/{id}
	chat.HandleFunc("/messages/{id}", chatHandler.UpdateMessage).Methods("PUT")

	// Delete a message (service checks if user is sender or admin)
	// DELETE /api/v1/chat/messages/{id}
	chat.HandleFunc("/messages/{id}", chatHandler.DeleteMessage).Methods("DELETE")

	// ============================================================================
	// Participant endpoints
	// ============================================================================

	// Add a participant to a conversation (service checks if user is owner/admin)
	// POST /api/v1/chat/conversations/{id}/participants
	chat.HandleFunc("/conversations/{id}/participants", chatHandler.AddParticipant).Methods("POST")

	// List participants in a conversation (service checks if user is participant)
	// GET /api/v1/chat/conversations/{id}/participants
	chat.HandleFunc("/conversations/{id}/participants", chatHandler.ListParticipants).Methods("GET")

	// Remove a participant from a conversation (service checks permissions)
	// DELETE /api/v1/chat/conversations/{id}/participants/{userId}
	chat.HandleFunc("/conversations/{id}/participants/{userId}", chatHandler.RemoveParticipant).Methods("DELETE")

	// Update a participant's role (service checks if user is owner/admin)
	// PATCH /api/v1/chat/conversations/{id}/participants/{userId}/role
	chat.HandleFunc("/conversations/{id}/participants/{userId}/role", chatHandler.UpdateParticipantRole).Methods("PATCH")

	// ============================================================================
	// Read receipts & Typing indicators
	// ============================================================================

	// Mark messages as read (service checks if user is participant)
	// POST /api/v1/chat/conversations/{id}/read
	chat.HandleFunc("/conversations/{id}/read", chatHandler.MarkAsRead).Methods("POST")

	// Send typing indicator (service checks if user is participant)
	// POST /api/v1/chat/conversations/{id}/typing
	chat.HandleFunc("/conversations/{id}/typing", chatHandler.SendTypingIndicator).Methods("POST")

	// Get typing users (service checks if user is participant)
	// GET /api/v1/chat/conversations/{id}/typing
	chat.HandleFunc("/conversations/{id}/typing", chatHandler.GetTypingUsers).Methods("GET")

	// ============================================================================
	// Reaction endpoints
	// ============================================================================

	// Add a reaction to a message (service checks if user is participant)
	// POST /api/v1/chat/messages/{id}/reactions
	chat.HandleFunc("/messages/{id}/reactions", chatHandler.AddReaction).Methods("POST")

	// List reactions for a message (service checks if user is participant)
	// GET /api/v1/chat/messages/{id}/reactions
	chat.HandleFunc("/messages/{id}/reactions", chatHandler.ListReactions).Methods("GET")

	// Remove a reaction from a message (service checks if user added the reaction)
	// DELETE /api/v1/chat/messages/{id}/reactions/{reaction}
	chat.HandleFunc("/messages/{id}/reactions/{reaction}", chatHandler.RemoveReaction).Methods("DELETE")

	// ============================================================================
	// Attachment endpoints
	// ============================================================================

	// Send an attachment (service checks if user is participant)
	// POST /api/v1/chat/conversations/{id}/messages/{messageId}/attachments
	chat.HandleFunc("/conversations/{id}/messages/{messageId}/attachments", chatHandler.SendAttachment).Methods("POST")

	// List attachments in a conversation (service checks if user is participant)
	// GET /api/v1/chat/conversations/{id}/attachments
	chat.HandleFunc("/conversations/{id}/attachments", chatHandler.ListAttachments).Methods("GET")
}

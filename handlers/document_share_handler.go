package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// CreateDocumentShareHandler creates a shareable link for a document
func CreateDocumentShareHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var req struct {
		AccessLevel string `json:"access_level"`
		CanDownload bool   `json:"can_download"`
		Password    string `json:"password"`
		MaxAccess   int    `json:"max_access"`
		ExpiresAt   string `json:"expires_at"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Verify document exists
	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Generate random share token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "failed to generate token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	shareToken := hex.EncodeToString(tokenBytes)

	// Parse expiration date
	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		parsedTime, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err == nil {
			expiresAt = &parsedTime
		}
	}

	// Hash password if provided
	var hashedPassword string
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to hash password: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hashedPassword = string(hash)
	}

	// Set default access level
	accessLevel := models.DocumentAccessView
	if req.AccessLevel != "" {
		accessLevel = models.DocumentAccessLevel(req.AccessLevel)
	}

	// Create share
	share := models.DocumentShare{
		DocumentID:  document.ID,
		ShareToken:  shareToken,
		AccessLevel: accessLevel,
		CanDownload: req.CanDownload,
		Password:    hashedPassword,
		MaxAccess:   req.MaxAccess,
		ExpiresAt:   expiresAt,
		CreatedByID: userID,
		IsActive:    true,
	}

	if err := config.DB.Create(&share).Error; err != nil {
		http.Error(w, "failed to create share: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionShare,
		Details:    models.DocumentMetadata{"share_token": shareToken, "access_level": accessLevel},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	// Build share URL
	baseURL := r.Header.Get("Origin")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	shareURL := baseURL + "/api/v1/documents/shared/" + shareToken

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Share link created successfully",
		"share_url": shareURL,
		"share":     share,
	})
}

// GetDocumentSharesHandler returns all shares for a document
func GetDocumentSharesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	var shares []models.DocumentShare
	if err := config.DB.Preload("CreatedBy").Where("document_id = ?", documentID).
		Order("created_at DESC").Find(&shares).Error; err != nil {
		http.Error(w, "failed to fetch shares: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shares)
}

// AccessSharedDocumentHandler handles access to a shared document
func AccessSharedDocumentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shareToken := vars["token"]

	var share models.DocumentShare
	if err := config.DB.Preload("Document").Preload("Document.Category").Preload("Document.Tags").
		First(&share, "share_token = ?", shareToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "share link not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch share: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if share is active
	if !share.IsActive {
		http.Error(w, "share link is inactive", http.StatusForbidden)
		return
	}

	// Check expiration
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "share link has expired", http.StatusForbidden)
		return
	}

	// Check max access
	if share.MaxAccess > 0 && share.AccessCount >= share.MaxAccess {
		http.Error(w, "share link has reached maximum access count", http.StatusForbidden)
		return
	}

	// Check password if required
	if share.Password != "" {
		password := r.URL.Query().Get("password")
		if password == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"password_required": true,
				"message":           "Password is required to access this document",
			})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(share.Password), []byte(password)); err != nil {
			http.Error(w, "invalid password", http.StatusUnauthorized)
			return
		}
	}

	// Increment access count
	config.DB.Model(&share).Update("access_count", gorm.Expr("access_count + 1"))

	// Log access
	auditLog := models.DocumentAuditLog{
		DocumentID: share.DocumentID,
		Action:     models.DocumentAuditActionView,
		Details:    models.DocumentMetadata{"via_share": true, "share_token": shareToken},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"document":     share.Document,
		"access_level": share.AccessLevel,
		"can_download": share.CanDownload,
	})
}

// DownloadSharedDocumentHandler handles downloading a shared document
func DownloadSharedDocumentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shareToken := vars["token"]

	var share models.DocumentShare
	if err := config.DB.Preload("Document").First(&share, "share_token = ?", shareToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "share link not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch share: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if download is allowed
	if !share.CanDownload {
		http.Error(w, "download not allowed for this share", http.StatusForbidden)
		return
	}

	// Check if share is active and not expired
	if !share.IsActive {
		http.Error(w, "share link is inactive", http.StatusForbidden)
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		http.Error(w, "share link has expired", http.StatusForbidden)
		return
	}

	// Log download
	auditLog := models.DocumentAuditLog{
		DocumentID: share.DocumentID,
		Action:     models.DocumentAuditActionDownload,
		Details:    models.DocumentMetadata{"via_share": true, "share_token": shareToken},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	// Serve file
	w.Header().Set("Content-Disposition", "attachment; filename="+share.Document.FileName)
	w.Header().Set("Content-Type", share.Document.FileType)
	http.ServeFile(w, r, share.Document.FilePath)
}

// RevokeDocumentShareHandler revokes a share link
func RevokeDocumentShareHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shareID := vars["share_id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var share models.DocumentShare
	if err := config.DB.First(&share, "id = ?", shareID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "share not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch share: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Deactivate share
	share.IsActive = false
	if err := config.DB.Save(&share).Error; err != nil {
		http.Error(w, "failed to revoke share: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: share.DocumentID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionUnshare,
		Details:    models.DocumentMetadata{"share_id": shareID},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Share link revoked successfully",
	})
}

// GrantDocumentPermissionHandler grants permission to a user or role
func GrantDocumentPermissionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var req struct {
		UserID         string `json:"user_id"`
		RoleID         string `json:"role_id"`
		BusinessRoleID string `json:"business_role_id"`
		AccessLevel    string `json:"access_level"`
		CanDownload    bool   `json:"can_download"`
		CanShare       bool   `json:"can_share"`
		CanDelete      bool   `json:"can_delete"`
		ExpiresAt      string `json:"expires_at"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Verify document exists
	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	permission := models.DocumentPermission{
		DocumentID:  document.ID,
		AccessLevel: models.DocumentAccessLevel(req.AccessLevel),
		CanDownload: req.CanDownload,
		CanShare:    req.CanShare,
		CanDelete:   req.CanDelete,
		GrantedByID: userID,
	}

	if req.UserID != "" {
		uid, err := uuid.Parse(req.UserID)
		if err == nil {
			permission.UserID = &uid
		}
	}

	if req.RoleID != "" {
		rid, err := uuid.Parse(req.RoleID)
		if err == nil {
			permission.RoleID = &rid
		}
	}

	if req.BusinessRoleID != "" {
		brid, err := uuid.Parse(req.BusinessRoleID)
		if err == nil {
			permission.BusinessRoleID = &brid
		}
	}

	if req.ExpiresAt != "" {
		parsedTime, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err == nil {
			permission.ExpiresAt = &parsedTime
		}
	}

	if err := config.DB.Create(&permission).Error; err != nil {
		http.Error(w, "failed to grant permission: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionPermissionChange,
		Details:    models.DocumentMetadata{"permission_id": permission.ID, "access_level": req.AccessLevel},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Permission granted successfully",
		"permission": permission,
	})
}

// GetDocumentPermissionsHandler returns all permissions for a document
func GetDocumentPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	var permissions []models.DocumentPermission
	if err := config.DB.Preload("User").Preload("Role").Preload("BusinessRole").
		Where("document_id = ?", documentID).Find(&permissions).Error; err != nil {
		http.Error(w, "failed to fetch permissions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)
}

// RevokeDocumentPermissionHandler revokes a permission
func RevokeDocumentPermissionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	permissionID := vars["permission_id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var permission models.DocumentPermission
	if err := config.DB.First(&permission, "id = ?", permissionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "permission not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch permission: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := config.DB.Delete(&permission).Error; err != nil {
		http.Error(w, "failed to revoke permission: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: permission.DocumentID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionPermissionChange,
		Details:    models.DocumentMetadata{"permission_revoked": permissionID},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Permission revoked successfully",
	})
}

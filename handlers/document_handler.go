package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// DocumentUploadRequest represents the upload request
type DocumentUploadRequest struct {
	Title              string                 `json:"title"`
	Description        string                 `json:"description"`
	CategoryID         string                 `json:"category_id"`
	Tags               []string               `json:"tags"`
	Metadata           map[string]interface{} `json:"metadata"`
	BusinessVerticalID string                 `json:"business_vertical_id"`
	WorkflowID         string                 `json:"workflow_id"`
	IsPublic           bool                   `json:"is_public"`
}

// UploadDocumentHandler handles document uploads with metadata
func UploadDocumentHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Parse metadata
	var req DocumentUploadRequest
	if metadataStr := r.FormValue("metadata"); metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &req); err != nil {
			http.Error(w, "invalid metadata: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Set defaults
	if req.Title == "" {
		req.Title = header.Filename
	}

	// Calculate file hash for deduplication
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		http.Error(w, "failed to calculate hash: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	// Reset file pointer
	file.Seek(0, 0)

	// Check for duplicate file
	var existingDoc models.Document
	if err := config.DB.Where("file_hash = ? AND deleted_at IS NULL", fileHash).First(&existingDoc).Error; err == nil {
		// File already exists, return existing document
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "File already exists",
			"document": existingDoc,
		})
		return
	}

	// Determine storage path
	uploadDir := "./uploads/documents"
	useGCS := os.Getenv("USE_GCS") == "true"

	if !useGCS {
		// Ensure upload directory exists
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			http.Error(w, "failed to create upload directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Create unique filename
	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s-%s%s", timestamp, uuid.New().String()[:8], ext)
	filePath := filepath.Join(uploadDir, fileName)

	// Save file
	if useGCS {
		// TODO: Implement GCS upload
		http.Error(w, "GCS upload not implemented yet", http.StatusNotImplemented)
		return
	} else {
		// Save to local filesystem
		dst, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "failed to create file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Get file info
	fileInfo, _ := os.Stat(filePath)
	fileSize := fileInfo.Size()

	// Parse UUIDs
	var categoryID *uuid.UUID
	if req.CategoryID != "" {
		cid, err := uuid.Parse(req.CategoryID)
		if err == nil {
			categoryID = &cid
		}
	}

	var businessVerticalID *uuid.UUID
	fmt.Println("BusinessVerticalID:", req.BusinessVerticalID)
	req.BusinessVerticalID = "6e5deba2-e31a-4ba7-8681-3b2c8793b6db" // Temporary hardcode for testing
	if req.BusinessVerticalID != "" {
		bvid, err := uuid.Parse(req.BusinessVerticalID)
		if err == nil {
			businessVerticalID = &bvid
		}
	}

	var workflowID *uuid.UUID
	if req.WorkflowID != "" {
		wid, err := uuid.Parse(req.WorkflowID)
		if err == nil {
			workflowID = &wid
		}
	}

	// Parse user ID from claims
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Create document record
	document := models.Document{
		Title:              req.Title,
		Description:        req.Description,
		FileName:           header.Filename,
		FileSize:           fileSize,
		FileType:           header.Header.Get("Content-Type"),
		FileExtension:      ext,
		FilePath:           filePath,
		FileHash:           fileHash,
		Status:             models.DocumentStatusDraft,
		Version:            1,
		CategoryID:         categoryID,
		Metadata:           req.Metadata,
		BusinessVerticalID: businessVerticalID,
		UploadedByID:       userID,
		WorkflowID:         workflowID,
		IsPublic:           req.IsPublic,
	}

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create document
	if err := tx.Create(&document).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create initial version
	version := models.DocumentVersion{
		DocumentID:       document.ID,
		VersionNumber:    1,
		FileName:         header.Filename,
		FileSize:         fileSize,
		FileType:         header.Header.Get("Content-Type"),
		FilePath:         filePath,
		FileHash:         fileHash,
		ChangeLog:        "Initial upload",
		CreatedByID:      userID,
		IsCurrentVersion: true,
	}

	if err := tx.Create(&version).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create version: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Add tags
	if len(req.Tags) > 0 {
		var tags []models.DocumentTag
		for _, tagName := range req.Tags {
			var tag models.DocumentTag
			if err := config.DB.Where("name = ?", tagName).First(&tag).Error; err == gorm.ErrRecordNotFound {
				// Create new tag
				tag = models.DocumentTag{
					Name:               tagName,
					BusinessVerticalID: businessVerticalID,
				}
				if err := tx.Create(&tag).Error; err != nil {
					continue
				}
			}
			tags = append(tags, tag)
		}
		if err := tx.Model(&document).Association("Tags").Append(tags); err != nil {
			tx.Rollback()
			http.Error(w, "failed to add tags: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionCreate,
		Details:    models.DocumentMetadata{"file_name": header.Filename, "file_size": fileSize},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	tx.Create(&auditLog)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load relationships
	config.DB.Preload("Category").Preload("Tags").Preload("UploadedBy").First(&document, document.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Document uploaded successfully",
		"document": document,
	})
}

// GetDocumentsHandler returns a list of documents with filtering and pagination
func GetDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Filters
	categoryID := r.URL.Query().Get("category_id")
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	businessVerticalID := r.URL.Query().Get("business_vertical_id")
	tag := r.URL.Query().Get("tag")

	// Build query
	query := config.DB.Model(&models.Document{}).Preload("Category").Preload("Tags").Preload("UploadedBy")

	if categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if businessVerticalID != "" {
		query = query.Where("business_vertical_id = ?", businessVerticalID)
	}

	if search != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ? OR file_name ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if tag != "" {
		query = query.Joins("JOIN document_tags ON document_tags.document_id = documents.id").
			Joins("JOIN tags ON tags.id = document_tags.tag_id").
			Where("tags.name = ?", tag)
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get documents
	var documents []models.Document
	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&documents).Error; err != nil {
		http.Error(w, "failed to fetch documents: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"documents": documents,
		"total":     total,
		"page":      page,
		"limit":     limit,
		"pages":     (total + int64(limit) - 1) / int64(limit),
	})
}

// GetDocumentHandler returns a single document by ID
func GetDocumentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var document models.Document
	if err := config.DB.Preload("Category").Preload("Tags").Preload("UploadedBy").
		Preload("Versions").Preload("Permissions").
		First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Increment view count
	config.DB.Model(&document).Update("view_count", gorm.Expr("view_count + 1"))

	// Parse user ID from claims
	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userUUID,
		Action:     models.DocumentAuditActionView,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(document)
}

// UpdateDocumentHandler updates document metadata
func UpdateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Parse request body
	var req struct {
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		CategoryID  string                 `json:"category_id"`
		Tags        []string               `json:"tags"`
		Metadata    map[string]interface{} `json:"metadata"`
		Status      string                 `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update fields
	if req.Title != "" {
		document.Title = req.Title
	}
	if req.Description != "" {
		document.Description = req.Description
	}
	if req.CategoryID != "" {
		categoryID, err := uuid.Parse(req.CategoryID)
		if err == nil {
			document.CategoryID = &categoryID
		}
	}
	if req.Metadata != nil {
		document.Metadata = req.Metadata
	}
	if req.Status != "" {
		document.Status = models.DocumentStatus(req.Status)
	}

	// Save changes
	if err := config.DB.Save(&document).Error; err != nil {
		http.Error(w, "failed to update document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update tags
	if len(req.Tags) > 0 {
		var tags []models.DocumentTag
		for _, tagName := range req.Tags {
			var tag models.DocumentTag
			if err := config.DB.Where("name = ?", tagName).First(&tag).Error; err == gorm.ErrRecordNotFound {
				tag = models.DocumentTag{Name: tagName}
				config.DB.Create(&tag)
			}
			tags = append(tags, tag)
		}
		config.DB.Model(&document).Association("Tags").Replace(tags)
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionEdit,
		Details:    models.DocumentMetadata{"changes": req},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	// Reload with relationships
	config.DB.Preload("Category").Preload("Tags").Preload("UploadedBy").First(&document, document.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Document updated successfully",
		"document": document,
	})
}

// DeleteDocumentHandler soft deletes a document
func DeleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Soft delete
	if err := config.DB.Delete(&document).Error; err != nil {
		http.Error(w, "failed to delete document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionDelete,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Document deleted successfully",
	})
}

// DownloadDocumentHandler handles document downloads
func DownloadDocumentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if file exists
	if _, err := os.Stat(document.FilePath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Increment download count
	config.DB.Model(&document).Update("download_count", gorm.Expr("download_count + 1"))

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionDownload,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", document.FileName))
	w.Header().Set("Content-Type", document.FileType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", document.FileSize))

	// Serve file
	http.ServeFile(w, r, document.FilePath)
}

// GetDocumentAuditLogsHandler returns audit logs for a document
func GetDocumentAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	var logs []models.DocumentAuditLog
	if err := config.DB.Preload("User").Where("document_id = ?", documentID).
		Order("created_at DESC").Find(&logs).Error; err != nil {
		http.Error(w, "failed to fetch audit logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// SearchDocumentsHandler performs full-text search on documents
func SearchDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "search query is required", http.StatusBadRequest)
		return
	}

	// Perform search
	var documents []models.Document
	searchPattern := "%" + strings.ToLower(query) + "%"

	if err := config.DB.Preload("Category").Preload("Tags").Preload("UploadedBy").
		Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(file_name) LIKE ? OR LOWER(metadata::text) LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern).
		Order("created_at DESC").
		Limit(50).
		Find(&documents).Error; err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   query,
		"results": documents,
		"count":   len(documents),
	})
}

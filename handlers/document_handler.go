package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

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
	ProjectID          string                 `json:"project_id"`
	TaskID             string                 `json:"task_id"`
	WorkflowID         string                 `json:"workflow_id"`
	IsPublic           bool                   `json:"is_public"`
}

type DocumentContextBackfillRequest struct {
	DryRun bool `json:"dry_run"`
	Limit  int  `json:"limit"`
}

type DocumentContextBackfillResult struct {
	DryRun              bool `json:"dry_run"`
	Scanned             int  `json:"scanned"`
	Matched             int  `json:"matched"`
	UpdatedDocuments    int  `json:"updated_documents"`
	UpdatedProjectLinks int  `json:"updated_project_links"`
	UpdatedTaskLinks    int  `json:"updated_task_links"`
	SkippedInvalid      int  `json:"skipped_invalid"`
}

func hasDocumentContextColumns() (bool, bool) {
	projectColumnExists := config.DB.Migrator().HasColumn(&models.Document{}, "project_id")
	taskColumnExists := config.DB.Migrator().HasColumn(&models.Document{}, "task_id")
	return projectColumnExists, taskColumnExists
}

func getUUIDFromDocumentMetadata(metadata models.DocumentMetadata, key string) *uuid.UUID {
	if metadata == nil {
		return nil
	}

	raw, ok := metadata[key]
	if !ok || raw == nil {
		return nil
	}

	value, ok := raw.(string)
	if !ok {
		return nil
	}

	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil
	}

	return &parsed
}

// BackfillDocumentContextLinksHandler migrates legacy metadata context links into first-class project_id/task_id fields.
func BackfillDocumentContextLinksHandler(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req DocumentContextBackfillRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if req.Limit < 0 {
		req.Limit = 0
	}

	query := config.DB.Model(&models.Document{}).
		Where("deleted_at IS NULL").
		Where("(project_id IS NULL AND metadata ? 'project_id') OR (task_id IS NULL AND metadata ? 'task_id')")

	if req.Limit > 0 {
		query = query.Limit(req.Limit)
	}

	var documents []models.Document
	if err := query.Find(&documents).Error; err != nil {
		http.Error(w, "failed to fetch documents for backfill: "+err.Error(), http.StatusInternalServerError)
		return
	}

	result := DocumentContextBackfillResult{
		DryRun:  req.DryRun,
		Scanned: len(documents),
	}

	if req.DryRun {
		for _, document := range documents {
			projectID := getUUIDFromDocumentMetadata(document.Metadata, "project_id")
			taskID := getUUIDFromDocumentMetadata(document.Metadata, "task_id")

			canUpdateProject := document.ProjectID == nil && projectID != nil
			canUpdateTask := document.TaskID == nil && taskID != nil

			if canUpdateProject || canUpdateTask {
				result.Matched++
				if canUpdateProject {
					result.UpdatedProjectLinks++
				}
				if canUpdateTask {
					result.UpdatedTaskLinks++
				}
			} else {
				result.SkippedInvalid++
			}
		}

		result.UpdatedDocuments = result.Matched

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Dry run completed",
			"result":  result,
		})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if recovered := recover(); recovered != nil {
			tx.Rollback()
		}
	}()

	for _, document := range documents {
		projectID := getUUIDFromDocumentMetadata(document.Metadata, "project_id")
		taskID := getUUIDFromDocumentMetadata(document.Metadata, "task_id")

		updates := map[string]interface{}{}

		if document.ProjectID == nil && projectID != nil {
			updates["project_id"] = *projectID
		}

		if document.TaskID == nil && taskID != nil {
			updates["task_id"] = *taskID
		}

		if len(updates) == 0 {
			result.SkippedInvalid++
			continue
		}

		if err := tx.Model(&models.Document{}).Where("id = ?", document.ID).Updates(updates).Error; err != nil {
			tx.Rollback()
			http.Error(w, "failed to backfill document context links: "+err.Error(), http.StatusInternalServerError)
			return
		}

		result.Matched++
		result.UpdatedDocuments++
		if _, ok := updates["project_id"]; ok {
			result.UpdatedProjectLinks++
		}
		if _, ok := updates["task_id"]; ok {
			result.UpdatedTaskLinks++
		}
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit context backfill transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Context backfill completed",
		"result":  result,
	})
}

// UploadDocumentHandler handles document uploads with metadata
func UploadDocumentHandler(w http.ResponseWriter, r *http.Request) {
	// Get claims and validate
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get full user context with roles and business verticals
	user := middleware.GetUser(r)
	if user.ID == uuid.Nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
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

	// Backward compatibility: allow project/task context inside metadata map.
	if req.ProjectID == "" && req.Metadata != nil {
		if projectIDValue, ok := req.Metadata["project_id"].(string); ok {
			req.ProjectID = strings.TrimSpace(projectIDValue)
		}
	}

	if req.TaskID == "" && req.Metadata != nil {
		if taskIDValue, ok := req.Metadata["task_id"].(string); ok {
			req.TaskID = strings.TrimSpace(taskIDValue)
		}
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

	// Check for duplicate file only for global uploads.
	// Context-scoped uploads (project/task) should create their own records for traceability.
	hasScopedContext := strings.TrimSpace(req.ProjectID) != "" || strings.TrimSpace(req.TaskID) != ""
	if !hasScopedContext {
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
	}

	upload, err := storeUploadedFile(r, "file", "./uploads/documents")
	if err != nil {
		http.Error(w, "failed to store file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ext := filepath.Ext(upload.OriginalFilename)
	filePath := upload.Path
	fileSize := upload.Size

	// Parse UUIDs
	var categoryID *uuid.UUID
	if req.CategoryID != "" {
		cid, err := uuid.Parse(req.CategoryID)
		if err == nil {
			categoryID = &cid
		}
	}

	// Get business vertical from user's accessible verticals or request
	var businessVerticalID *uuid.UUID
	if req.BusinessVerticalID != "" {
		bvid, err := uuid.Parse(req.BusinessVerticalID)
		if err == nil {
			businessVerticalID = &bvid
		}
	} else if len(user.UserBusinessRoles) > 0 && user.UserBusinessRoles[0].BusinessRole.BusinessVerticalID != uuid.Nil {
		// Use user's primary business vertical from their business role
		bvID := user.UserBusinessRoles[0].BusinessRole.BusinessVerticalID
		businessVerticalID = &bvID
	}

	var workflowID *uuid.UUID
	if req.WorkflowID != "" {
		wid, err := uuid.Parse(req.WorkflowID)
		if err == nil {
			workflowID = &wid
		}
	}

	var projectID *uuid.UUID
	if req.ProjectID != "" {
		pid, err := uuid.Parse(req.ProjectID)
		if err == nil {
			projectID = &pid
		}
	}

	var taskID *uuid.UUID
	if req.TaskID != "" {
		tid, err := uuid.Parse(req.TaskID)
		if err == nil {
			taskID = &tid
		}
	}

	var workflowDef *models.WorkflowDefinition
	if workflowID != nil {
		var selectedWorkflow models.WorkflowDefinition
		if err := config.DB.Where("id = ? AND is_active = ?", *workflowID, true).First(&selectedWorkflow).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				http.Error(w, "invalid or inactive workflow selected", http.StatusBadRequest)
			} else {
				http.Error(w, "failed to load workflow: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		workflowDef = &selectedWorkflow
	}

	initialState := resolveInitialDocumentState(workflowDef)
	initialStatus := mapDocumentStateToStatus(initialState)

	// Use user ID from loaded user context
	userID := user.ID

	// Create document record
	document := models.Document{
		Title:              req.Title,
		Description:        req.Description,
		FileName:           upload.OriginalFilename,
		FileSize:           fileSize,
		FileType:           upload.MimeType,
		FileExtension:      ext,
		FilePath:           filePath,
		FileHash:           fileHash,
		Status:             initialStatus,
		Version:            1,
		CategoryID:         categoryID,
		Metadata:           req.Metadata,
		BusinessVerticalID: businessVerticalID,
		ProjectID:          projectID,
		TaskID:             taskID,
		UploadedByID:       userID,
		WorkflowID:         workflowID,
		CurrentState:       initialState,
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
		FileName:         upload.OriginalFilename,
		FileSize:         fileSize,
		FileType:         upload.MimeType,
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
		Details:    models.DocumentMetadata{"file_name": upload.OriginalFilename, "file_size": fileSize},
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
	projectID := r.URL.Query().Get("project_id")
	taskID := r.URL.Query().Get("task_id")
	tag := r.URL.Query().Get("tag")

	if projectID != "" {
		if _, err := uuid.Parse(projectID); err != nil {
			http.Error(w, "invalid project_id", http.StatusBadRequest)
			return
		}
	}

	if taskID != "" {
		if _, err := uuid.Parse(taskID); err != nil {
			http.Error(w, "invalid task_id", http.StatusBadRequest)
			return
		}
	}

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

	projectColumnExists, taskColumnExists := hasDocumentContextColumns()

	if projectID != "" {
		if projectColumnExists {
			query = query.Where("project_id = ? OR (metadata ->> 'project_id' = ?)", projectID, projectID)
		} else {
			query = query.Where("metadata ->> 'project_id' = ?", projectID)
		}
	}

	if taskID != "" {
		if taskColumnExists {
			query = query.Where("task_id = ? OR (metadata ->> 'task_id' = ?)", taskID, taskID)
		} else {
			query = query.Where("metadata ->> 'task_id' = ?", taskID)
		}
	}

	if search != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ? OR file_name ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if tag != "" {
		query = query.Joins("JOIN document_tag_links ON document_tag_links.document_id = documents.id").
			Joins("JOIN document_tags ON document_tags.id = document_tag_links.document_tag_id").
			Where("document_tags.name = ?", tag)
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
	// Get claims and validate
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get full user context
	user := middleware.GetUser(r)
	if user.ID == uuid.Nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	documentID := vars["id"]

	// Validate UUID format to avoid SQL errors from path segments like "categories" or "tags".
	if _, err := uuid.Parse(documentID); err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
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

	// Log audit with user ID
	userID := user.ID
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
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
	// Get claims and validate
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get full user context
	user := middleware.GetUser(r)
	if user.ID == uuid.Nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := user.ID

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
		if strings.TrimSpace(document.CurrentState) != "" {
			http.Error(w, "status is managed by workflow for this document; use workflow transition endpoint", http.StatusBadRequest)
			return
		}
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
	// Get claims and validate
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get full user context
	user := middleware.GetUser(r)
	if user.ID == uuid.Nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := user.ID

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
	// Get claims and validate
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get full user context
	user := middleware.GetUser(r)
	if user.ID == uuid.Nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := user.ID

	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
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

	if err := serveStoredFile(w, r, document.FilePath, document.FileName, document.FileType, document.FileSize); err != nil {
		if errors.Is(err, errStoredFileNotFound) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to serve file: "+err.Error(), http.StatusInternalServerError)
	}
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

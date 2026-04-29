package handlers

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// docTagLink is the junction record for document_tag_links (many2many).
type docTagLink struct {
	DocumentID    uuid.UUID `gorm:"column:document_id"`
	DocumentTagID uuid.UUID `gorm:"column:document_tag_id"`
}

func (docTagLink) TableName() string { return "document_tag_links" }

// BulkDeleteDocumentsHandler deletes multiple documents
func BulkDeleteDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getDocumentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req struct {
		DocumentIDs []string `json:"document_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.DocumentIDs) == 0 {
		http.Error(w, "no document IDs provided", http.StatusBadRequest)
		return
	}

	// Batch fetch IDs that actually exist (for accurate count + audit)
	var documents []models.Document
	if err := config.DB.Select("id").Where("id IN ?", req.DocumentIDs).Find(&documents).Error; err != nil {
		http.Error(w, "failed to fetch documents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(documents) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Documents deleted successfully", "deleted": 0, "total": len(req.DocumentIDs)})
		return
	}

	validIDs := make([]uuid.UUID, len(documents))
	for i, d := range documents {
		validIDs[i] = d.ID
	}

	tx := config.DB.Begin()
	defer func() {
		if rec := recover(); rec != nil {
			tx.Rollback()
		}
	}()

	// Single batch soft-delete
	if err := tx.Where("id IN ?", validIDs).Delete(&models.Document{}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to delete documents: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Batch audit logs
	auditLogs := make([]models.DocumentAuditLog, len(documents))
	for i, doc := range documents {
		auditLogs[i] = models.DocumentAuditLog{
			DocumentID: doc.ID,
			UserID:     &userID,
			Action:     models.DocumentAuditActionDelete,
			Details:    models.DocumentMetadata{"bulk_delete": true},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
	}
	if err := tx.CreateInBatches(auditLogs, 100).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to log audit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to delete documents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	deletedCount := len(documents)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Documents deleted successfully",
		"deleted": deletedCount,
		"total":   len(req.DocumentIDs),
	})
}

// BulkUpdateDocumentsHandler updates multiple documents
func BulkUpdateDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getDocumentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req struct {
		DocumentIDs []string               `json:"document_ids"`
		Updates     map[string]interface{} `json:"updates"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.DocumentIDs) == 0 {
		http.Error(w, "no document IDs provided", http.StatusBadRequest)
		return
	}

	// Batch fetch valid document IDs
	var documents []models.Document
	if err := config.DB.Select("id").Where("id IN ?", req.DocumentIDs).Find(&documents).Error; err != nil {
		http.Error(w, "failed to fetch documents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(documents) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Documents updated successfully", "updated": 0, "total": len(req.DocumentIDs)})
		return
	}

	validIDs := make([]uuid.UUID, len(documents))
	for i, d := range documents {
		validIDs[i] = d.ID
	}

	// Build safe, whitelist-only updates map
	safeUpdates := map[string]interface{}{}
	if status, ok := req.Updates["status"].(string); ok {
		safeUpdates["status"] = models.DocumentStatus(status)
	}
	if categoryIDStr, ok := req.Updates["category_id"].(string); ok {
		if cid, err := uuid.Parse(categoryIDStr); err == nil {
			safeUpdates["category_id"] = cid
		}
	}

	tx := config.DB.Begin()
	defer func() {
		if rec := recover(); rec != nil {
			tx.Rollback()
		}
	}()

	if len(safeUpdates) > 0 {
		if err := tx.Model(&models.Document{}).Where("id IN ?", validIDs).Updates(safeUpdates).Error; err != nil {
			tx.Rollback()
			http.Error(w, "failed to update documents: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Batch audit logs
	auditLogs := make([]models.DocumentAuditLog, len(documents))
	for i, doc := range documents {
		auditLogs[i] = models.DocumentAuditLog{
			DocumentID: doc.ID,
			UserID:     &userID,
			Action:     models.DocumentAuditActionEdit,
			Details:    models.DocumentMetadata{"bulk_update": true, "updates": req.Updates},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
	}
	if err := tx.CreateInBatches(auditLogs, 100).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to log audit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to update documents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	updatedCount := len(documents)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Documents updated successfully",
		"updated": updatedCount,
		"total":   len(req.DocumentIDs),
	})
}

// BulkDownloadDocumentsHandler creates a zip file of multiple documents
func BulkDownloadDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getDocumentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req struct {
		DocumentIDs []string `json:"document_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.DocumentIDs) == 0 {
		http.Error(w, "no document IDs provided", http.StatusBadRequest)
		return
	}

	// Fetch documents
	var documents []models.Document
	if err := config.DB.Where("id IN ?", req.DocumentIDs).Find(&documents).Error; err != nil {
		http.Error(w, "failed to fetch documents: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(documents) == 0 {
		http.Error(w, "no documents found", http.StatusNotFound)
		return
	}

	// Create temporary zip file
	zipFileName := fmt.Sprintf("documents-%s.zip", uuid.New().String()[:8])
	zipFilePath := filepath.Join("./uploads/temp", zipFileName)

	// Ensure temp directory exists
	os.MkdirAll("./uploads/temp", 0755)

	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		http.Error(w, "failed to create zip file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()
	defer os.Remove(zipFilePath) // Clean up after sending

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add documents to zip
	for _, doc := range documents {
		srcFile, _, err := openStoredFileReader(r.Context(), doc.FilePath)
		if errors.Is(err, errStoredFileNotFound) {
			continue
		}
		if err != nil {
			continue
		}

		// Create file in zip
		zipFileWriter, err := zipWriter.Create(doc.FileName)
		if err != nil {
			_ = srcFile.Close()
			continue
		}

		// Copy file content
		if _, err := io.Copy(zipFileWriter, srcFile); err != nil {
			_ = srcFile.Close()
			continue
		}
		_ = srcFile.Close()

		// Log download
		auditLog := models.DocumentAuditLog{
			DocumentID: doc.ID,
			UserID:     &userID,
			Action:     models.DocumentAuditActionDownload,
			Details:    models.DocumentMetadata{"bulk_download": true},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
		config.DB.Create(&auditLog)
	}

	zipWriter.Close()
	zipFile.Close()

	// Serve zip file
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipFileName))
	http.ServeFile(w, r, zipFilePath)

}

// GetDocumentStatisticsHandler returns statistics about documents
func GetDocumentStatisticsHandler(w http.ResponseWriter, r *http.Request) {
	businessVerticalID := r.URL.Query().Get("business_vertical_id")

	var stats struct {
		TotalDocuments    int64                    `json:"total_documents"`
		TotalSize         int64                    `json:"total_size"`
		DocumentsByStatus map[string]int64         `json:"documents_by_status"`
		DocumentsByType   map[string]int64         `json:"documents_by_type"`
		RecentUploads     []models.Document        `json:"recent_uploads"`
		TopCategories     []map[string]interface{} `json:"top_categories"`
		TotalDownloads    int64                    `json:"total_downloads"`
		TotalViews        int64                    `json:"total_views"`
	}

	scopedDocuments := func() *gorm.DB {
		query := config.DB.Model(&models.Document{})
		if businessVerticalID != "" {
			query = query.Where("business_vertical_id = ?", businessVerticalID)
		}
		return query
	}

	// Total documents
	scopedDocuments().Count(&stats.TotalDocuments)

	// Total size
	var result struct {
		TotalSize int64
	}
	scopedDocuments().Select("COALESCE(SUM(file_size), 0) as total_size").Scan(&result)
	stats.TotalSize = result.TotalSize

	// Documents by status
	stats.DocumentsByStatus = make(map[string]int64)
	var statusCounts []struct {
		Status string
		Count  int64
	}
	scopedDocuments().Select("status, COUNT(*) as count").Group("status").Scan(&statusCounts)
	for _, sc := range statusCounts {
		stats.DocumentsByStatus[sc.Status] = sc.Count
	}

	// Documents by file type
	stats.DocumentsByType = make(map[string]int64)
	var typeCounts []struct {
		FileExtension string
		Count         int64
	}
	scopedDocuments().Select("file_extension, COUNT(*) as count").Group("file_extension").Limit(10).Scan(&typeCounts)
	for _, tc := range typeCounts {
		stats.DocumentsByType[tc.FileExtension] = tc.Count
	}

	// Recent uploads
	recentUploadsQuery := config.DB.Preload("UploadedBy").Preload("Category").Model(&models.Document{})
	if businessVerticalID != "" {
		recentUploadsQuery = recentUploadsQuery.Where("business_vertical_id = ?", businessVerticalID)
	}
	recentUploadsQuery.Order("created_at DESC").Limit(10).Find(&stats.RecentUploads)

	// Total downloads and views — single aggregate query
	var downloadViewTotals struct {
		TotalDownloads int64
		TotalViews     int64
	}
	scopedDocuments().Select("COALESCE(SUM(download_count), 0) as total_downloads, COALESCE(SUM(view_count), 0) as total_views").Scan(&downloadViewTotals)
	stats.TotalDownloads = downloadViewTotals.TotalDownloads
	stats.TotalViews = downloadViewTotals.TotalViews

	// Top categories
	var categoryStats []struct {
		CategoryID   uuid.UUID
		CategoryName string
		DocCount     int64
	}
	categoryQuery := config.DB.Table("documents").
		Select("category_id, document_categories.name as category_name, COUNT(*) as doc_count").
		Joins("LEFT JOIN document_categories ON documents.category_id = document_categories.id").
		Where("documents.deleted_at IS NULL AND category_id IS NOT NULL")
	if businessVerticalID != "" {
		categoryQuery = categoryQuery.Where("documents.business_vertical_id = ?", businessVerticalID)
	}
	categoryQuery.Group("category_id, document_categories.name").
		Order("doc_count DESC").
		Limit(5).
		Scan(&categoryStats)

	stats.TopCategories = make([]map[string]interface{}, len(categoryStats))
	for i, cs := range categoryStats {
		stats.TopCategories[i] = map[string]interface{}{
			"category_id":   cs.CategoryID,
			"category_name": cs.CategoryName,
			"count":         cs.DocCount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)

}

// BulkAddTagsHandler adds tags to multiple documents
func BulkAddTagsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getDocumentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var req struct {
		DocumentIDs []string `json:"document_ids"`
		TagNames    []string `json:"tag_names"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.DocumentIDs) == 0 || len(req.TagNames) == 0 {
		http.Error(w, "document IDs and tag names are required", http.StatusBadRequest)
		return
	}

	// Batch find existing tags, then create missing ones
	var existingTags []models.DocumentTag
	config.DB.Where("name IN ?", req.TagNames).Find(&existingTags)

	existingByName := make(map[string]models.DocumentTag, len(existingTags))
	for _, t := range existingTags {
		existingByName[t.Name] = t
	}

	var newTags []models.DocumentTag
	for _, name := range req.TagNames {
		if _, ok := existingByName[name]; !ok {
			newTags = append(newTags, models.DocumentTag{Name: name})
		}
	}
	if len(newTags) > 0 {
		config.DB.CreateInBatches(newTags, 100)
		for _, t := range newTags {
			existingByName[t.Name] = t
		}
	}

	tags := make([]models.DocumentTag, 0, len(req.TagNames))
	for _, name := range req.TagNames {
		if t, ok := existingByName[name]; ok {
			tags = append(tags, t)
		}
	}

	// Batch fetch documents
	var documents []models.Document
	config.DB.Select("id").Where("id IN ?", req.DocumentIDs).Find(&documents)
	if len(documents) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Tags added successfully", "updated": 0, "total": len(req.DocumentIDs)})
		return
	}

	// Batch insert junction records (ON CONFLICT DO NOTHING for idempotency)
	links := make([]docTagLink, 0, len(documents)*len(tags))
	for _, doc := range documents {
		for _, tag := range tags {
			links = append(links, docTagLink{DocumentID: doc.ID, DocumentTagID: tag.ID})
		}
	}
	if len(links) > 0 {
		config.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(links, 200)
	}

	// Batch audit logs
	auditLogs := make([]models.DocumentAuditLog, len(documents))
	for i, doc := range documents {
		auditLogs[i] = models.DocumentAuditLog{
			DocumentID: doc.ID,
			UserID:     &userID,
			Action:     models.DocumentAuditActionEdit,
			Details:    models.DocumentMetadata{"bulk_add_tags": true, "tags": req.TagNames},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
	}
	config.DB.CreateInBatches(auditLogs, 100)

	updatedCount := len(documents)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tags added successfully",
		"updated": updatedCount,
		"total":   len(req.DocumentIDs),
	})
}

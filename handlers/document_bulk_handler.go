package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// BulkDeleteDocumentsHandler deletes multiple documents
func BulkDeleteDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uuid.UUID)

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

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	deletedCount := 0
	for _, docID := range req.DocumentIDs {
		var document models.Document
		if err := tx.First(&document, "id = ?", docID).Error; err != nil {
			continue
		}

		// Soft delete
		if err := tx.Delete(&document).Error; err != nil {
			continue
		}

		// Log audit
		auditLog := models.DocumentAuditLog{
			DocumentID: document.ID,
			UserID:     &userID,
			Action:     models.DocumentAuditActionDelete,
			Details:    models.DocumentMetadata{"bulk_delete": true},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
		tx.Create(&auditLog)

		deletedCount++
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to delete documents: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Documents deleted successfully",
		"deleted": deletedCount,
		"total":   len(req.DocumentIDs),
	})
}

// BulkUpdateDocumentsHandler updates multiple documents
func BulkUpdateDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uuid.UUID)

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

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updatedCount := 0
	for _, docID := range req.DocumentIDs {
		var document models.Document
		if err := tx.First(&document, "id = ?", docID).Error; err != nil {
			continue
		}

		// Apply updates
		if status, ok := req.Updates["status"].(string); ok {
			document.Status = models.DocumentStatus(status)
		}
		if categoryID, ok := req.Updates["category_id"].(string); ok {
			cid, err := uuid.Parse(categoryID)
			if err == nil {
				document.CategoryID = &cid
			}
		}

		if err := tx.Save(&document).Error; err != nil {
			continue
		}

		// Log audit
		auditLog := models.DocumentAuditLog{
			DocumentID: document.ID,
			UserID:     &userID,
			Action:     models.DocumentAuditActionEdit,
			Details:    models.DocumentMetadata{"bulk_update": true, "updates": req.Updates},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
		tx.Create(&auditLog)

		updatedCount++
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to update documents: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Documents updated successfully",
		"updated": updatedCount,
		"total":   len(req.DocumentIDs),
	})
}

// BulkDownloadDocumentsHandler creates a zip file of multiple documents
func BulkDownloadDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uuid.UUID)

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
		// Check if file exists
		if _, err := os.Stat(doc.FilePath); os.IsNotExist(err) {
			continue
		}

		// Open source file
		srcFile, err := os.Open(doc.FilePath)
		if err != nil {
			continue
		}

		// Create file in zip
		zipFileWriter, err := zipWriter.Create(doc.FileName)
		if err != nil {
			srcFile.Close()
			continue
		}

		// Copy file content
		if _, err := io.Copy(zipFileWriter, srcFile); err != nil {
			srcFile.Close()
			continue
		}
		srcFile.Close()

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

	query := config.DB.Model(&models.Document{})
	if businessVerticalID != "" {
		query = query.Where("business_vertical_id = ?", businessVerticalID)
	}

	// Total documents
	query.Count(&stats.TotalDocuments)

	// Total size
	var result struct {
		TotalSize int64
	}
	config.DB.Model(&models.Document{}).Select("COALESCE(SUM(file_size), 0) as total_size").Scan(&result)
	stats.TotalSize = result.TotalSize

	// Documents by status
	stats.DocumentsByStatus = make(map[string]int64)
	var statusCounts []struct {
		Status string
		Count  int64
	}
	config.DB.Model(&models.Document{}).Select("status, COUNT(*) as count").Group("status").Scan(&statusCounts)
	for _, sc := range statusCounts {
		stats.DocumentsByStatus[sc.Status] = sc.Count
	}

	// Documents by file type
	stats.DocumentsByType = make(map[string]int64)
	var typeCounts []struct {
		FileExtension string
		Count         int64
	}
	config.DB.Model(&models.Document{}).Select("file_extension, COUNT(*) as count").Group("file_extension").Limit(10).Scan(&typeCounts)
	for _, tc := range typeCounts {
		stats.DocumentsByType[tc.FileExtension] = tc.Count
	}

	// Recent uploads
	config.DB.Preload("UploadedBy").Preload("Category").Order("created_at DESC").Limit(10).Find(&stats.RecentUploads)

	// Total downloads and views
	config.DB.Model(&models.Document{}).Select("COALESCE(SUM(download_count), 0)").Scan(&stats.TotalDownloads)
	config.DB.Model(&models.Document{}).Select("COALESCE(SUM(view_count), 0)").Scan(&stats.TotalViews)

	// Top categories
	var categoryStats []struct {
		CategoryID   uuid.UUID
		CategoryName string
		DocCount     int64
	}
	config.DB.Table("documents").
		Select("category_id, document_categories.name as category_name, COUNT(*) as doc_count").
		Joins("LEFT JOIN document_categories ON documents.category_id = document_categories.id").
		Where("documents.deleted_at IS NULL AND category_id IS NOT NULL").
		Group("category_id, document_categories.name").
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
	userID := r.Context().Value("userID").(uuid.UUID)

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

	// Get or create tags
	var tags []models.DocumentTag
	for _, tagName := range req.TagNames {
		var tag models.DocumentTag
		if err := config.DB.Where("name = ?", tagName).First(&tag).Error; err == gorm.ErrRecordNotFound {
			tag = models.DocumentTag{Name: tagName}
			config.DB.Create(&tag)
		}
		tags = append(tags, tag)
	}

	// Add tags to documents
	updatedCount := 0
	for _, docID := range req.DocumentIDs {
		var document models.Document
		if err := config.DB.First(&document, "id = ?", docID).Error; err != nil {
			continue
		}

		if err := config.DB.Model(&document).Association("Tags").Append(tags); err != nil {
			continue
		}

		// Log audit
		auditLog := models.DocumentAuditLog{
			DocumentID: document.ID,
			UserID:     &userID,
			Action:     models.DocumentAuditActionEdit,
			Details:    models.DocumentMetadata{"bulk_add_tags": true, "tags": req.TagNames},
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		}
		config.DB.Create(&auditLog)

		updatedCount++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tags added successfully",
		"updated": updatedCount,
		"total":   len(req.DocumentIDs),
	})
}

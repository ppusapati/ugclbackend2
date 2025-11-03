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
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// GetDocumentVersionsHandler returns all versions of a document
func GetDocumentVersionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]

	var versions []models.DocumentVersion
	if err := config.DB.Preload("CreatedBy").Where("document_id = ?", documentID).
		Order("version_number DESC").Find(&versions).Error; err != nil {
		http.Error(w, "failed to fetch versions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

// CreateDocumentVersionHandler creates a new version of a document
func CreateDocumentVersionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	// Get existing document
	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Parse multipart form
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

	// Get changelog
	changeLog := r.FormValue("change_log")
	if changeLog == "" {
		changeLog = "Version update"
	}

	// Calculate file hash
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		http.Error(w, "failed to calculate hash: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fileHash := hex.EncodeToString(hasher.Sum(nil))
	file.Seek(0, 0)

	// Save new file
	uploadDir := "./uploads/documents"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "failed to create upload directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s-%s%s", timestamp, uuid.New().String()[:8], ext)
	filePath := filepath.Join(uploadDir, fileName)

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

	fileInfo, _ := os.Stat(filePath)
	fileSize := fileInfo.Size()

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Mark all existing versions as not current
	if err := tx.Model(&models.DocumentVersion{}).
		Where("document_id = ?", documentID).
		Update("is_current_version", false).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update versions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get next version number
	var lastVersion models.DocumentVersion
	tx.Where("document_id = ?", documentID).Order("version_number DESC").First(&lastVersion)
	nextVersion := lastVersion.VersionNumber + 1

	// Create new version
	version := models.DocumentVersion{
		DocumentID:       document.ID,
		VersionNumber:    nextVersion,
		FileName:         header.Filename,
		FileSize:         fileSize,
		FileType:         header.Header.Get("Content-Type"),
		FilePath:         filePath,
		FileHash:         fileHash,
		ChangeLog:        changeLog,
		CreatedByID:      userID,
		IsCurrentVersion: true,
	}

	if err := tx.Create(&version).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create version: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update document
	document.Version = nextVersion
	document.FilePath = filePath
	document.FileSize = fileSize
	document.FileType = header.Header.Get("Content-Type")
	document.FileHash = fileHash
	document.FileName = header.Filename
	document.FileExtension = ext

	if err := tx.Save(&document).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionVersionCreate,
		Details:    models.DocumentMetadata{"version": nextVersion, "change_log": changeLog},
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
	config.DB.Preload("CreatedBy").First(&version, version.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Version created successfully",
		"version": version,
	})
}

// RollbackDocumentVersionHandler rolls back a document to a specific version
func RollbackDocumentVersionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	versionID := vars["version_id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	// Get document
	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Get target version
	var targetVersion models.DocumentVersion
	if err := config.DB.First(&targetVersion, "id = ? AND document_id = ?", versionID, documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "version not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch version: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Mark all versions as not current
	tx.Model(&models.DocumentVersion{}).
		Where("document_id = ?", documentID).
		Update("is_current_version", false)

	// Mark target version as current
	targetVersion.IsCurrentVersion = true
	tx.Save(&targetVersion)

	// Update document to point to target version
	document.Version = targetVersion.VersionNumber
	document.FilePath = targetVersion.FilePath
	document.FileSize = targetVersion.FileSize
	document.FileType = targetVersion.FileType
	document.FileHash = targetVersion.FileHash
	document.FileName = targetVersion.FileName

	if err := tx.Save(&document).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionVersionRollback,
		Details:    models.DocumentMetadata{"version": targetVersion.VersionNumber},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	tx.Create(&auditLog)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Document rolled back successfully",
		"version": targetVersion.VersionNumber,
	})
}

// DownloadDocumentVersionHandler downloads a specific version of a document
func DownloadDocumentVersionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	versionID := vars["version_id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	var version models.DocumentVersion
	if err := config.DB.First(&version, "id = ? AND document_id = ?", versionID, documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "version not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch version: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if file exists
	if _, err := os.Stat(version.FilePath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Log audit
	auditLog := models.DocumentAuditLog{
		DocumentID: uuid.MustParse(documentID),
		UserID:     &userID,
		Action:     models.DocumentAuditActionDownload,
		Details:    models.DocumentMetadata{"version": version.VersionNumber},
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", version.FileName))
	w.Header().Set("Content-Type", version.FileType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", version.FileSize))

	// Serve file
	http.ServeFile(w, r, version.FilePath)
}

// CompareDocumentVersionsHandler compares two versions of a document
func CompareDocumentVersionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	documentID := vars["id"]
	version1ID := r.URL.Query().Get("version1")
	version2ID := r.URL.Query().Get("version2")

	if version1ID == "" || version2ID == "" {
		http.Error(w, "both version1 and version2 parameters are required", http.StatusBadRequest)
		return
	}

	var version1, version2 models.DocumentVersion
	if err := config.DB.Preload("CreatedBy").First(&version1, "id = ? AND document_id = ?", version1ID, documentID).Error; err != nil {
		http.Error(w, "version1 not found", http.StatusNotFound)
		return
	}

	if err := config.DB.Preload("CreatedBy").First(&version2, "id = ? AND document_id = ?", version2ID, documentID).Error; err != nil {
		http.Error(w, "version2 not found", http.StatusNotFound)
		return
	}

	// Build comparison
	comparison := map[string]interface{}{
		"version1": map[string]interface{}{
			"id":             version1.ID,
			"version_number": version1.VersionNumber,
			"file_name":      version1.FileName,
			"file_size":      version1.FileSize,
			"file_type":      version1.FileType,
			"file_hash":      version1.FileHash,
			"change_log":     version1.ChangeLog,
			"created_by":     version1.CreatedBy,
			"created_at":     version1.CreatedAt,
		},
		"version2": map[string]interface{}{
			"id":             version2.ID,
			"version_number": version2.VersionNumber,
			"file_name":      version2.FileName,
			"file_size":      version2.FileSize,
			"file_type":      version2.FileType,
			"file_hash":      version2.FileHash,
			"change_log":     version2.ChangeLog,
			"created_by":     version2.CreatedBy,
			"created_at":     version2.CreatedAt,
		},
		"differences": map[string]interface{}{
			"file_name_changed": version1.FileName != version2.FileName,
			"file_size_delta":   version2.FileSize - version1.FileSize,
			"content_changed":   version1.FileHash != version2.FileHash,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comparison)
}

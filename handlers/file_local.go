package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	uploadDir = "./uploads" // Local directory for file storage
)

// UploadFileLocal handles file uploads to local filesystem instead of GCS
func UploadFileLocal(w http.ResponseWriter, r *http.Request) {
	// Ensure upload directory exists
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "failed to create upload directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the multipart form (max 50MB)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "bad multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create unique filename with timestamp to avoid collisions
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s", timestamp, header.Filename)
	filepath := filepath.Join(uploadDir, filename)

	// Create the file on disk
	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "failed to create file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy uploaded content to file
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return local file URL
	// In production, you'd use your domain. For dev, use relative path
	url := fmt.Sprintf("/uploads/%s", filename)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":      url,
		"filename": filename,
	})
}

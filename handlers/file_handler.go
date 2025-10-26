package handlers

import (
	"net/http"
	"os"
)

// UploadFileHandler routes to the appropriate upload handler based on environment
func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	// Check if running in production (Google Cloud)
	// Google Cloud sets GOOGLE_APPLICATION_CREDENTIALS or K_SERVICE (Cloud Run)
	useGCS := os.Getenv("USE_GCS") == "true" ||
		os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" ||
		os.Getenv("K_SERVICE") != "" // Cloud Run indicator

	if useGCS {
		// Production: Use Google Cloud Storage
		UploadFile(w, r)
	} else {
		// Development: Use local file storage
		UploadFileLocal(w, r)
	}
}

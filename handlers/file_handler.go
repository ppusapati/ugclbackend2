package handlers

import (
	"net/http"
)

// UploadFileHandler routes to the appropriate upload handler based on environment
func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	if useGCSStorage() {
		// Production: Use Google Cloud Storage
		UploadFile(w, r)
	} else {
		// Development: Use local file storage
		UploadFileLocal(w, r)
	}
}

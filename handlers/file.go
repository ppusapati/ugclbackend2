package handlers

import (
	"encoding/json"
	"net/http"
)

func UploadFile(w http.ResponseWriter, r *http.Request) {
	upload, err := storeUploadedFile(r, "file", "./uploads")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": upload.URL})
}

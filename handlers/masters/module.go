package masters

import (
	"encoding/json"
	"fmt"
	"net/http"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// GetModules returns all modules
// GET /api/v1/modules
func GetModules(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var modules []models.Module
	if err := config.DB.
		Order("created_at DESC").
		Find(&modules).Error; err != nil {
		fmt.Println("Error fetching modules:", err)
		http.Error(w, "failed to fetch modules", http.StatusInternalServerError)
		return
	}
	fmt.Println("Fetched modules:", len(modules))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	})
}

func CreateModule(w http.ResponseWriter, r *http.Request) {
	fmt.Println("CreateModule called")
	claims := middleware.GetClaims(r)
	fmt.Println("Claims:", claims)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var module models.Module
	if err := json.NewDecoder(r.Body).Decode(&module); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	fmt.Println("Module to create:", module)
	if err := config.DB.Create(&module).Error; err != nil {
		http.Error(w, "failed to create module", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"module": module,
	})
}

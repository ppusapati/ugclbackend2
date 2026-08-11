package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// CreateFormTableHandler creates a dedicated table for a form (admin only)
// POST /api/v1/admin/forms/{formCode}/create-table
func CreateFormTableHandler(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	log.Printf("📊 Creating dedicated table for form: %s", formCode)

	// Get the form
	var form models.AppForm
	if err := db.Where("code = ?", formCode).First(&form).Error; err != nil {
		log.Printf("❌ Form not found: %s", formCode)
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	// Check if form has a table name
	if form.DBTableName == "" {
		http.Error(w, "form does not have a table name configured", http.StatusBadRequest)
		return
	}

	// Create table manager
	tableManager := NewFormTableManager(db)

	// Check if table already exists
	exists, err := tableManager.TableExists(form.DBTableName)
	if err != nil {
		log.Printf("❌ Error checking table existence: %v", err)
		http.Error(w, "failed to check table existence", http.StatusInternalServerError)
		return
	}

	if exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":    "table already exists",
			"table_name": form.DBTableName,
			"form_code":  formCode,
		})
		return
	}

	// Create the table
	if err := tableManager.CreateFormTable(&form); err != nil {
		log.Printf("❌ Error creating table: %v", err)
		http.Error(w, "failed to create table", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Successfully created table: %s for form: %s", form.DBTableName, formCode)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "table created successfully",
		"table_name": form.DBTableName,
		"form_code":  formCode,
	})
}

// CheckFormTableStatus checks if a form's dedicated table exists
// GET /api/v1/admin/forms/{formCode}/table-status
func CheckFormTableStatus(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	// Get the form
	var form models.AppForm
	if err := db.Where("code = ?", formCode).First(&form).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	tableManager := NewFormTableManager(db)

	var exists bool
	if form.DBTableName != "" {
		exists, err = tableManager.TableExists(form.DBTableName)
		if err != nil {
			http.Error(w, "failed to check table status", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"form_code":        formCode,
		"table_name":       form.DBTableName,
		"has_table_name":   form.DBTableName != "",
		"table_exists":     exists,
		"using_dedicated":  form.DBTableName != "" && exists,
	})
}

// DropFormTableHandler drops a form's dedicated table (use with caution!)
// DELETE /api/v1/admin/forms/{formCode}/table
func DropFormTableHandler(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	formCode := vars["formCode"]

	log.Printf("⚠️  Request to drop table for form: %s", formCode)

	// Get the form
	var form models.AppForm
	if err := db.Where("code = ?", formCode).First(&form).Error; err != nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	if form.DBTableName == "" {
		http.Error(w, "form does not have a table name configured", http.StatusBadRequest)
		return
	}

	// Require confirmation parameter
	if r.URL.Query().Get("confirm") != "true" {
		http.Error(w, "confirmation required - add ?confirm=true to URL", http.StatusBadRequest)
		return
	}

	// Drop the table
	tableManager := NewFormTableManager(db)
	if err := tableManager.DropFormTable(form.DBTableName); err != nil {
		log.Printf("❌ Error dropping table: %v", err)
		http.Error(w, "failed to drop table", http.StatusInternalServerError)
		return
	}

	log.Printf("⚠️  Successfully dropped table: %s for form: %s", form.DBTableName, formCode)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "table dropped successfully",
		"table_name": form.DBTableName,
		"form_code":  formCode,
		"warning":    "all data in this table has been permanently deleted",
	})
}

// BulkCreateFormTablesHandler creates tables for all forms that need them
// POST /api/v1/admin/forms/create-all-tables
func BulkCreateFormTablesHandler(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("📊 Bulk creating tables for all forms with table names")

	// Get all forms with table names
	var forms []models.AppForm
	if err := db.Where("table_name IS NOT NULL AND table_name != ''").Find(&forms).Error; err != nil {
		http.Error(w, "failed to fetch forms", http.StatusInternalServerError)
		return
	}

	tableManager := NewFormTableManager(db)
	results := make([]map[string]interface{}, 0)

	for _, form := range forms {
		result := map[string]interface{}{
			"form_code":  form.Code,
			"table_name": form.DBTableName,
		}

		// Check if table exists
		exists, err := tableManager.TableExists(form.DBTableName)
		if err != nil {
			result["status"] = "error"
			result["message"] = "failed to check table existence"
			results = append(results, result)
			continue
		}

		if exists {
			result["status"] = "skipped"
			result["message"] = "table already exists"
		} else {
			// Create table
			if err := tableManager.CreateFormTable(&form); err != nil {
				result["status"] = "error"
				result["message"] = err.Error()
			} else {
				result["status"] = "created"
				result["message"] = "table created successfully"
			}
		}

		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "bulk table creation completed",
		"results": results,
		"total":   len(results),
	})
}

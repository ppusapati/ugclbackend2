package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// CreateDocumentCategoryHandler creates a new document category
func CreateDocumentCategoryHandler(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Name               string `json:"name"`
		Description        string `json:"description"`
		ParentID           string `json:"parent_id"`
		Color              string `json:"color"`
		Icon               string `json:"icon"`
		BusinessVerticalID string `json:"business_vertical_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	category := models.DocumentCategory{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		IsActive:    true,
	}

	if req.ParentID != "" {
		parentID, err := uuid.Parse(req.ParentID)
		if err == nil {
			category.ParentID = &parentID
		}
	}

	// Get business vertical from request or user's accessible verticals
	if req.BusinessVerticalID != "" {
		bvID, err := uuid.Parse(req.BusinessVerticalID)
		if err == nil {
			category.BusinessVerticalID = &bvID
		}
	} else if len(user.UserBusinessRoles) > 0 && user.UserBusinessRoles[0].BusinessRole.BusinessVerticalID != uuid.Nil {
		// Use user's primary business vertical
		bvID := user.UserBusinessRoles[0].BusinessRole.BusinessVerticalID
		category.BusinessVerticalID = &bvID
	}

	if err := config.DB.Create(&category).Error; err != nil {
		http.Error(w, "failed to create category: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load relationships
	config.DB.Preload("Parent").Preload("BusinessVertical").First(&category, category.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Category created successfully",
		"category": category,
	})

}

// GetDocumentCategoriesHandler returns all document categories
func GetDocumentCategoriesHandler(w http.ResponseWriter, r *http.Request) {
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

	businessVerticalID := r.URL.Query().Get("business_vertical_id")

	query := config.DB.Model(&models.DocumentCategory{}).
		Preload("Parent").
		Preload("BusinessVertical").
		Where("is_active = ?", true)

	if businessVerticalID != "" {
		query = query.Where("business_vertical_id = ? OR business_vertical_id IS NULL", businessVerticalID)
	}

	var categories []models.DocumentCategory
	if err := query.Order("name ASC").Find(&categories).Error; err != nil {
		http.Error(w, "failed to fetch categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

// GetDocumentCategoryHandler returns a single category by ID
func GetDocumentCategoryHandler(w http.ResponseWriter, r *http.Request) {
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
	categoryID := vars["id"]

	var category models.DocumentCategory
	if err := config.DB.Preload("Parent").Preload("BusinessVertical").
		First(&category, "id = ?", categoryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "category not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch category: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(category)
}

// UpdateDocumentCategoryHandler updates a category
func UpdateDocumentCategoryHandler(w http.ResponseWriter, r *http.Request) {
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
	categoryID := vars["id"]

	var category models.DocumentCategory
	if err := config.DB.First(&category, "id = ?", categoryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "category not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch category: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ParentID    string `json:"parent_id"`
		Color       string `json:"color"`
		Icon        string `json:"icon"`
		IsActive    *bool  `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		category.Name = req.Name
	}
	if req.Description != "" {
		category.Description = req.Description
	}
	if req.Color != "" {
		category.Color = req.Color
	}
	if req.Icon != "" {
		category.Icon = req.Icon
	}
	if req.ParentID != "" {
		parentID, err := uuid.Parse(req.ParentID)
		if err == nil {
			category.ParentID = &parentID
		}
	}
	if req.IsActive != nil {
		category.IsActive = *req.IsActive
	}

	if err := config.DB.Save(&category).Error; err != nil {
		http.Error(w, "failed to update category: "+err.Error(), http.StatusInternalServerError)
		return
	}

	config.DB.Preload("Parent").Preload("BusinessVertical").First(&category, category.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Category updated successfully",
		"category": category,
	})
}

// DeleteDocumentCategoryHandler soft deletes a category
func DeleteDocumentCategoryHandler(w http.ResponseWriter, r *http.Request) {
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
	categoryID := vars["id"]

	var category models.DocumentCategory
	if err := config.DB.First(&category, "id = ?", categoryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "category not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch category: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if category has documents
	var docCount int64
	config.DB.Model(&models.Document{}).Where("category_id = ?", categoryID).Count(&docCount)
	if docCount > 0 {
		http.Error(w, "cannot delete category with documents", http.StatusConflict)
		return
	}

	if err := config.DB.Delete(&category).Error; err != nil {
		http.Error(w, "failed to delete category: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Category deleted successfully",
	})
}

// GetDocumentTagsHandler returns all document tags
func GetDocumentTagsHandler(w http.ResponseWriter, r *http.Request) {
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

	businessVerticalID := r.URL.Query().Get("business_vertical_id")

	query := config.DB.Model(&models.DocumentTag{}).Preload("BusinessVertical")

	if businessVerticalID != "" {
		query = query.Where("business_vertical_id = ? OR business_vertical_id IS NULL", businessVerticalID)
	}

	var tags []models.DocumentTag
	if err := query.Order("name ASC").Find(&tags).Error; err != nil {
		http.Error(w, "failed to fetch tags: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

// CreateDocumentTagHandler creates a new tag
func CreateDocumentTagHandler(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Name               string `json:"name"`
		Color              string `json:"color"`
		BusinessVerticalID string `json:"business_vertical_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	tag := models.DocumentTag{
		Name:  req.Name,
		Color: req.Color,
	}

	// Get business vertical from request or user's accessible verticals
	if req.BusinessVerticalID != "" {
		bvID, err := uuid.Parse(req.BusinessVerticalID)
		if err == nil {
			tag.BusinessVerticalID = &bvID
		}
	} else if len(user.UserBusinessRoles) > 0 && user.UserBusinessRoles[0].BusinessRole.BusinessVerticalID != uuid.Nil {
		// Use user's primary business vertical
		bvID := user.UserBusinessRoles[0].BusinessRole.BusinessVerticalID
		tag.BusinessVerticalID = &bvID
	}

	if err := config.DB.Create(&tag).Error; err != nil {
		http.Error(w, "failed to create tag: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tag created successfully",
		"tag":     tag,
	})
}

// UpdateDocumentTagHandler updates a tag
func UpdateDocumentTagHandler(w http.ResponseWriter, r *http.Request) {
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
	tagID := vars["id"]

	var tag models.DocumentTag
	if err := config.DB.First(&tag, "id = ?", tagID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "tag not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch tag: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		tag.Name = req.Name
	}
	if req.Color != "" {
		tag.Color = req.Color
	}

	if err := config.DB.Save(&tag).Error; err != nil {
		http.Error(w, "failed to update tag: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tag updated successfully",
		"tag":     tag,
	})
}

// DeleteDocumentTagHandler deletes a tag
func DeleteDocumentTagHandler(w http.ResponseWriter, r *http.Request) {
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
	tagID := vars["id"]

	var tag models.DocumentTag
	if err := config.DB.First(&tag, "id = ?", tagID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "tag not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch tag: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Remove tag associations
	config.DB.Exec("DELETE FROM document_tags WHERE tag_id = ?", tagID)

	if err := config.DB.Delete(&tag).Error; err != nil {
		http.Error(w, "failed to delete tag: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Tag deleted successfully",
	})
}

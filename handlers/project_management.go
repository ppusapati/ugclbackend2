package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// ProjectHandler handles project management operations
type ProjectHandler struct {
	db        *gorm.DB
	kmzParser *KMZParser
}

// NewProjectHandler creates a new project handler
func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{
		db:        config.DB,
		kmzParser: NewKMZParser(),
	}
}

// CreateProjectRequest represents the request to create a project
type CreateProjectRequest struct {
	Code               string     `json:"code"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	BusinessVerticalID uuid.UUID  `json:"business_vertical_id"`
	StartDate          *time.Time `json:"start_date"`
	EndDate            *time.Time `json:"end_date"`
	TotalBudget        float64    `json:"total_budget"`
	Currency           string     `json:"currency"`
}

// UpdateProjectRequest represents the request to update a project
type UpdateProjectRequest struct {
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	StartDate          *time.Time `json:"start_date"`
	EndDate            *time.Time `json:"end_date"`
	TotalBudget        float64    `json:"total_budget"`
	Status             string     `json:"status"`
	Progress           float64    `json:"progress"`
}

// CreateProject creates a new project
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Code == "" || req.Name == "" {
		http.Error(w, "Code and Name are required", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	claims := middleware.GetClaims(r)
	userID := claims.UserID

	// Create project
	project := models.Project{
		Code:               req.Code,
		Name:               req.Name,
		Description:        req.Description,
		BusinessVerticalID: req.BusinessVerticalID,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		TotalBudget:        req.TotalBudget,
		Currency:           req.Currency,
		Status:             "draft",
		Progress:           0,
		CreatedBy:          userID,
	}

	if project.Currency == "" {
		project.Currency = "INR"
	}

	if err := h.db.Create(&project).Error; err != nil {
		log.Printf("❌ Failed to create project: %v", err)
		http.Error(w, "Failed to create project", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Created project: %s (ID: %s)", project.Name, project.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Project created successfully",
		"project": project,
	})
}

// UploadKMZ handles KMZ file upload and processing
func (h *ProjectHandler) UploadKMZ(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	// Get project
	var project models.Project
	if err := h.db.First(&project, "id = ?", projectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("kmz_file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	kmzData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Parse KMZ
	parsedData, err := h.kmzParser.ParseKMZToStructuredData(kmzData)
	if err != nil {
		log.Printf("❌ Failed to parse KMZ: %v", err)
		http.Error(w, fmt.Sprintf("Failed to parse KMZ: %v", err), http.StatusBadRequest)
		return
	}

	// Start transaction
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Store GeoJSON data
	geoJSONBytes, _ := json.Marshal(parsedData.GeoJSON)
	now := time.Now()

	// Update project with KMZ info
	project.KMZFileName = header.Filename
	project.KMZFilePath = fmt.Sprintf("projects/%s/%s", projectID, header.Filename)
	project.KMZUploadedAt = &now
	project.GeoJSONData = json.RawMessage(geoJSONBytes)

	if err := tx.Save(&project).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to update project", http.StatusInternalServerError)
		return
	}

	// Create zones
	zoneMap := make(map[string]uuid.UUID) // name -> zone ID
	for _, zoneData := range parsedData.Zones {
		zoneGeoJSON, _ := json.Marshal(zoneData.GeoJSON)
		zoneProps, _ := json.Marshal(zoneData.Properties)

		zone := models.Zone{
			ProjectID:  project.ID,
			Name:       zoneData.Name,
			Code:       zoneData.Code,
			Label:      zoneData.Label,
			GeoJSON:    json.RawMessage(zoneGeoJSON),
			Properties: json.RawMessage(zoneProps),
		}

		if err := tx.Create(&zone).Error; err != nil {
			tx.Rollback()
			log.Printf("❌ Failed to create zone: %v", err)
			http.Error(w, "Failed to create zones", http.StatusInternalServerError)
			return
		}

		zoneMap[zoneData.Name] = zone.ID
	}

	// Create nodes
	for _, nodeData := range parsedData.Nodes {
		nodeGeoJSON, _ := json.Marshal(nodeData.GeoJSON)
		nodeProps, _ := json.Marshal(nodeData.Properties)

		// Find zone for this node
		var zoneID uuid.UUID
		if folder, ok := nodeData.Properties["folder"].(string); ok {
			if id, exists := zoneMap[folder]; exists {
				zoneID = id
			}
		}
		// Default to first zone if no specific zone found
		if zoneID == uuid.Nil && len(zoneMap) > 0 {
			for _, id := range zoneMap {
				zoneID = id
				break
			}
		}

		// Create PostGIS POINT from coordinates
		locationWKT := fmt.Sprintf("SRID=4326;POINT(%f %f)", nodeData.Longitude, nodeData.Latitude)

		node := models.Node{
			ProjectID:   project.ID,
			ZoneID:      zoneID,
			Name:        nodeData.Name,
			Code:        nodeData.Code,
			Label:       nodeData.Label,
			NodeType:    nodeData.NodeType,
			Latitude:    nodeData.Latitude,
			Longitude:   nodeData.Longitude,
			Elevation:   nodeData.Elevation,
			Location:    locationWKT,
			GeoJSON:     json.RawMessage(nodeGeoJSON),
			Properties:  json.RawMessage(nodeProps),
			Status:      "available",
		}

		if err := tx.Create(&node).Error; err != nil {
			tx.Rollback()
			log.Printf("❌ Failed to create node: %v", err)
			http.Error(w, "Failed to create nodes", http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Successfully processed KMZ for project %s: %d zones, %d nodes",
		project.ID, len(parsedData.Zones), len(parsedData.Nodes))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "KMZ uploaded and processed successfully",
		"total_features": parsedData.TotalFeatures,
		"zones_created":  len(parsedData.Zones),
		"nodes_created":  len(parsedData.Nodes),
		"labels_found":   len(parsedData.Labels),
	})
}

// GetProject retrieves a project by ID
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var project models.Project
	if err := h.db.
		Preload("BusinessVertical").
		Preload("Zones").
		Preload("Tasks").
		First(&project, "id = ?", projectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

// ListProjects lists all projects with filters
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	var projects []models.Project

	query := h.db.Preload("BusinessVertical")

	// Apply filters
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if businessVerticalID := r.URL.Query().Get("business_vertical_id"); businessVerticalID != "" {
		query = query.Where("business_vertical_id = ?", businessVerticalID)
	}

	if err := query.Order("created_at DESC").Find(&projects).Error; err != nil {
		http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"projects": projects,
		"count":    len(projects),
	})
}

// UpdateProject updates a project
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var req UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var project models.Project
	if err := h.db.First(&project, "id = ?", projectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// Get user ID from context
	claims := middleware.GetClaims(r)
	userID := claims.UserID

	// Update fields
	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Description != "" {
		project.Description = req.Description
	}
	if req.StartDate != nil {
		project.StartDate = req.StartDate
	}
	if req.EndDate != nil {
		project.EndDate = req.EndDate
	}
	if req.TotalBudget > 0 {
		project.TotalBudget = req.TotalBudget
	}
	if req.Status != "" {
		project.Status = req.Status
	}
	if req.Progress >= 0 {
		project.Progress = req.Progress
	}

	project.UpdatedBy = userID

	if err := h.db.Save(&project).Error; err != nil {
		http.Error(w, "Failed to update project", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Updated project: %s", project.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Project updated successfully",
		"project": project,
	})
}

// DeleteProject soft deletes a project
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var project models.Project
	if err := h.db.First(&project, "id = ?", projectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	if err := h.db.Delete(&project).Error; err != nil {
		http.Error(w, "Failed to delete project", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Deleted project: %s", projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Project deleted successfully",
	})
}

// GetProjectZones retrieves all zones for a project
func (h *ProjectHandler) GetProjectZones(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var zones []models.Zone
	if err := h.db.Where("project_id = ?", projectID).Find(&zones).Error; err != nil {
		http.Error(w, "Failed to fetch zones", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"zones": zones,
		"count": len(zones),
	})
}

// GetProjectNodes retrieves all nodes for a project
func (h *ProjectHandler) GetProjectNodes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var nodes []models.Node
	query := h.db.Where("project_id = ?", projectID)

	// Filter by node type
	if nodeType := r.URL.Query().Get("node_type"); nodeType != "" {
		query = query.Where("node_type = ?", nodeType)
	}

	// Filter by status
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&nodes).Error; err != nil {
		http.Error(w, "Failed to fetch nodes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

// GetProjectGeoJSON retrieves the GeoJSON data for a project
func (h *ProjectHandler) GetProjectGeoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var project models.Project
	if err := h.db.First(&project, "id = ?", projectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	if len(project.GeoJSONData) == 0 {
		http.Error(w, "No GeoJSON data available", http.StatusNotFound)
		return
	}

	// Parse and return GeoJSON
	var geoJSON interface{}
	if err := json.Unmarshal(project.GeoJSONData, &geoJSON); err != nil {
		http.Error(w, "Failed to parse GeoJSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(geoJSON)
}

// GetProjectStats retrieves statistics for a project
func (h *ProjectHandler) GetProjectStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	var project models.Project
	if err := h.db.First(&project, "id = ?", projectID).Error; err != nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// Count zones
	var zoneCount int64
	h.db.Model(&models.Zone{}).Where("project_id = ?", projectID).Count(&zoneCount)

	// Count nodes by type
	var nodeStats []struct {
		NodeType string
		Count    int64
	}
	h.db.Model(&models.Node{}).
		Select("node_type, count(*) as count").
		Where("project_id = ?", projectID).
		Group("node_type").
		Scan(&nodeStats)

	// Count tasks by status
	var taskStats []struct {
		Status string
		Count  int64
	}
	h.db.Model(&models.Task{}).
		Select("status, count(*) as count").
		Where("project_id = ?", projectID).
		Group("status").
		Scan(&taskStats)

	// Budget stats
	var budgetStats struct {
		TotalAllocated float64
		TotalSpent     float64
	}
	h.db.Model(&models.BudgetAllocation{}).
		Select("COALESCE(SUM(planned_amount), 0) as total_allocated, COALESCE(SUM(actual_amount), 0) as total_spent").
		Where("project_id = ?", projectID).
		Scan(&budgetStats)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"project_id":       projectID,
		"total_budget":     project.TotalBudget,
		"allocated_budget": budgetStats.TotalAllocated,
		"spent_budget":     budgetStats.TotalSpent,
		"progress":         project.Progress,
		"status":           project.Status,
		"zones_count":      zoneCount,
		"nodes_by_type":    nodeStats,
		"tasks_by_status":  taskStats,
	})
}

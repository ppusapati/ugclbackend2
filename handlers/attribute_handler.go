package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/pkg/abac"
)

// CreateAttribute creates a new attribute definition
func CreateAttribute(w http.ResponseWriter, r *http.Request) {
	var attribute models.Attribute
	if err := json.NewDecoder(r.Body).Decode(&attribute); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	createdAttr, err := attributeService.CreateAttribute(attribute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdAttr)
}

// UpdateAttribute updates an attribute definition
func UpdateAttribute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attributeID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	updatedAttr, err := attributeService.UpdateAttribute(attributeID, updates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedAttr)
}

// DeleteAttribute deletes an attribute definition
func DeleteAttribute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attributeID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	if err := attributeService.DeleteAttribute(attributeID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAttributes lists all attributes with optional filtering
func ListAttributes(w http.ResponseWriter, r *http.Request) {
	typeStr := r.URL.Query().Get("type")
	activeStr := r.URL.Query().Get("active")

	var attrType *models.AttributeType
	if typeStr != "" {
		t := models.AttributeType(typeStr)
		attrType = &t
	}

	var isActive *bool
	if activeStr != "" {
		if activeStr == "true" {
			t := true
			isActive = &t
		} else if activeStr == "false" {
			f := false
			isActive = &f
		}
	}

	attributeService := abac.NewAttributeService(config.DB)
	attributes, err := attributeService.ListAttributes(attrType, isActive)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attributes)
}

// AssignUserAttribute assigns an attribute to a user
func AssignUserAttribute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["user_id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		AttributeID string     `json:"attribute_id"`
		Value       string     `json:"value"`
		ValidUntil  *time.Time `json:"valid_until"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	attributeID, err := uuid.Parse(req.AttributeID)
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	assignedByStr := middleware.GetUserID(r)
	assignedBy, err := uuid.Parse(assignedByStr)
	if err != nil {
		http.Error(w, "Invalid assigned by user ID", http.StatusInternalServerError)
		return
	}
	attributeService := abac.NewAttributeService(config.DB)

	if err := attributeService.AssignUserAttribute(userID, attributeID, assignedBy, req.Value, req.ValidUntil); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Attribute assigned successfully"})
}

// RemoveUserAttribute removes an attribute from a user
func RemoveUserAttribute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["user_id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	attributeID, err := uuid.Parse(vars["attribute_id"])
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	if err := attributeService.RemoveUserAttribute(userID, attributeID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetUserAttributes retrieves all attributes for a user
func GetUserAttributes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["user_id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	attributes, err := attributeService.GetUserAttributes(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attributes)
}

// BulkAssignUserAttributes assigns multiple attributes to a user
func BulkAssignUserAttributes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["user_id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Attributes map[string]string `json:"attributes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	assignedByStr := middleware.GetUserID(r)
	assignedBy, err := uuid.Parse(assignedByStr)
	if err != nil {
		http.Error(w, "Invalid assigned by user ID", http.StatusInternalServerError)
		return
	}
	attributeService := abac.NewAttributeService(config.DB)

	if err := attributeService.BulkAssignUserAttributes(userID, assignedBy, req.Attributes); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Attributes assigned successfully"})
}

// AssignResourceAttribute assigns an attribute to a resource
func AssignResourceAttribute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ResourceType string     `json:"resource_type"`
		ResourceID   string     `json:"resource_id"`
		AttributeID  string     `json:"attribute_id"`
		Value        string     `json:"value"`
		ValidUntil   *time.Time `json:"valid_until"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resourceID, err := uuid.Parse(req.ResourceID)
	if err != nil {
		http.Error(w, "Invalid resource ID", http.StatusBadRequest)
		return
	}

	attributeID, err := uuid.Parse(req.AttributeID)
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	assignedByStr := middleware.GetUserID(r)
	assignedBy, err := uuid.Parse(assignedByStr)
	if err != nil {
		http.Error(w, "Invalid assigned by user ID", http.StatusInternalServerError)
		return
	}
	attributeService := abac.NewAttributeService(config.DB)

	if err := attributeService.AssignResourceAttribute(req.ResourceType, resourceID, attributeID, assignedBy, req.Value, req.ValidUntil); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Attribute assigned successfully"})
}

// RemoveResourceAttribute removes an attribute from a resource
func RemoveResourceAttribute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceType := vars["resource_type"]

	resourceID, err := uuid.Parse(vars["resource_id"])
	if err != nil {
		http.Error(w, "Invalid resource ID", http.StatusBadRequest)
		return
	}

	attributeID, err := uuid.Parse(vars["attribute_id"])
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	if err := attributeService.RemoveResourceAttribute(resourceType, resourceID, attributeID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetResourceAttributes retrieves all attributes for a resource
func GetResourceAttributes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceType := vars["resource_type"]

	resourceID, err := uuid.Parse(vars["resource_id"])
	if err != nil {
		http.Error(w, "Invalid resource ID", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	attributes, err := attributeService.GetResourceAttributes(resourceType, resourceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attributes)
}

// GetUserAttributeHistory retrieves attribute assignment history for a user
func GetUserAttributeHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["user_id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	attributeID, err := uuid.Parse(vars["attribute_id"])
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	attributeService := abac.NewAttributeService(config.DB)
	history, err := attributeService.GetUserAttributeHistory(userID, attributeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

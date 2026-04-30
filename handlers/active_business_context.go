package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

type setActiveBusinessContextRequest struct {
	BusinessID   string `json:"business_id"`
	BusinessCode string `json:"business_code"`
	ClientKey    string `json:"client_key"`
}

// SetActiveBusinessContext stores the user's active business for the current client scope.
func SetActiveBusinessContext(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.NewAuthService().LoadUserContext(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req setActiveBusinessContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	businessID := resolveBusinessSelection(req.BusinessID, req.BusinessCode)
	if businessID == uuid.Nil {
		http.Error(w, "business_id or business_code is required", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessBusiness(userCtx, businessID) {
		http.Error(w, middleware.ErrNoBusinessAccess.Message, middleware.ErrNoBusinessAccess.Code)
		return
	}

	clientKey := strings.TrimSpace(req.ClientKey)
	if clientKey == "" {
		clientKey = middleware.GetActiveBusinessClientKey(r)
	}

	activeContext, err := middleware.SaveActiveBusinessContext(userCtx.User.ID, businessID, clientKey)
	if err != nil {
		http.Error(w, "failed to save active business context", http.StatusInternalServerError)
		return
	}

	respondWithActiveBusinessContext(w, http.StatusOK, activeContext, "stored")
}

// GetActiveBusinessContext returns the effective active business for the current request.
func GetActiveBusinessContext(w http.ResponseWriter, r *http.Request) {
	userCtx, err := middleware.NewAuthService().LoadUserContext(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	businessID, err := middleware.ResolveEffectiveBusinessID(r, userCtx)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	clientKey := middleware.GetActiveBusinessClientKey(r)
	activeContext, err := middleware.GetStoredActiveBusinessContext(userCtx.User.ID, clientKey)
	if err == nil && activeContext.BusinessID == businessID {
		respondWithActiveBusinessContext(w, http.StatusOK, activeContext, "stored")
		return
	}

	var business models.BusinessVertical
	if err := config.DB.First(&business, "id = ?", businessID).Error; err != nil {
		http.Error(w, "business not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"business_id":   business.ID,
		"business_code": business.Code,
		"business_name": business.Name,
		"client_key":    clientKey,
		"source":        inferBusinessSource(userCtx, businessID),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func resolveBusinessSelection(rawBusinessID, rawBusinessCode string) uuid.UUID {
	if id := strings.TrimSpace(rawBusinessID); id != "" {
		if parsedID, err := uuid.Parse(id); err == nil {
			return parsedID
		}
		if resolved := middleware.ResolveBusinessIdentifier(id); resolved != uuid.Nil {
			return resolved
		}
	}

	if code := strings.TrimSpace(rawBusinessCode); code != "" {
		return middleware.ResolveBusinessIdentifier(code)
	}

	return uuid.Nil
}

func respondWithActiveBusinessContext(w http.ResponseWriter, status int, activeContext *models.UserActiveBusinessContext, source string) {
	response := map[string]interface{}{
		"business_id": activeContext.BusinessID,
		"client_key":  activeContext.ClientKey,
		"source":      source,
	}

	if activeContext.Business != nil {
		response["business_code"] = activeContext.Business.Code
		response["business_name"] = activeContext.Business.Name
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func inferBusinessSource(userCtx *middleware.UserContext, businessID uuid.UUID) string {
	if userCtx.User.BusinessVerticalID != nil && *userCtx.User.BusinessVerticalID == businessID {
		return "user_default"
	}

	accessible := middleware.NewAuthService().GetAccessibleBusinessVerticals(*userCtx.User)
	if len(accessible) == 1 && accessible[0] == businessID {
		return "single_accessible_business"
	}

	return "request"
}

func writeAuthError(w http.ResponseWriter, err error) {
	if authErr, ok := err.(*middleware.AuthError); ok {
		http.Error(w, authErr.Message, authErr.Code)
		return
	}

	http.Error(w, "authorization error", http.StatusInternalServerError)
}

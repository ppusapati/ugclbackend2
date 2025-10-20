package handlers

// import (
// 	"encoding/json"
// 	"net/http"

// 	"p9e.in/ugcl/middleware"
// )

// // TestAuthEndpoint provides information about the current user's authentication status
// func TestAuthEndpoint(w http.ResponseWriter, r *http.Request) {
// 	claims := middleware.GetClaims(r)
// 	if claims == nil {
// 		http.Error(w, "not authenticated", http.StatusUnauthorized)
// 		return
// 	}

// 	user := middleware.GetUser(r)
// 	permissions := middleware.GetUserPermissions(r)

// 	response := map[string]interface{}{
// 		"authenticated": true,
// 		"user_id":       claims.UserID,
// 		"name":          claims.Name,
// 		"phone":         claims.Phone,
// 		"role":          claims.Role,
// 		"user_details":  user,
// 		"permissions":   permissions,
// 		"token_valid":   true,
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(response)
// }

// // TestPermissionEndpoint tests if user has specific permissions
// func TestPermissionEndpoint(w http.ResponseWriter, r *http.Request) {
// 	permission := r.URL.Query().Get("permission")
// 	if permission == "" {
// 		http.Error(w, "permission parameter required", http.StatusBadRequest)
// 		return
// 	}

// 	claims := middleware.GetClaims(r)
// 	if claims == nil {
// 		http.Error(w, "not authenticated", http.StatusUnauthorized)
// 		return
// 	}

// 	permissions := middleware.GetUserPermissions(r)
// 	hasPermission := false
// 	for _, p := range permissions {
// 		if p == permission {
// 			hasPermission = true
// 			break
// 		}
// 	}

// 	response := map[string]interface{}{
// 		"user_id":        claims.UserID,
// 		"role":           claims.Role,
// 		"permission":     permission,
// 		"has_permission": hasPermission,
// 		"all_permissions": permissions,
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(response)
// }

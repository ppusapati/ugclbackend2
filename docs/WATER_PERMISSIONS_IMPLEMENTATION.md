# Water Reports Permission Implementation Guide

## Overview
This guide explains how to enforce role-based permissions for water tanker reports based on business roles:
- **Supervisor**: Read-only (water:read_consumption)
- **Engineer**: Read + Write (water:read_consumption, inventory:create, inventory:update)
- **Water_Admin**: Full access (all water and inventory permissions)

## Backend Implementation

### 1. Route-Level Protection (Recommended Approach)

Update your routes to enforce permissions at the route level. This ensures unauthorized requests are blocked before reaching handlers.

#### File: `routes/business_routes.go`

```go
// Water Works specific routes
water := business.PathPrefix("/water").Subrouter()

// Read operations - Supervisor, Engineer, and Water_Admin can read
water.Handle("/reports/tanker", middleware.RequireBusinessPermission("water:read_consumption")(
    http.HandlerFunc(handlers.GetAllWaterTankerReports))).Methods("GET")

water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("water:read_consumption")(
    http.HandlerFunc(handlers.GetWaterTankerReport))).Methods("GET")

// Write operations - Engineer and Water_Admin can create
water.Handle("/reports/tanker", middleware.RequireBusinessPermission("inventory:create")(
    http.HandlerFunc(handlers.CreateWaterTankerReport))).Methods("POST")

// Batch operations - Engineer and Water_Admin
water.Handle("/reports/tanker/batch", middleware.RequireBusinessPermission("inventory:create")(
    http.HandlerFunc(handlers.BatchWaterReports))).Methods("POST")

// Update operations - Engineer and Water_Admin can update
water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:update")(
    http.HandlerFunc(handlers.UpdateWaterTankerReport))).Methods("PUT")

// Delete operations - Only Water_Admin can delete
water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:delete")(
    http.HandlerFunc(handlers.DeleteWaterTankerReport))).Methods("DELETE")

// Consumption data
water.Handle("/consumption", middleware.RequireBusinessPermission("water:read_consumption")(
    http.HandlerFunc(handlers.GetWaterConsumption))).Methods("GET")

// Supply management - Engineer and Water_Admin
water.Handle("/supply", middleware.RequireBusinessPermission("water:manage_supply")(
    http.HandlerFunc(handlers.GetWaterSupply))).Methods("GET")

// Quality control - Engineer and Water_Admin
water.Handle("/quality", middleware.RequireBusinessPermission("water:quality_control")(
    http.HandlerFunc(handlers.GetWaterQuality))).Methods("GET")
```

### 2. Handler-Level Permission Check (Granular Control)

For more complex scenarios where you need conditional logic based on permissions:

#### File: `handlers/water.go`

```go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/google/uuid"
    "p9e.in/ugcl/config"
    "p9e.in/ugcl/middleware"
    "p9e.in/ugcl/models"
)

// GetAllWaterTankerReports with business context filtering
func GetAllWaterTankerReports(w http.ResponseWriter, r *http.Request) {
    // Get business context (contains permissions, roles, business ID)
    businessContext := middleware.GetUserBusinessContext(r)
    if businessContext == nil {
        http.Error(w, "business context not found", http.StatusBadRequest)
        return
    }

    businessID, ok := businessContext["business_id"].(uuid.UUID)
    if !ok {
        http.Error(w, "invalid business context", http.StatusInternalServerError)
        return
    }

    // Parse query parameters
    params, err := models.ParseReportParams(r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := params.Validate(); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Filter reports by business vertical (important for multi-tenancy)
    service := models.NewReportService(config.DB, models.Water{})

    // Add business filter to query
    params.Filters["business_vertical_id"] = businessID.String()

    response, err := service.GetReport(params)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// UpdateWaterTankerReport with permission-based logic
func UpdateWaterTankerReport(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetClaims(r)
    if claims == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // Get user with business roles
    var user models.User
    if err := config.DB.Preload("UserBusinessRoles.BusinessRole.Permissions").
        First(&user, "id = ?", claims.UserID).Error; err != nil {
        http.Error(w, "user not found", http.StatusUnauthorized)
        return
    }

    businessContext := middleware.GetUserBusinessContext(r)
    if businessContext == nil {
        http.Error(w, "business context not found", http.StatusBadRequest)
        return
    }

    // Check if user has update permission
    permissions, _ := businessContext["permissions"].([]string)
    hasUpdatePermission := false
    for _, perm := range permissions {
        if perm == "inventory:update" || perm == "admin_all" {
            hasUpdatePermission = true
            break
        }
    }

    if !hasUpdatePermission {
        http.Error(w, "insufficient permissions to update reports", http.StatusForbidden)
        return
    }

    // Proceed with update logic
    params := mux.Vars(r)
    id, _ := strconv.Atoi(params["id"])

    var item models.Water
    if err := config.DB.First(&item, id).Error; err != nil {
        http.Error(w, "report not found", http.StatusNotFound)
        return
    }

    if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    if err := config.DB.Save(&item).Error; err != nil {
        http.Error(w, "failed to update report", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(item)
}
```

### 3. Helper Function for Permission Checking

Create a utility function to check permissions easily:

#### File: `middleware/business_auth.go` (add this function)

```go
// HasBusinessPermissionInContext checks if user has permission in current business context
func HasBusinessPermissionInContext(r *http.Request, permission string) bool {
    businessContext := GetUserBusinessContext(r)
    if businessContext == nil {
        return false
    }

    // Super admins have all permissions
    if isSuperAdmin, ok := businessContext["is_super_admin"].(bool); ok && isSuperAdmin {
        return true
    }

    // Check in user's permissions
    if permissions, ok := businessContext["permissions"].([]string); ok {
        for _, perm := range permissions {
            if perm == permission {
                return true
            }
        }
    }

    return false
}

// GetBusinessPermissions returns all permissions for the current business context
func GetBusinessPermissions(r *http.Request) []string {
    businessContext := GetUserBusinessContext(r)
    if businessContext == nil {
        return []string{}
    }

    if permissions, ok := businessContext["permissions"].([]string); ok {
        return permissions
    }

    return []string{}
}
```

## Frontend Implementation

### 1. Get User Permissions from Backend

When user logs in or navigates to a business vertical, fetch their permissions:

```javascript
// API call to get business context
const getBusinessContext = async (businessCode) => {
  const response = await fetch(`/api/v1/business/${businessCode}/context`, {
    headers: {
      'Authorization': `Bearer ${token}`,
      'x-api-key': API_KEY
    }
  });

  const context = await response.json();
  // Store in state management (Redux, Context API, etc.)
  return context;
  // Returns: {
  //   business_id: "uuid",
  //   permissions: ["water:read_consumption", "inventory:create", ...],
  //   business_roles: [...],
  //   is_admin: false,
  //   is_super_admin: false
  // }
};
```

### 2. Create Permission Helper Functions

```javascript
// utils/permissions.js
export const hasPermission = (userPermissions, permission) => {
  return userPermissions.includes(permission) || userPermissions.includes('admin_all');
};

export const hasAnyPermission = (userPermissions, permissions) => {
  return permissions.some(perm => hasPermission(userPermissions, perm));
};

export const hasAllPermissions = (userPermissions, permissions) => {
  return permissions.every(perm => hasPermission(userPermissions, perm));
};

// Role-based checks
export const canReadWaterReports = (permissions) => {
  return hasPermission(permissions, 'water:read_consumption');
};

export const canCreateWaterReports = (permissions) => {
  return hasPermission(permissions, 'inventory:create');
};

export const canUpdateWaterReports = (permissions) => {
  return hasPermission(permissions, 'inventory:update');
};

export const canDeleteWaterReports = (permissions) => {
  return hasPermission(permissions, 'inventory:delete');
};
```

### 3. Conditional UI Rendering (React Example)

```jsx
import React, { useContext } from 'react';
import { BusinessContext } from './contexts/BusinessContext';
import { canCreateWaterReports, canDeleteWaterReports } from './utils/permissions';

const WaterReportsPage = () => {
  const { permissions } = useContext(BusinessContext);

  return (
    <div>
      <h1>Water Tanker Reports</h1>

      {/* Show create button only if user has permission */}
      {canCreateWaterReports(permissions) && (
        <button onClick={handleCreate}>
          Create New Report
        </button>
      )}

      <table>
        <thead>
          <tr>
            <th>Date</th>
            <th>Volume</th>
            <th>Location</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {reports.map(report => (
            <tr key={report.id}>
              <td>{report.date}</td>
              <td>{report.volume}</td>
              <td>{report.location}</td>
              <td>
                {/* View button - everyone can view */}
                <button onClick={() => handleView(report.id)}>View</button>

                {/* Edit button - only Engineer and Water_Admin */}
                {canUpdateWaterReports(permissions) && (
                  <button onClick={() => handleEdit(report.id)}>Edit</button>
                )}

                {/* Delete button - only Water_Admin */}
                {canDeleteWaterReports(permissions) && (
                  <button onClick={() => handleDelete(report.id)}>Delete</button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
```

### 4. Route Guards (React Router Example)

```jsx
import { Navigate } from 'react-router-dom';
import { hasPermission } from './utils/permissions';

const ProtectedRoute = ({ children, requiredPermission }) => {
  const { permissions } = useContext(BusinessContext);

  if (!hasPermission(permissions, requiredPermission)) {
    return <Navigate to="/unauthorized" />;
  }

  return children;
};

// Usage in routes
<Routes>
  <Route path="/water/reports" element={
    <ProtectedRoute requiredPermission="water:read_consumption">
      <WaterReportsPage />
    </ProtectedRoute>
  } />

  <Route path="/water/reports/create" element={
    <ProtectedRoute requiredPermission="inventory:create">
      <CreateWaterReport />
    </ProtectedRoute>
  } />
</Routes>
```

## Permission Matrix

| Role | Read Reports | Create Reports | Update Reports | Delete Reports |
|------|--------------|----------------|----------------|----------------|
| **Supervisor** | ✅ (water:read_consumption) | ❌ | ❌ | ❌ |
| **Engineer** | ✅ (water:read_consumption) | ✅ (inventory:create) | ✅ (inventory:update) | ❌ |
| **Water_Admin** | ✅ (all permissions) | ✅ (all permissions) | ✅ (all permissions) | ✅ (inventory:delete) |
| **Super Admin** | ✅ (admin_all) | ✅ (admin_all) | ✅ (admin_all) | ✅ (admin_all) |

## Testing Permissions

### 1. Backend Testing (using curl)

```bash
# As Supervisor - Should succeed (read)
curl -X GET "http://localhost:8080/api/v1/business/WATER/reports/tanker" \
  -H "Authorization: Bearer $SUPERVISOR_TOKEN" \
  -H "x-api-key: $API_KEY"

# As Supervisor - Should fail (create)
curl -X POST "http://localhost:8080/api/v1/business/WATER/reports/tanker" \
  -H "Authorization: Bearer $SUPERVISOR_TOKEN" \
  -H "x-api-key: $API_KEY" \
  -d '{"volume": 1000, "location": "Site A"}'

# As Engineer - Should succeed (create)
curl -X POST "http://localhost:8080/api/v1/business/WATER/reports/tanker" \
  -H "Authorization: Bearer $ENGINEER_TOKEN" \
  -H "x-api-key: $API_KEY" \
  -d '{"volume": 1000, "location": "Site A"}'

# As Engineer - Should fail (delete)
curl -X DELETE "http://localhost:8080/api/v1/business/WATER/reports/tanker/123" \
  -H "Authorization: Bearer $ENGINEER_TOKEN" \
  -H "x-api-key: $API_KEY"

# As Water_Admin - Should succeed (delete)
curl -X DELETE "http://localhost:8080/api/v1/business/WATER/reports/tanker/123" \
  -H "Authorization: Bearer $WATER_ADMIN_TOKEN" \
  -H "x-api-key: $API_KEY"
```

## Security Best Practices

1. **Always enforce on backend** - Frontend checks are for UX only
2. **Use business context** - Filter data by business vertical ID
3. **Log permission checks** - Track who accessed what
4. **Validate resource ownership** - Ensure users can only access their business's data
5. **Use HTTPS** - Protect tokens in transit
6. **Implement rate limiting** - Prevent abuse
7. **Regular permission audits** - Review who has what access

## Next Steps

1. Update your routes with permission middleware
2. Test each endpoint with different roles
3. Implement frontend permission checks
4. Add audit logging for sensitive operations
5. Document which roles can perform which actions

# Gorilla Mux Conversion Status

## ✅ Completed Files

### 1. handlers/project_management.go
- ✅ All methods converted to use `http.ResponseWriter` and `*http.Request`
- ✅ Uses `middleware.GetClaims(r)` for user context
- ✅ Uses `mux.Vars(r)` for URL parameters
- ✅ Uses `r.URL.Query().Get()` for query parameters
- ✅ All JSON encoding/decoding updated

### 2. handlers/task_management.go
- ✅ All methods converted to use `http.ResponseWriter` and `*http.Request`
- ✅ Uses `middleware.GetClaims(r)` and `middleware.GetUser(r)` for user context
- ✅ All error responses use `http.Error()`
- ✅ All JSON responses use `json.NewEncoder(w).Encode()`

### 3. handlers/kmz_parser.go
- ✅ Already correct - doesn't use web framework
- ✅ Pure Go structs and methods

### 4. models/project.go
- ✅ Already correct - just model definitions
- ✅ No framework dependencies

### 5. routes/project_routes.go
- ✅ Properly uses Gorilla Mux router
- ✅ All routes wrapped with `middleware.RequirePermission()`
- ✅ Uses `http.HandlerFunc()` wrapper

## ⚠️  Remaining Files (Need Conversion)

### 1. handlers/budget_management.go
**Status**: Needs conversion from Gin to Gorilla Mux

**Quick conversion needed**:
- Change `c *gin.Context` → `w http.ResponseWriter, r *http.Request`
- Change `c.ShouldBindJSON(&req)` → `json.NewDecoder(r.Body).Decode(&req)`
- Change `c.Param("id")` → `mux.Vars(r)["id"]`
- Change `c.Query("...")` → `r.URL.Query().Get("...")`
- Change `c.Get("user_id")` → `middleware.GetClaims(r).UserID`
- Change `c.JSON(status, data)` → `w.Header().Set(...); json.NewEncoder(w).Encode(data)`

### 2. handlers/project_workflow.go
**Status**: Needs conversion from Gin to Gorilla Mux

**Same conversion pattern as above**

### 3. handlers/project_roles.go
**Status**: Needs conversion from Gin to Gorilla Mux

**Same conversion pattern as above**

## How to Complete Remaining Conversions

For each file, run this mental checklist:

1. **Imports**: Replace `"github.com/gin-gonic/gin"` with `"github.com/gorilla/mux"`
2. **Function signatures**: `(c *gin.Context)` → `(w http.ResponseWriter, r *http.Request)`
3. **JSON decoding**: `c.ShouldBindJSON(&req)` → `json.NewDecoder(r.Body).Decode(&req)`
4. **URL params**: `c.Param("id")` → `mux.Vars(r)["id"]`
5. **Query params**: `c.Query("key")` → `r.URL.Query().Get("key")`
6. **Get user**: `c.Get("user_id")` → `middleware.GetClaims(r).UserID`
7. **JSON response**: `c.JSON(200, data)` → `w.Header().Set("Content-Type", "application/json"); json.NewEncoder(w).Encode(data)`
8. **Error response**: `c.JSON(400, gin.H{"error": "msg"})` → `http.Error(w, "msg", 400)`

## Integration Steps

Once all files are converted:

1. **Add to main routes** (in `routes/routes_v2.go` or similar):
   ```go
   // In RegisterRoutesV2() or your main route function
   routes.RegisterProjectRoutes(r) // where r is *mux.Router
   ```

2. **Run the migration**:
   ```bash
   psql -U postgres -d ugcl -f migrations/010_create_project_management_tables.sql
   ```

3. **Install Go dependencies**:
   ```bash
   go get github.com/paulmach/orb
   go get github.com/paulmach/orb/geojson
   go mod tidy
   ```

4. **Build and test**:
   ```bash
   go build
   ./ugcl_backend
   ```

## Quick Reference

### Before (Gin):
```go
func (h *Handler) Method(c *gin.Context) {
    var req Request
    c.ShouldBindJSON(&req)
    id := c.Param("id")
    userID, _ := c.Get("user_id")
    c.JSON(200, gin.H{"data": result})
}
```

### After (Gorilla Mux):
```go
func (h *Handler) Method(w http.ResponseWriter, r *http.Request) {
    var req Request
    json.NewDecoder(r.Body).Decode(&req)
    vars := mux.Vars(r)
    id := vars["id"]
    claims := middleware.GetClaims(r)
    userID := claims.UserID
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{"data": result})
}
```

## Next Steps

Would you like me to:
1. ✅ Convert the remaining 3 files (budget_management, project_workflow, project_roles)?
2. ⏭️  Leave them as-is and you can convert manually using the guide?
3. 📝 Create stub files with correct signatures that you can fill in later?

**Recommendation**: Option 1 - Let me complete the conversion now so everything works together.

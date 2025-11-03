# Report Builder - Gorilla Mux Backend Setup

## ✅ Completed Conversion

All report builder handlers have been **converted from Gin to Gorilla Mux** to match your application's architecture.

## 📁 Files Created/Updated

### Backend Handlers (Mux)
1. ✅ **`handlers/report_handlers.go`** - All CRUD operations for reports (Mux)
2. ✅ **`handlers/report_export.go`** - Export functionality (Mux)
3. ✅ **`handlers/report_engine.go`** - Query engine (framework-agnostic)
4. ✅ **`handlers/report_scheduler.go`** - Scheduling system (framework-agnostic)
5. ✅ **`routes/report_routes.go`** - All route definitions (Mux)
6. ✅ **`models/report_builder.go`** - Data models
7. ✅ **`utils/analytics.go`** - Analytics utilities

### Routes Integration
✅ **`routes/routes_v2.go`** - Updated to include `RegisterReportRoutes(r)`

## 🔄 Key Differences from Gin

### Handler Signature
```go
// Gin (OLD - removed)
func Handler(c *gin.Context) {
    reportID := c.Param("id")
    userID, _ := c.Get("user_id")
    c.JSON(http.StatusOK, gin.H{"data": data})
}

// Mux (NEW - implemented)
func Handler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    reportID := vars["id"]
    userID := r.Context().Value("userID").(uuid.UUID)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}
```

### Error Handling
```go
// Gin
c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})

// Mux
http.Error(w, "Not found", http.StatusNotFound)
```

### Route Parameters
```go
// Gin
reportID := c.Param("id")

// Mux
vars := mux.Vars(r)
reportID := vars["id"]
```

### Query Parameters
```go
// Gin
param := c.Query("param_name")

// Mux
param := r.URL.Query().Get("param_name")
```

### JSON Binding
```go
// Gin
var req RequestType
c.ShouldBindJSON(&req)

// Mux
var req RequestType
json.NewDecoder(r.Body).Decode(&req)
```

## 🚀 Quick Start

### 1. Install Dependencies
```bash
cd d:\Maheshwari\UGCL\backend\v1
go get github.com/xuri/excelize/v2
go mod tidy
```

### 2. Run Migrations
Migrations are already added to `config/migrations.go`. Just run:
```bash
go run main.go
```

This will create all report builder tables automatically.

### 3. Test the API

**Get available tables:**
```bash
curl http://localhost:8080/api/v1/reports/forms/tables \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Create a report:**
```bash
curl -X POST http://localhost:8080/api/v1/reports/definitions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "code": "test_report",
    "name": "Test Report",
    "report_type": "table",
    "business_vertical_id": "YOUR_VERTICAL_ID",
    "data_sources": [{
      "alias": "data",
      "table_name": "form_site_inspection",
      "form_code": "SITE_INSPECTION",
      "form_id": "uuid"
    }],
    "fields": [{
      "field_name": "site_name",
      "alias": "Site Name",
      "data_source": "data",
      "data_type": "text",
      "is_visible": true,
      "order": 1
    }]
  }'
```

**Execute report:**
```bash
curl -X POST http://localhost:8080/api/v1/reports/definitions/REPORT_ID/execute \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Export to Excel:**
```bash
curl http://localhost:8080/api/v1/reports/definitions/REPORT_ID/export/excel \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o report.xlsx
```

## 📋 All API Endpoints (Mux)

### Report Definitions
```
POST   /api/v1/reports/definitions              - Create report
GET    /api/v1/reports/definitions              - List reports
GET    /api/v1/reports/definitions/{id}         - Get single report
PUT    /api/v1/reports/definitions/{id}         - Update report
DELETE /api/v1/reports/definitions/{id}         - Delete report
POST   /api/v1/reports/definitions/{id}/clone   - Clone report
POST   /api/v1/reports/definitions/{id}/favorite - Toggle favorite
```

### Report Execution
```
POST   /api/v1/reports/definitions/{id}/execute - Execute report
GET    /api/v1/reports/definitions/{id}/history - Execution history
```

### Report Export
```
GET    /api/v1/reports/definitions/{id}/export/excel - Export Excel
GET    /api/v1/reports/definitions/{id}/export/csv   - Export CSV
GET    /api/v1/reports/definitions/{id}/export/pdf   - Export PDF
```

### Form Table Discovery
```
GET    /api/v1/reports/forms/tables                    - List all form tables
GET    /api/v1/reports/forms/tables/{table_name}/fields - Get table schema
```

### Dashboards
```
POST   /api/v1/dashboards                          - Create dashboard
GET    /api/v1/dashboards                          - List dashboards
GET    /api/v1/dashboards/{id}                     - Get dashboard
POST   /api/v1/dashboards/{id}/widgets             - Add widget
DELETE /api/v1/dashboards/{id}/widgets/{widget_id} - Remove widget
```

### Report Templates
```
GET    /api/v1/report-templates                       - List templates
POST   /api/v1/report-templates/{template_id}/create - Create from template
```

### Scheduled Reports
```
GET    /api/v1/scheduled-reports                  - List scheduled
POST   /api/v1/scheduled-reports/{id}/schedule    - Schedule report
POST   /api/v1/scheduled-reports/{id}/unschedule  - Unschedule report
POST   /api/v1/scheduled-reports/{id}/execute-now - Execute now
```

## 🔐 Authentication & Context

All handlers expect:
```go
// JWT Middleware sets userID in context
userID := r.Context().Value("userID").(uuid.UUID)

// Token in header
Authorization: Bearer <jwt-token>
```

This matches your existing middleware pattern in `routes/routes_v2.go`.

## 🎯 Testing Checklist

- [ ] Run `go run main.go` - Should compile without errors
- [ ] Check migrations run successfully
- [ ] Test GET `/api/v1/reports/forms/tables`
- [ ] Test POST create report
- [ ] Test POST execute report
- [ ] Test GET export to Excel
- [ ] Test frontend integration

## 🐛 Common Issues & Solutions

### Issue: "cannot use r (variable of type *mux.Router)"
**Solution**: Old Gin files removed. Make sure you have:
- `handlers/report_handlers.go` (Mux version)
- `handlers/report_export.go` (Mux version)
- `routes/report_routes.go` (Mux version)

### Issue: "userID type assertion failed"
**Solution**: Ensure JWT middleware sets userID as uuid.UUID:
```go
ctx := context.WithValue(r.Context(), "userID", userUUID)
```

### Issue: "table doesn't exist"
**Solution**:
1. Check migrations ran: `psql -U postgres -d ugcl -c "\dt report_*"`
2. Run migrations again if needed

### Issue: "Export returns empty file"
**Solution**:
```bash
go get github.com/xuri/excelize/v2
go mod tidy
```

## 📊 Database Schema

Tables created by migrations:
```sql
report_definitions
report_executions
report_widgets
dashboards
report_templates
report_shares
```

All tables include:
- UUID primary keys
- Soft delete support
- Audit fields (created_by, updated_by)
- Business vertical filtering

## 🎨 Frontend Integration

Your frontend screens at `D:\Maheshwari\UGCL\web\v1\src\routes\analytics\` are already configured to work with these Mux endpoints.

Just ensure your API base URL matches:
```typescript
const API_BASE = 'http://localhost:8080'; // or your backend URL
```

## 📚 Related Documentation

- **Backend API Guide**: `backend/v1/docs/REPORT_BUILDER_GUIDE.md`
- **Frontend Screens**: `web/v1/docs/REPORT_BUILDER_SCREENS.md`
- **Installation Guide**: `backend/v1/docs/REPORT_BUILDER_INSTALLATION.md`
- **Quick Start**: `backend/v1/docs/REPORT_BUILDER_QUICK_START.md`

## ✅ Summary

Your report builder is now **fully compatible with Gorilla Mux**! All handlers follow your existing patterns:

- ✅ Uses `*mux.Router` instead of `*gin.Engine`
- ✅ Uses `http.ResponseWriter` and `*http.Request`
- ✅ Uses `mux.Vars(r)` for path parameters
- ✅ Uses context values for user authentication
- ✅ Follows your middleware patterns
- ✅ Integrated into `routes/routes_v2.go`

**You're ready to use the report builder!** 🚀📊

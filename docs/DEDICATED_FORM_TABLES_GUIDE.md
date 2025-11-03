# Dedicated Form Tables System

## Overview

Instead of storing all form submission data in a generic `form_submissions` JSONB table, this system creates **dedicated tables** for each form type with properly typed columns based on the form schema.

## Benefits

1. **Better Performance**: Proper indexing and typed columns
2. **Easier Querying**: Direct SQL queries on specific fields
3. **Data Integrity**: Database-level validation and constraints
4. **Simplified Reporting**: Standard SQL joins and aggregations
5. **Type Safety**: Proper data types (integer, decimal, date, etc.)

## How It Works

### 1. Form Configuration

Each `AppForm` has a `table_name` field (already exists in your model at `models/app_form.go:99`):

```go
type AppForm struct {
    // ... other fields
    DBTableName   string `gorm:"size:255" json:"table_name,omitempty"`
    // ... other fields
}
```

**Example**: If you have a "Water Tanker Report" form with code `water_tanker_report`, set its `table_name` to `water_tanker_reports`.

### 2. Automatic Table Creation

When you set a `table_name` for a form, the system can automatically create a dedicated table with:

**Standard Fields (all tables have these)**:
- `id` (UUID, primary key)
- `created_by` (VARCHAR)
- `created_at` (TIMESTAMP)
- `updated_by` (VARCHAR)
- `updated_at` (TIMESTAMP)
- `deleted_by` (VARCHAR)
- `deleted_at` (TIMESTAMP) - for soft deletes
- `business_vertical_id` (UUID, FK)
- `site_id` (UUID, FK, optional)
- `workflow_id` (UUID, FK, optional)
- `current_state` (VARCHAR) - workflow state
- `form_id` (UUID, FK to app_forms)
- `form_code` (VARCHAR)

**Custom Fields**: Based on your form schema (`form_schema` field in `app_forms`)

### 3. Form Schema to Table Mapping

The system reads your `form_schema` JSON and creates appropriate columns:

| Form Field Type | Database Column Type |
|----------------|---------------------|
| text, email, url | VARCHAR or TEXT |
| number, integer | INTEGER |
| decimal, currency | DECIMAL(15,2) |
| date | DATE |
| datetime, timestamp | TIMESTAMP |
| boolean, checkbox | BOOLEAN |
| select, radio | VARCHAR(255) |
| multiselect, checkbox_group | JSONB |
| file, image | VARCHAR(500) - stores path |
| json, object | JSONB |

## API Endpoints

### Admin Endpoints (Create/Manage Tables)

#### 1. Create Table for a Form
```http
POST /api/v1/admin/forms/{formCode}/create-table
Authorization: Bearer <admin-token>
```

**Example**:
```bash
curl -X POST http://localhost:8080/api/v1/admin/forms/water_tanker_report/create-table \
  -H "Authorization: Bearer YOUR_TOKEN"
```

#### 2. Check Table Status
```http
GET /api/v1/admin/forms/{formCode}/table-status
Authorization: Bearer <admin-token>
```

**Response**:
```json
{
  "form_code": "water_tanker_report",
  "table_name": "water_tanker_reports",
  "has_table_name": true,
  "table_exists": true,
  "using_dedicated": true
}
```

#### 3. Bulk Create All Tables
```http
POST /api/v1/admin/forms/create-all-tables
Authorization: Bearer <admin-token>
```

Creates tables for all forms that have `table_name` configured.

#### 4. Drop Table (DANGEROUS!)
```http
DELETE /api/v1/admin/forms/{formCode}/table?confirm=true
Authorization: Bearer <admin-token>
```

### Form Submission Endpoints (Dedicated Tables)

All these endpoints work with dedicated tables. URLs have `/dedicated` in the path:

#### 1. Submit Form (Create Submission)
```http
POST /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated
Authorization: Bearer <token>
Content-Type: application/json

{
  "form_data": {
    "tanker_number": "TN-001",
    "driver_name": "John Doe",
    "capacity_liters": 5000,
    "delivery_date": "2025-01-15"
  },
  "site_id": "optional-site-uuid"
}
```

#### 2. Get All Submissions
```http
GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated
GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated?state=approved
GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated?my_submissions=true
```

#### 3. Get Single Submission
```http
GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}
```

#### 4. Update Submission (Draft Only)
```http
PUT /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}
Content-Type: application/json

{
  "form_data": {
    "tanker_number": "TN-002",
    "notes": "Updated information"
  }
}
```

#### 5. Workflow Transition
```http
POST /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}/transition
Content-Type: application/json

{
  "action": "submit",
  "comment": "Ready for approval",
  "metadata": {
    "approved_budget": 5000
  }
}
```

#### 6. Delete Submission (Soft Delete)
```http
DELETE /api/v1/business/{businessCode}/forms/{formCode}/submissions/dedicated/{submissionId}
```

## Frontend Integration

### Option 1: No Changes Required (Transparent)

If you want the frontend to work without changes, you can:

1. Keep the old endpoints (`/submissions` without `/dedicated`)
2. Modify the old handlers to automatically detect if a form has a dedicated table
3. Route to dedicated or JSONB storage based on form configuration

**Example modification to existing handler**:
```go
func CreateFormSubmission(w http.ResponseWriter, r *http.Request) {
    // Get form
    var form models.AppForm
    // ... fetch form ...

    // Check if form has dedicated table
    if form.DBTableName != "" {
        // Use dedicated table engine
        getWorkflowEngineDedicated().CreateSubmissionDedicated(...)
    } else {
        // Use old JSONB engine
        getWorkflowEngine().CreateSubmission(...)
    }
}
```

### Option 2: Update Frontend (Recommended)

**Minimal Changes**: Just update the API endpoint URLs to include `/dedicated`:

**Before**:
```javascript
// Old endpoint
POST /api/v1/business/solar/forms/maintenance_form/submissions
```

**After**:
```javascript
// New endpoint
POST /api/v1/business/solar/forms/maintenance_form/submissions/dedicated
```

**The request and response formats remain the same!**

### Response Format Comparison

Both systems return similar structures:

**JSONB Storage Response**:
```json
{
  "submission": {
    "id": "uuid",
    "form_code": "water_tanker_report",
    "current_state": "draft",
    "form_data": {  // All fields in JSON
      "tanker_number": "TN-001",
      "driver_name": "John Doe"
    },
    "created_by": "user-id",
    "created_at": "2025-01-15T10:00:00Z"
  }
}
```

**Dedicated Table Response**:
```json
{
  "submission": {
    "id": "uuid",
    "form_code": "water_tanker_report",
    "current_state": "draft",
    "form_data": {  // Custom fields as separate columns
      "tanker_number": "TN-001",
      "driver_name": "John Doe"
    },
    "created_by": "user-id",
    "created_at": "2025-01-15T10:00:00Z"
  }
}
```

## Migration Strategy

### For Existing Forms

1. **Update Form Configuration**:
```sql
UPDATE app_forms
SET table_name = 'water_tanker_reports'
WHERE code = 'water_tanker_report';
```

2. **Create Dedicated Table**:
```bash
curl -X POST http://localhost:8080/api/v1/admin/forms/water_tanker_report/create-table \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN"
```

3. **Migrate Existing Data** (Optional):
```go
// Migration script example
func MigrateFormSubmissionsToTable(formCode string) error {
    // 1. Get all submissions from form_submissions table
    var submissions []models.FormSubmission
    db.Where("form_code = ?", formCode).Find(&submissions)

    // 2. Get form to find table name
    var form models.AppForm
    db.Where("code = ?", formCode).First(&form)

    tableManager := NewFormTableManager()

    // 3. Insert each into dedicated table
    for _, sub := range submissions {
        var formData map[string]interface{}
        json.Unmarshal(sub.FormData, &formData)

        tableManager.InsertFormData(
            form.DBTableName,
            form.ID,
            formCode,
            sub.BusinessVerticalID,
            sub.SiteID,
            sub.WorkflowID,
            sub.CurrentState,
            formData,
            sub.SubmittedBy,
        )
    }

    return nil
}
```

## Code Files Created

1. **[handlers/form_table_manager.go](handlers/form_table_manager.go)** - Core table management
   - CreateFormTable() - Creates dedicated table
   - InsertFormData() - Insert submission
   - UpdateFormData() - Update submission
   - GetFormData() - Retrieve submission
   - SoftDeleteFormData() - Delete submission

2. **[handlers/workflow_engine_dedicated.go](handlers/workflow_engine_dedicated.go)** - Workflow engine for dedicated tables
   - CreateSubmissionDedicated()
   - TransitionStateDedicated()
   - UpdateSubmissionDataDedicated()
   - GetSubmissionDedicated()
   - GetSubmissionsByFormDedicated()

3. **[handlers/form_table_admin_handlers.go](handlers/form_table_admin_handlers.go)** - Admin endpoints
   - CreateFormTableHandler()
   - CheckFormTableStatus()
   - DropFormTableHandler()
   - BulkCreateFormTablesHandler()

4. **[handlers/workflow_handlers_dedicated.go](handlers/workflow_handlers_dedicated.go)** - Form submission handlers
   - CreateFormSubmissionDedicated()
   - GetFormSubmissionsDedicated()
   - UpdateFormSubmissionDedicated()
   - TransitionFormSubmissionDedicated()
   - DeleteFormSubmissionDedicated()

## Example: Water Tanker Report Form

### 1. Form Schema
```json
{
  "fields": [
    {
      "name": "tanker_number",
      "type": "text",
      "required": true,
      "max_length": 50
    },
    {
      "name": "driver_name",
      "type": "text",
      "required": true
    },
    {
      "name": "capacity_liters",
      "type": "integer",
      "required": true
    },
    {
      "name": "delivery_date",
      "type": "date",
      "required": true
    },
    {
      "name": "cost",
      "type": "decimal",
      "required": false
    }
  ]
}
```

### 2. Generated Table Structure
```sql
CREATE TABLE water_tanker_reports (
  -- Base fields
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  created_by VARCHAR(255) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_by VARCHAR(255),
  updated_at TIMESTAMP DEFAULT NOW(),
  deleted_by VARCHAR(255),
  deleted_at TIMESTAMP,
  business_vertical_id UUID NOT NULL REFERENCES business_verticals(id),
  site_id UUID REFERENCES sites(id),
  workflow_id UUID REFERENCES workflow_definitions(id),
  current_state VARCHAR(50) NOT NULL DEFAULT 'draft',
  form_id UUID NOT NULL REFERENCES app_forms(id),
  form_code VARCHAR(50) NOT NULL,

  -- Custom fields from form schema
  tanker_number VARCHAR(50) NOT NULL,
  driver_name TEXT NOT NULL,
  capacity_liters INTEGER NOT NULL,
  delivery_date DATE NOT NULL,
  cost DECIMAL(15,2)
);

-- Indexes
CREATE INDEX idx_water_tanker_reports_business_vertical ON water_tanker_reports(business_vertical_id);
CREATE INDEX idx_water_tanker_reports_site ON water_tanker_reports(site_id);
CREATE INDEX idx_water_tanker_reports_state ON water_tanker_reports(current_state);
CREATE INDEX idx_water_tanker_reports_deleted ON water_tanker_reports(deleted_at);
```

## Querying Dedicated Tables

### Direct SQL Queries (For Reports)

```sql
-- Get all approved water tanker reports for a site
SELECT
  tanker_number,
  driver_name,
  capacity_liters,
  delivery_date,
  cost,
  created_at
FROM water_tanker_reports
WHERE site_id = 'your-site-uuid'
  AND current_state = 'approved'
  AND deleted_at IS NULL
ORDER BY delivery_date DESC;

-- Monthly summary
SELECT
  DATE_TRUNC('month', delivery_date) as month,
  COUNT(*) as total_deliveries,
  SUM(capacity_liters) as total_liters,
  SUM(cost) as total_cost
FROM water_tanker_reports
WHERE deleted_at IS NULL
GROUP BY DATE_TRUNC('month', delivery_date)
ORDER BY month DESC;
```

## Workflow Integration

Workflow state transitions work the same way:
- Transitions are still stored in `workflow_transitions` table
- Each transition references the record ID from the dedicated table
- State is updated in both the dedicated table and transition history

## Summary

✅ **Backend Complete**: All handlers and utilities created
✅ **Backward Compatible**: Old JSONB system still works
✅ **Frontend Changes**: Minimal - just add `/dedicated` to URLs
✅ **Migration Path**: Clear strategy for moving existing data
✅ **Better Performance**: Proper indexing and typed columns
✅ **Easier Reporting**: Direct SQL queries possible

## Next Steps

1. **Update routes in [routes/business_routes.go](../routes/business_routes.go)** to include new endpoints
2. **Test with a sample form** to verify table creation
3. **Update frontend** to use new endpoints (or make old endpoints auto-detect)
4. **Migrate existing forms** one by one as needed

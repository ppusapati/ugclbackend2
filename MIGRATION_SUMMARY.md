# Form System Migration & Workflow Implementation - Summary

## ✅ Completed Tasks

### 1. Form Model Migration (Form → AppForm)

**Status:** ✅ Complete

**Changes:**
- ❌ **Deleted:** `models/form.go` (old Form model)
- ❌ **Deleted:** `handlers/forms.go` (old form handlers)
- ✅ **Created:** `config/migrate_form_to_appform.go` (migration script)

**Migration Script Features:**
- Automatically converts old Form records to AppForm
- Migrates business vertical associations from join table to JSONB array
- Creates modules if they don't exist
- Safe to run (checks for existing data)

### 2. Workflow System Implementation

**Status:** ✅ Complete

**New Files Created:**

#### Models
- `models/workflow.go` - Complete workflow system models
  - `WorkflowDefinition` - Reusable workflow configurations
  - `FormSubmission` - Form instances with workflow state
  - `WorkflowTransition` - Audit trail for state changes
  - Helper methods for workflow operations

#### Handlers
- `handlers/workflow_engine.go` - Core workflow engine
  - `CreateSubmission()` - Create form submissions
  - `TransitionState()` - Perform state transitions
  - `UpdateSubmissionData()` - Update draft submissions
  - `GetSubmission()` - Retrieve submission with history
  - `GetSubmissionsByForm()` - List submissions with filters
  - `ValidateTransition()` - Permission-based validation
  - `GetWorkflowStats()` - Analytics

- `handlers/workflow_handlers.go` - HTTP handlers
  - `CreateFormSubmission` - POST /forms/{code}/submissions
  - `GetFormSubmissions` - GET /forms/{code}/submissions
  - `GetFormSubmission` - GET /forms/{code}/submissions/{id}
  - `UpdateFormSubmission` - PUT /forms/{code}/submissions/{id}
  - `TransitionFormSubmission` - POST /forms/{code}/submissions/{id}/transition
  - `GetWorkflowHistory` - GET /forms/{code}/submissions/{id}/history
  - `GetWorkflowStats` - GET /forms/{code}/stats
  - `CreateWorkflowDefinition` - Admin endpoint
  - `GetAllWorkflows` - Admin endpoint

#### Database Migrations
- `migrations/000012_create_workflow_tables.up.sql`
  - Creates `workflow_definitions` table
  - Creates `form_submissions` table
  - Creates `workflow_transitions` table
  - Includes sample "standard_approval" workflow
  - Comprehensive indexes for performance

- `migrations/000012_create_workflow_tables.down.sql` - Rollback migration

- `migrations/000013_drop_old_form_tables.up.sql` - Drop old Form tables

- `migrations/000013_drop_old_form_tables.down.sql` - Rollback (recreate empty tables)

#### Routes
- **Modified:** `routes/business_routes.go`
  - Added 7 workflow submission endpoints
  - Added 2 admin workflow management endpoints

#### Documentation
- `docs/WORKFLOW_SYSTEM_IMPLEMENTATION.md` - Complete implementation guide

## 🚀 How to Deploy

### Step 1: Run Database Migrations

```bash
# Run migrations in order
migrate -path ./migrations -database "your_db_url" up
```

This will:
1. Create workflow tables (migration 000012)
2. Insert sample "standard_approval" workflow
3. Drop old Form tables (migration 000013)

### Step 2: Migrate Existing Data (If Needed)

If you have existing Form data that needs migration:

```go
// In your initialization code or admin endpoint
config.MigrateFormToAppForm()
```

**Note:** Only needed if old Form tables still contain data.

### Step 3: Link Forms to Workflows

```sql
-- Example: Link water form to standard approval workflow
UPDATE app_forms
SET workflow_id = (SELECT id FROM workflow_definitions WHERE code = 'standard_approval')
WHERE code = 'water';
```

Or via admin API:
```bash
curl -X POST http://localhost:8080/api/v1/admin/app-forms \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "code": "water",
    "workflow_id": "uuid-of-standard-approval"
  }'
```

### Step 4: Test the System

```bash
# 1. Create a submission
curl -X POST http://localhost:8080/api/v1/business/ugcl/forms/water/submissions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "form_data": {
      "tanker_number": "TK-001",
      "quantity": 5000,
      "date": "2024-01-15"
    }
  }'

# 2. Submit for approval
curl -X POST http://localhost:8080/api/v1/business/ugcl/forms/water/submissions/{id}/transition \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"action": "submit"}'

# 3. Approve (requires project:approve permission)
curl -X POST http://localhost:8080/api/v1/business/ugcl/forms/water/submissions/{id}/transition \
  -H "Authorization: Bearer $APPROVER_TOKEN" \
  -d '{"action": "approve", "comment": "Looks good!"}'
```

## 📊 Database Schema

### workflow_definitions
```
id                UUID PRIMARY KEY
code              VARCHAR(50) UNIQUE
name              VARCHAR(100)
initial_state     VARCHAR(50)
states            JSONB  -- Array of state definitions
transitions       JSONB  -- Array of transition rules
is_active         BOOLEAN
```

### form_submissions
```
id                    UUID PRIMARY KEY
form_code             VARCHAR(50)
form_id               UUID → app_forms
business_vertical_id  UUID → business_verticals
site_id               UUID → sites (optional)
workflow_id           UUID → workflow_definitions
current_state         VARCHAR(50)
form_data             JSONB
submitted_by          VARCHAR(255)
submitted_at          TIMESTAMP
```

### workflow_transitions
```
id               UUID PRIMARY KEY
submission_id    UUID → form_submissions
from_state       VARCHAR(50)
to_state         VARCHAR(50)
action           VARCHAR(50)
actor_id         VARCHAR(255)
actor_name       VARCHAR(255)
comment          TEXT
metadata         JSONB
transitioned_at  TIMESTAMP
```

## 🎯 API Endpoints

### User Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/business/{code}/forms/{formCode}/submissions` | Create submission |
| GET | `/api/v1/business/{code}/forms/{formCode}/submissions` | List submissions |
| GET | `/api/v1/business/{code}/forms/{formCode}/submissions/{id}` | Get submission |
| PUT | `/api/v1/business/{code}/forms/{formCode}/submissions/{id}` | Update draft |
| POST | `/api/v1/business/{code}/forms/{formCode}/submissions/{id}/transition` | Change state |
| GET | `/api/v1/business/{code}/forms/{formCode}/submissions/{id}/history` | View history |
| GET | `/api/v1/business/{code}/forms/{formCode}/stats` | Get statistics |

### Admin Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/admin/workflows` | List all workflows |
| POST | `/api/v1/admin/workflows` | Create workflow |

## 🔐 Permissions

Workflow transitions respect permission requirements:

```json
{
  "from": "submitted",
  "to": "approved",
  "action": "approve",
  "permission": "project:approve"
}
```

Users without `project:approve` permission cannot approve submissions.

## 📈 Features

### ✅ Implemented

- [x] Form submission creation
- [x] Draft submissions (editable)
- [x] State transitions with validation
- [x] Permission-based transition control
- [x] Required comments for specific actions
- [x] Complete audit trail
- [x] Workflow history tracking
- [x] Submission filtering (by state, site, user)
- [x] Statistics and analytics
- [x] Multiple workflow definitions
- [x] JSONB-based flexible form data
- [x] Business vertical scoping

### 🔮 Future Enhancements

- [ ] Email/SMS notifications on state changes
- [ ] SLA tracking and deadline management
- [ ] Multi-level approval chains
- [ ] Conditional transitions
- [ ] Workflow builder UI
- [ ] Bulk operations
- [ ] Export to PDF/Excel
- [ ] Advanced analytics dashboard
- [ ] Workflow templates

## 🐛 Troubleshooting

### Issue: Migration fails
**Solution:** Check if old `forms` table exists. If not, skip data migration.

### Issue: Cannot transition state
**Check:**
1. User has required permission
2. Transition is valid from current state
3. Comment is provided if required

### Issue: Submission not found
**Check:**
1. User has access to the business vertical
2. Submission exists and isn't soft-deleted
3. Correct business context in request

## 📚 Additional Documentation

- [Workflow System Implementation Guide](docs/WORKFLOW_SYSTEM_IMPLEMENTATION.md)
- [Form Definition Guide](form_definitions/water.json)
- [Backend Implementation Guide](docs/BACKEND_IMPLEMENTATION_GUIDE.md)

## ✨ Summary

### Files Created: 8
- `models/workflow.go`
- `handlers/workflow_engine.go`
- `handlers/workflow_handlers.go`
- `config/migrate_form_to_appform.go`
- `migrations/000012_create_workflow_tables.up.sql`
- `migrations/000012_create_workflow_tables.down.sql`
- `migrations/000013_drop_old_form_tables.up.sql`
- `migrations/000013_drop_old_form_tables.down.sql`
- `docs/WORKFLOW_SYSTEM_IMPLEMENTATION.md`

### Files Modified: 1
- `routes/business_routes.go`

### Files Deleted: 2
- `models/form.go`
- `handlers/forms.go`

### Build Status: ✅ PASSING

---

**Migration completed successfully! 🎉**

The system is now ready for workflow-based form submissions with approval processes.

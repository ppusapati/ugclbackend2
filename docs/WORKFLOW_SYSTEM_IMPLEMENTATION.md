# Workflow System Implementation Guide

## Overview

This document describes the complete workflow system implementation for dynamic forms with approval workflows.

## Migration: Form → AppForm

### What Changed?

We migrated from a dual-model system (`Form` + `AppForm`) to a unified `AppForm` system.

| Aspect | Old (`Form`) | New (`AppForm`) |
|--------|--------------|-----------------|
| **Purpose** | Simple navigation menu | Complete dynamic form system |
| **Schema** | Basic metadata only | Full JSON-based form definition |
| **Vertical Access** | Many-to-many join table | JSONB array |
| **Workflow** | Not supported | Full workflow support |
| **Flexibility** | Low | High |

### Migration Steps

1. **Run the migration script** (one-time operation):
   ```go
   config.MigrateFormToAppForm()
   ```

2. **Verify data migration**:
   - Check that all forms are in `app_forms` table
   - Verify `accessible_verticals` arrays are populated
   - Confirm modules are created

3. **Drop old tables** (after verification):
   ```go
   config.DropOldFormTables()
   ```
   Or run migration: `000013_drop_old_form_tables.up.sql`

## Workflow System Architecture

### Database Tables

#### 1. `workflow_definitions`
Stores reusable workflow configurations.

```sql
CREATE TABLE workflow_definitions (
    id UUID PRIMARY KEY,
    code VARCHAR(50) UNIQUE,
    name VARCHAR(100),
    initial_state VARCHAR(50),
    states JSONB,      -- Array of state definitions
    transitions JSONB, -- Array of transition rules
    is_active BOOLEAN
);
```

**Example workflow definition:**
```json
{
  "code": "standard_approval",
  "initial_state": "draft",
  "states": [
    {"code": "draft", "name": "Draft"},
    {"code": "submitted", "name": "Submitted"},
    {"code": "approved", "name": "Approved", "is_final": true},
    {"code": "rejected", "name": "Rejected", "is_final": true}
  ],
  "transitions": [
    {
      "from": "draft",
      "to": "submitted",
      "action": "submit",
      "permission": "project:create"
    },
    {
      "from": "submitted",
      "to": "approved",
      "action": "approve",
      "permission": "project:approve"
    }
  ]
}
```

#### 2. `form_submissions`
Stores submitted form instances with workflow state.

```sql
CREATE TABLE form_submissions (
    id UUID PRIMARY KEY,
    form_code VARCHAR(50),
    form_id UUID REFERENCES app_forms(id),
    business_vertical_id UUID,
    site_id UUID (optional),
    workflow_id UUID REFERENCES workflow_definitions(id),
    current_state VARCHAR(50),
    form_data JSONB,
    submitted_by VARCHAR(255),
    submitted_at TIMESTAMP
);
```

#### 3. `workflow_transitions`
Audit trail for all state transitions.

```sql
CREATE TABLE workflow_transitions (
    id UUID PRIMARY KEY,
    submission_id UUID REFERENCES form_submissions(id),
    from_state VARCHAR(50),
    to_state VARCHAR(50),
    action VARCHAR(50),
    actor_id VARCHAR(255),
    comment TEXT,
    transitioned_at TIMESTAMP
);
```

## API Endpoints

### Form Submission Endpoints

#### Create Form Submission
```http
POST /api/v1/business/{businessCode}/forms/{formCode}/submissions
```

**Request Body:**
```json
{
  "form_data": {
    "site": "site-uuid",
    "date": "2024-01-15",
    "tanker_number": "TK-001",
    "quantity": 5000
  },
  "site_id": "uuid-optional"
}
```

**Response:**
```json
{
  "message": "form submission created successfully",
  "submission": {
    "id": "uuid",
    "form_code": "water",
    "current_state": "draft",
    "available_actions": [
      {
        "action": "submit",
        "label": "Submit for Approval",
        "to_state": "submitted"
      }
    ]
  }
}
```

#### Get Submissions
```http
GET /api/v1/business/{businessCode}/forms/{formCode}/submissions?state=submitted&my_submissions=true
```

#### Get Single Submission
```http
GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}
```

#### Update Draft Submission
```http
PUT /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}
```

**Note:** Only submissions in `draft` state can be updated.

#### Transition Workflow State
```http
POST /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}/transition
```

**Request Body:**
```json
{
  "action": "approve",
  "comment": "Looks good, approved!",
  "metadata": {
    "reviewed_by": "John Doe",
    "additional_notes": "Fast tracked"
  }
}
```

**Response:**
```json
{
  "message": "transition successful",
  "current_state": "approved",
  "submission": {
    "id": "uuid",
    "current_state": "approved",
    "available_actions": []
  }
}
```

#### Get Workflow History
```http
GET /api/v1/business/{businessCode}/forms/{formCode}/submissions/{submissionId}/history
```

**Response:**
```json
{
  "history": [
    {
      "from_state": "draft",
      "to_state": "submitted",
      "action": "submit",
      "actor_name": "Jane Smith",
      "transitioned_at": "2024-01-15T10:30:00Z"
    },
    {
      "from_state": "submitted",
      "to_state": "approved",
      "action": "approve",
      "actor_name": "John Manager",
      "comment": "Approved",
      "transitioned_at": "2024-01-15T14:20:00Z"
    }
  ]
}
```

#### Get Workflow Stats
```http
GET /api/v1/business/{businessCode}/forms/{formCode}/stats
```

**Response:**
```json
{
  "form_code": "water",
  "stats": {
    "draft": 5,
    "submitted": 12,
    "approved": 45,
    "rejected": 3
  }
}
```

### Admin Endpoints

#### Create Workflow Definition
```http
POST /api/v1/admin/workflows
```

**Request Body:**
```json
{
  "code": "custom_workflow",
  "name": "Custom Approval Workflow",
  "initial_state": "draft",
  "states": [...],
  "transitions": [...]
}
```

#### Get All Workflows
```http
GET /api/v1/admin/workflows
```

## Usage Examples

### 1. Setting up a Form with Workflow

```go
// 1. Create or get workflow
var workflow models.WorkflowDefinition
db.Where("code = ?", "standard_approval").First(&workflow)

// 2. Link form to workflow
var form models.AppForm
db.Where("code = ?", "water").First(&form)
form.WorkflowID = &workflow.ID
db.Save(&form)
```

### 2. Submitting a Form

```javascript
// Frontend code example
const submitForm = async (formData) => {
  // Step 1: Create draft submission
  const response = await fetch(
    '/api/v1/business/ugcl/forms/water/submissions',
    {
      method: 'POST',
      body: JSON.stringify({ form_data: formData })
    }
  );

  const { submission } = await response.json();

  // Step 2: Immediately submit for approval
  await fetch(
    `/api/v1/business/ugcl/forms/water/submissions/${submission.id}/transition`,
    {
      method: 'POST',
      body: JSON.stringify({ action: 'submit' })
    }
  );
};
```

### 3. Approving/Rejecting Submissions

```javascript
// Approve
const approve = async (submissionId) => {
  await fetch(
    `/api/v1/business/ugcl/forms/water/submissions/${submissionId}/transition`,
    {
      method: 'POST',
      body: JSON.stringify({
        action: 'approve',
        comment: 'Verified and approved'
      })
    }
  );
};

// Reject
const reject = async (submissionId, reason) => {
  await fetch(
    `/api/v1/business/ugcl/forms/water/submissions/${submissionId}/transition`,
    {
      method: 'POST',
      body: JSON.stringify({
        action: 'reject',
        comment: reason
      })
    }
  );
};
```

## Workflow Engine Features

### Permission-Based Transitions

Each transition can require specific permissions:

```json
{
  "from": "submitted",
  "to": "approved",
  "action": "approve",
  "permission": "project:approve"
}
```

The engine automatically validates user permissions before allowing transitions.

### Required Comments

Transitions can require comments (useful for rejections):

```json
{
  "from": "submitted",
  "to": "rejected",
  "action": "reject",
  "requires_comment": true
}
```

### Audit Trail

Every state transition is recorded with:
- Actor information (ID, name, role)
- Timestamp
- Comment (if provided)
- Custom metadata

### Available Actions

The API automatically returns only valid actions for the current state:

```json
{
  "available_actions": [
    {
      "action": "approve",
      "label": "Approve",
      "to_state": "approved",
      "permission": "project:approve"
    },
    {
      "action": "reject",
      "label": "Reject",
      "to_state": "rejected",
      "requires_comment": true
    }
  ]
}
```

## Form Definition with Workflow

Example: [water.json](../form_definitions/water.json)

```json
{
  "form_code": "water",
  "title": "Water Supply Form",
  "workflow": {
    "initial_state": "draft",
    "states": ["draft", "submitted", "approved", "rejected"],
    "transitions": [
      {
        "from": "draft",
        "to": "submitted",
        "action": "submit",
        "permission": "project:create"
      },
      {
        "from": "submitted",
        "to": "approved",
        "action": "approve",
        "permission": "project:approve"
      }
    ]
  }
}
```

## Testing the System

### Manual Testing Steps

1. **Create a submission:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/business/ugcl/forms/water/submissions \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"form_data": {"tanker_number": "TK-001", "quantity": 5000}}'
   ```

2. **Submit for approval:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/business/ugcl/forms/water/submissions/$SUBMISSION_ID/transition \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"action": "submit"}'
   ```

3. **Approve:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/business/ugcl/forms/water/submissions/$SUBMISSION_ID/transition \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"action": "approve", "comment": "Approved"}'
   ```

4. **View history:**
   ```bash
   curl http://localhost:8080/api/v1/business/ugcl/forms/water/submissions/$SUBMISSION_ID/history \
     -H "Authorization: Bearer $TOKEN"
   ```

## Error Handling

Common errors:

- **403 Forbidden**: User lacks required permission for transition
- **400 Bad Request**: Invalid transition (action not allowed from current state)
- **400 Bad Request**: Comment required but not provided
- **404 Not Found**: Submission not found
- **400 Bad Request**: Cannot update submission (not in draft state)

## Next Steps

1. **Frontend Integration**: Build UI for form submissions and approvals
2. **Notifications**: Add email/SMS notifications for state changes
3. **Advanced Workflows**: Implement multi-level approvals
4. **Reporting**: Build dashboards for workflow analytics
5. **Deadline Tracking**: Add SLA tracking for pending approvals

## Files Created/Modified

### New Files:
- `models/workflow.go` - Workflow models
- `handlers/workflow_engine.go` - Workflow engine logic
- `handlers/workflow_handlers.go` - HTTP handlers
- `config/migrate_form_to_appform.go` - Migration script
- `migrations/000012_create_workflow_tables.up.sql` - Workflow tables
- `migrations/000013_drop_old_form_tables.up.sql` - Cleanup old tables

### Deleted Files:
- `models/form.go` - Old Form model
- `handlers/forms.go` - Old form handlers

### Modified Files:
- `routes/business_routes.go` - Added workflow routes

## Support

For issues or questions, refer to:
- [Form Definition Guide](./FORM_DEFINITION_GUIDE.md)
- [API Documentation](./API_DOCS.md)
- [Backend Implementation Guide](./BACKEND_IMPLEMENTATION_GUIDE.md)

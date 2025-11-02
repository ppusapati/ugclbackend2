# Notification System Guide

## Overview

The UGCL notification system is designed to integrate seamlessly with your existing RBAC + ABAC + PBAC + Workflow architecture. It provides flexible, multi-level notification targeting based on roles, permissions, attributes, policies, and dynamic criteria.

## Architecture

### Core Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Workflow Engine                          │
│  (Handles state transitions and triggers notifications)     │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│              Notification Service                           │
│  • Resolves recipients (RBAC/ABAC/PBAC)                    │
│  • Renders templates with context data                      │
│  • Creates notification instances                           │
│  • Respects user preferences                                │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│              Notification Delivery                          │
│  • In-App Notifications                                     │
│  • Email Notifications                                      │
│  • SMS Notifications                                        │
│  • Web Push Notifications                                   │
└─────────────────────────────────────────────────────────────┘
```

## Database Schema

### Tables

1. **`notification_rules`** - Configuration for when and how to send notifications
2. **`notification_recipients`** - Defines who receives notifications (multi-level targeting)
3. **`notifications`** - Actual notification instances sent to users
4. **`notification_preferences`** - User preferences for notification delivery

## Question 1: Who Should Receive Notifications?

### Multi-Level Targeting Support

The system supports **ALL** of these targeting strategies:

#### 1. **Specific User**
```json
{
  "type": "user",
  "value": "user_123"
}
```

#### 2. **By Role (RBAC)**
```json
{
  "type": "role",
  "role_id": "admin_role_uuid"
}
```

#### 3. **By Business Role (Business-Specific RBAC)**
```json
{
  "type": "business_role",
  "business_role_id": "solar_farm_manager_uuid"
}
```

#### 4. **By Permission**
```json
{
  "type": "permission",
  "permission_code": "project:approve"
}
```
All users with this permission receive the notification.

#### 5. **By Attributes (ABAC)**
```json
{
  "type": "attribute",
  "attribute_query": {
    "department": "field_operations",
    "location": "north_region"
  }
}
```
Dynamic targeting based on user attributes.

#### 6. **By Policy (PBAC)**
```json
{
  "type": "policy",
  "policy_id": "approval_policy_uuid"
}
```
Evaluate policy to determine recipients.

#### 7. **Dynamic Recipients**
```json
{
  "type": "submitter"  // The user who submitted the form
}

{
  "type": "approver"  // The user who last approved
}

{
  "type": "field_value",  // User referenced in a form field
  "value": "assigned_engineer_field"
}
```

### Example: Complex Multi-Target Notification

```json
{
  "recipients": [
    {
      "type": "submitter"
    },
    {
      "type": "permission",
      "permission_code": "project:approve"
    },
    {
      "type": "business_role",
      "business_role_id": "solar_farm_manager_uuid"
    }
  ]
}
```

This will notify:
- The person who submitted the form
- All users with `project:approve` permission
- All Solar Farm Managers

## Question 2: Where to Define Notifications?

### ✅ Recommended: Define in Workflow Transitions

Notifications should be defined **within the workflow transitions** (not in forms), because:

1. ✅ **Separation of Concerns** - Same form can have different workflows with different notification needs
2. ✅ **Event-Driven** - Notifications are triggered by state changes, not form structure
3. ✅ **Reusability** - Workflow definitions can be reused across multiple forms
4. ✅ **Flexibility** - Easy to modify notification logic without changing forms

### Workflow Definition Example

```json
{
  "code": "project_approval",
  "name": "Project Approval Workflow",
  "initial_state": "draft",
  "states": [
    {"code": "draft", "name": "Draft"},
    {"code": "submitted", "name": "Submitted for Approval"},
    {"code": "approved", "name": "Approved"},
    {"code": "rejected", "name": "Rejected"}
  ],
  "transitions": [
    {
      "from": "draft",
      "to": "submitted",
      "action": "submit",
      "label": "Submit for Approval",
      "permission": "project:create",
      "notifications": [
        {
          "recipients": [
            {
              "type": "permission",
              "permission_code": "project:approve"
            },
            {
              "type": "business_role",
              "business_role_id": "manager_role_uuid"
            }
          ],
          "title_template": "New Project Submission: {{form_title}}",
          "body_template": "{{submitter_name}} has submitted a project for your approval. Project: {{form_data.project_name}}",
          "priority": "high",
          "channels": ["in_app", "email"]
        }
      ]
    },
    {
      "from": "submitted",
      "to": "approved",
      "action": "approve",
      "label": "Approve Project",
      "permission": "project:approve",
      "requires_comment": false,
      "notifications": [
        {
          "recipients": [
            {
              "type": "submitter"
            },
            {
              "type": "field_value",
              "value": "project_engineer"
            }
          ],
          "title_template": "Project Approved: {{form_title}}",
          "body_template": "Your project '{{form_data.project_name}}' has been approved by {{approver_name}}",
          "priority": "normal",
          "channels": ["in_app", "email", "web_push"]
        }
      ]
    },
    {
      "from": "submitted",
      "to": "rejected",
      "action": "reject",
      "label": "Reject Project",
      "permission": "project:approve",
      "requires_comment": true,
      "notifications": [
        {
          "recipients": [
            {
              "type": "submitter"
            }
          ],
          "title_template": "Project Rejected: {{form_title}}",
          "body_template": "Your project '{{form_data.project_name}}' was rejected. Reason: {{comment}}",
          "priority": "high",
          "channels": ["in_app", "email"],
          "condition": {
            "always_notify_on_rejection": true
          }
        }
      ]
    }
  ]
}
```

## Template Variables

Notifications support dynamic template variables:

### Available Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{{form_title}}` | Form title | "Project Submission Form" |
| `{{form_code}}` | Form code | "project_submission" |
| `{{submitter_name}}` | Name of submitter | "John Doe" |
| `{{submitter_id}}` | ID of submitter | "user_123" |
| `{{approver_name}}` | Name of approver | "Jane Smith" |
| `{{current_state}}` | Current workflow state | "submitted" |
| `{{previous_state}}` | Previous workflow state | "draft" |
| `{{action}}` | Action performed | "approve" |
| `{{comment}}` | Transition comment | "Looks good!" |
| `{{form_data.field_name}}` | Form field value | `{{form_data.project_name}}` |
| `{{business_vertical}}` | Business vertical name | "Solar Farms" |
| `{{site_name}}` | Site name (if applicable) | "North Plant" |
| `{{submission_id}}` | Submission UUID | "uuid-here" |

## Notification Priorities

```go
type NotificationPriority string

const (
    NotificationPriorityLow      = "low"      // Informational
    NotificationPriorityNormal   = "normal"   // Standard notifications
    NotificationPriorityHigh     = "high"     // Important, requires attention
    NotificationPriorityCritical = "critical" // Urgent, immediate action needed
)
```

## Notification Channels

```go
type NotificationChannel string

const (
    NotificationChannelInApp   = "in_app"    // In-app notification bell
    NotificationChannelEmail   = "email"     // Email delivery
    NotificationChannelSMS     = "sms"       // SMS delivery
    NotificationChannelWebPush = "web_push"  // Browser push notification
)
```

## User Preferences

Users can control their notification preferences:

```json
{
  "user_id": "user_123",
  "enable_in_app": true,
  "enable_email": true,
  "enable_sms": false,
  "enable_web_push": true,
  "disabled_types": ["task_completed"],
  "quiet_hours_enabled": true,
  "quiet_hours_start": "22:00",
  "quiet_hours_end": "08:00",
  "digest_enabled": false,
  "digest_frequency": "daily"
}
```

## Implementation Flow

### 1. When Workflow Transition Occurs

```go
// In TransitionState function
func (we *WorkflowEngine) TransitionState(...) {
    // 1. Perform state transition
    submission.CurrentState = targetTransition.To

    // 2. Create transition record
    transition := models.WorkflowTransition{...}

    // 3. Trigger notifications
    if len(targetTransition.Notifications) > 0 {
        we.processTransitionNotifications(submission, transition, targetTransition)
    }
}
```

### 2. Notification Processing

```go
func (ns *NotificationService) processTransitionNotifications(...) {
    for _, notifConfig := range transition.Notifications {
        // 1. Resolve recipients based on type
        recipients := ns.resolveRecipients(notifConfig.Recipients, context)

        // 2. Render template with context data
        title := ns.renderTemplate(notifConfig.TitleTemplate, context)
        body := ns.renderTemplate(notifConfig.BodyTemplate, context)

        // 3. Check user preferences
        // 4. Create notification instances
        // 5. Send via configured channels
    }
}
```

### 3. Recipient Resolution

```go
func (ns *NotificationService) resolveRecipients(recipientDefs []NotificationRecipientDef, context Context) []string {
    userIDs := []string{}

    for _, def := range recipientDefs {
        switch def.Type {
        case "user":
            userIDs = append(userIDs, def.Value)

        case "role":
            users := ns.getUsersByRole(def.RoleID)
            userIDs = append(userIDs, users...)

        case "business_role":
            users := ns.getUsersByBusinessRole(def.BusinessRoleID, context.BusinessID)
            userIDs = append(userIDs, users...)

        case "permission":
            users := ns.getUsersByPermission(def.PermissionCode)
            userIDs = append(userIDs, users...)

        case "attribute":
            users := ns.getUsersByAttributes(def.AttributeQuery)
            userIDs = append(userIDs, users...)

        case "policy":
            users := ns.evaluatePolicy(def.PolicyID, context)
            userIDs = append(userIDs, users...)

        case "submitter":
            userIDs = append(userIDs, context.SubmitterID)

        case "approver":
            userIDs = append(userIDs, context.ActorID)

        case "field_value":
            if userID, ok := context.FormData[def.Value].(string); ok {
                userIDs = append(userIDs, userID)
            }
        }
    }

    // Deduplicate
    return unique(userIDs)
}
```

## API Endpoints

### User Notification Endpoints

```
GET    /api/v1/notifications                    - Get user's notifications
GET    /api/v1/notifications/:id                - Get specific notification
PATCH  /api/v1/notifications/:id/read           - Mark as read
PATCH  /api/v1/notifications/read-all           - Mark all as read
DELETE /api/v1/notifications/:id                - Delete notification
GET    /api/v1/notifications/unread-count       - Get unread count
```

### Notification Preferences

```
GET    /api/v1/notifications/preferences        - Get preferences
PUT    /api/v1/notifications/preferences        - Update preferences
```

### Admin Endpoints

```
GET    /api/v1/admin/notification-rules         - List all rules
POST   /api/v1/admin/notification-rules         - Create rule
GET    /api/v1/admin/notification-rules/:id     - Get rule
PUT    /api/v1/admin/notification-rules/:id     - Update rule
DELETE /api/v1/admin/notification-rules/:id     - Delete rule
```

## Example Use Cases

### Use Case 1: Approval Required

**Scenario:** When a project is submitted, notify all users who can approve it.

```json
{
  "recipients": [
    {"type": "permission", "permission_code": "project:approve"},
    {"type": "business_role", "business_role_id": "manager_uuid"}
  ],
  "title_template": "Approval Required: {{form_data.project_name}}",
  "body_template": "{{submitter_name}} has submitted a project for approval.",
  "priority": "high",
  "channels": ["in_app", "email"]
}
```

### Use Case 2: Field Team Assignment

**Scenario:** Notify the assigned engineer when a task is created.

```json
{
  "recipients": [
    {"type": "field_value", "value": "assigned_engineer"}
  ],
  "title_template": "New Task Assigned: {{form_data.task_title}}",
  "body_template": "You have been assigned a new task at {{form_data.site_name}}",
  "priority": "normal",
  "channels": ["in_app", "sms", "web_push"]
}
```

### Use Case 3: Regional Managers

**Scenario:** Notify all managers in a specific region using ABAC.

```json
{
  "recipients": [
    {
      "type": "attribute",
      "attribute_query": {
        "role_level": "manager",
        "region": "north"
      }
    }
  ],
  "title_template": "New Incident Report: {{form_data.incident_type}}",
  "body_template": "An incident has been reported at {{form_data.location}}",
  "priority": "critical",
  "channels": ["in_app", "email", "sms"]
}
```

## Best Practices

### 1. **Use Appropriate Priorities**
- `low` - FYI notifications, no action required
- `normal` - Standard workflow updates
- `high` - Requires attention or action
- `critical` - Urgent, immediate action needed

### 2. **Choose Right Channels**
- **In-App**: Always include for all notifications
- **Email**: For important updates that need documentation
- **SMS**: Only for critical, time-sensitive notifications
- **Web Push**: For real-time updates when user is online

### 3. **Template Best Practices**
- Keep titles short and descriptive (< 100 chars)
- Include key context in the body
- Always provide an action URL for navigation
- Use clear, professional language

### 4. **Recipient Targeting**
- Start broad (roles/permissions), refine as needed
- Use attributes for dynamic, context-based targeting
- Combine multiple recipient types for comprehensive coverage
- Always include `submitter` for status updates

### 5. **Performance Considerations**
- Use `batch_interval_minutes` for non-urgent notifications
- Set `deduplicate_key` to prevent spam
- Respect user preferences
- Use indexes for query optimization

## Migration and Testing

### 1. Run Migrations
```bash
go run . migrate
```

### 2. Create Sample Workflow with Notifications
```sql
INSERT INTO workflow_definitions (code, name, transitions) VALUES (
  'sample_workflow',
  'Sample Workflow with Notifications',
  '[{
    "from": "draft",
    "to": "submitted",
    "action": "submit",
    "notifications": [{
      "recipients": [{"type": "permission", "permission_code": "admin_all"}],
      "title_template": "Test Notification",
      "body_template": "This is a test notification",
      "priority": "normal",
      "channels": ["in_app"]
    }]
  }]'::jsonb
);
```

### 3. Test Notification Flow
1. Create a form submission
2. Perform a transition
3. Verify notification is created
4. Check recipient resolution
5. Verify delivery channels

## Future Enhancements

1. **Notification Templates Library** - Pre-built templates for common scenarios
2. **Notification Analytics** - Track open rates, click rates, delivery stats
3. **Advanced Scheduling** - Time-based delivery rules
4. **Notification Grouping** - Smart grouping of related notifications
5. **Rich Media Support** - Images, attachments in notifications
6. **Webhook Integration** - External system notifications
7. **Custom Channels** - Slack, Teams, WhatsApp integration

## Summary

Your notification system now supports:

✅ **Multi-level targeting**: Users, Roles, Business Roles, Permissions, Attributes, Policies
✅ **Workflow integration**: Notifications defined in transitions
✅ **Template-based content**: Dynamic content with variables
✅ **Multiple channels**: In-app, Email, SMS, Web Push
✅ **User preferences**: Granular control over delivery
✅ **Priority levels**: From low to critical
✅ **Context-aware**: Access to form data, workflow state, actors
✅ **Flexible recipient resolution**: Static and dynamic targeting
✅ **RBAC/ABAC/PBAC integration**: Leverages existing authorization system

This design provides maximum flexibility while maintaining clean separation of concerns and reusability.

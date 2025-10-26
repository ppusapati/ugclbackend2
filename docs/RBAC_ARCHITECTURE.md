# RBAC Architecture - UGCL Backend

## Overview

The UGCL backend implements a sophisticated **dual-role system** that combines global roles with business vertical-specific roles, allowing for flexible permission management across multiple business units.

## Architecture Components

### 1. Global Roles (`Role`)

Global roles provide system-wide permissions and are stored in the `roles` table.

**Model**: [models/permission.go](../models/permission.go)

```go
type Role struct {
    ID          uuid.UUID
    Name        string       // e.g., "super_admin", "System_Admin", "Admin"
    Description string
    IsActive    bool
    IsGlobal    bool         // true for global roles
    Level       int          // Hierarchy: 0=super_admin, 1=system_admin, 2-5=others
    Permissions []Permission // Many-to-many relationship
}
```

**Predefined Global Roles**:
- **super_admin** (Level 0) - Full system access with wildcard permission `*:*:*`
- **System_Admin** (Level 1) - User and role management across system
- **Admin** (Level 1) - Head Office admin with finance, HR, and reporting access
- **Manager** - Department-level manager with approval permissions
- **Consultant** - Limited access to planning and project modules

### 2. Business Roles (`BusinessRole`)

Business roles are specific to business verticals (WATER, SOLAR, HO, CONTRACTORS) and stored in the `business_roles` table.

**Model**: [models/business.go](../models/business.go)

```go
type BusinessRole struct {
    ID                 uuid.UUID
    Name               string           // e.g., "Water_Admin", "Solar_Admin"
    DisplayName        string           // User-friendly name
    Description        string
    BusinessVerticalID uuid.UUID        // Links to specific business vertical
    Permissions        []Permission     // Many-to-many relationship
    Level              int              // Hierarchy: 1=highest, 5=lowest
}
```

**Business Verticals**:
1. **WATER** - Water Works operations
2. **SOLAR** - Solar energy generation
3. **HO** - Head Office administration
4. **CONTRACTORS** - Contractor/subcontractor management

### 3. User-Business-Role Assignment (`UserBusinessRole`)

Users can have multiple business roles across different verticals through the `user_business_roles` junction table.

```go
type UserBusinessRole struct {
    ID             uuid.UUID
    UserID         uuid.UUID
    BusinessRoleID uuid.UUID
    IsActive       bool
    AssignedAt     time.Time
    AssignedBy     *uuid.UUID  // Who assigned this role
}
```

### 4. User Model

The `User` model combines both global and business roles:

```go
type User struct {
    ID                 uuid.UUID
    Name               string
    Email              string
    Phone              string
    PasswordHash       string
    RoleID             *uuid.UUID        // Global role (optional)
    RoleModel          *Role             // Relationship to global Role
    BusinessVerticalID *uuid.UUID        // Primary business vertical
    UserBusinessRoles  []UserBusinessRole // Multiple business roles
}
```

## Permission System

### Permission Structure

Permissions follow a `resource:action` naming convention:

```
project:create
project:read
project:update
project:delete
water:read_consumption
solar:manage_panels
user:create
role:assign
```

### Permission Checking

Users inherit permissions from:
1. **Global Role** (if assigned via `RoleID`)
2. **All Active Business Roles** (via `UserBusinessRoles`)

**Super Admin**: Has wildcard permission `*:*:*` granting access to everything.

**Example Permission Check**:
```go
func (u *User) HasPermission(permissionName string) bool {
    // Check global role
    if u.RoleModel != nil {
        return u.RoleModel.HasPermission(permissionName)
    }
    return false
}

func (u *User) HasPermissionInVertical(permission string, verticalID uuid.UUID) bool {
    // Check business role permissions in specific vertical
    for _, ubr := range u.UserBusinessRoles {
        if ubr.IsActive && ubr.BusinessRole.BusinessVerticalID == verticalID {
            for _, perm := range ubr.BusinessRole.Permissions {
                if perm.Name == permission {
                    return true
                }
            }
        }
    }
    return false
}
```

## Role Hierarchy and Assignment

### Hierarchy Levels

| Level | Description | Can Assign Levels |
|-------|-------------|------------------|
| 0 | Super Admin | 1-5 (all) |
| 1 | Vertical Admin / System Admin | 2-5 |
| 2 | Manager / Project Coordinator | 3-5 |
| 3 | Senior Engineer / Specialist | 4-5 |
| 4 | Engineer / Supervisor | 5 |
| 5 | Operator / Skilled Worker | - (none) |

### Role Assignment Rules

1. **Level-based Authorization**: Users can only assign roles with a **higher level number** (lower privilege) than their own highest role
2. **Vertical Constraint**: Business role assignments are constrained to specific business verticals
3. **One Role Per Vertical**: Users can have only ONE active business role per vertical at a time
4. **Assignment Tracking**: All role assignments track who assigned them (`AssignedBy`)

**Example**:
```go
func (u *User) CanAssignRole(targetRoleLevel int) bool {
    userLevel := u.GetHighestRoleLevel()
    return userLevel < targetRoleLevel
}
```

## Business Vertical Roles

### Water Works (WATER)

| Role | Level | Description |
|------|-------|-------------|
| Water_Admin | 1 | Full control within Water vertical |
| Project_Coordinator | 2 | Manage projects, assign tasks |
| Sr_Deputy_PM | 2 | Approve projects & plans |
| Engineer | 4 | Execute tasks, manage water systems |
| Supervisor | 4 | Supervise field execution |
| Operator | 5 | Operate water systems |
| Skilled_Worker | 5 | Basic field execution tasks |

### Solar Works (SOLAR)

| Role | Level | Description |
|------|-------|-------------|
| Solar_Admin | 1 | Full Solar vertical access |
| Area_Project_Manager | 2 | Manage projects, plans, approvals |
| Sr_Engineer | 3 | Manage panels, generation, maintenance |
| Skilled_Worker | 5 | Basic field execution tasks |

### Head Office (HO)

| Role | Level | Description |
|------|-------|-------------|
| HO_Admin | 1 | Full access to HO modules |
| HO_Manager | 2 | Manage projects, purchases, planning |
| HO_HR | 3 | Access HR & Payroll modules |
| HO_Consultant | 4 | Read/write access to Projects & Planning |
| HO_Skilled_Worker | 5 | Basic administrative tasks |

### Contractors (CONTRACTORS)

| Role | Level | Description |
|------|-------|-------------|
| Sub_Contractor | 5 | Read-only access to Projects, Materials, Inventory |

## API Endpoints

### Global Role Management

```
GET    /api/v1/admin/roles              - List all global roles
POST   /api/v1/admin/roles              - Create global role
PUT    /api/v1/admin/roles/{id}         - Update global role
DELETE /api/v1/admin/roles/{id}         - Delete global role
GET    /api/v1/admin/permissions        - List all permissions
POST   /api/v1/admin/permissions        - Create permission
```

### Business Role Management

```
GET    /api/v1/admin/businesses                        - List business verticals
POST   /api/v1/admin/businesses                        - Create business vertical
GET    /api/v1/business/{businessCode}/roles           - List roles for vertical
POST   /api/v1/business/{businessCode}/roles           - Create business role
```

### User Role Assignment

```
POST   /api/v1/users/{id}/roles/assign                 - Assign business role to user
DELETE /api/v1/users/{id}/roles/{roleId}               - Remove business role
GET    /api/v1/users/{id}/roles                        - Get user's business roles
GET    /api/v1/users/{id}/assignable-roles?verticalId= - Get roles user can assign
GET    /api/v1/business-verticals/{id}/roles           - Get all roles for vertical
```

### Authentication & Profile

```
POST   /api/v1/register    - Register new user (with role_id)
POST   /api/v1/login       - Login (returns token with role info)
GET    /api/v1/token       - Get current user with permissions
GET    /api/v1/profile     - Get user profile
```

## Middleware

### Authorization Middleware

1. **RequirePermission** - Checks if user has specific global or business permission
2. **RequireBusinessPermission** - Checks permission within current business context
3. **RequireBusinessAccess** - Validates user has access to business vertical
4. **JWTMiddleware** - Validates JWT token and loads user context

**Example**:
```go
api.Handle("/admin/users",
    middleware.RequirePermission("user:create")(
        http.HandlerFunc(handlers.CreateUser)
    )).Methods("POST")

business.Handle("/reports",
    middleware.RequireBusinessPermission("read_reports")(
        http.HandlerFunc(handlers.GetBusinessReports)
    )).Methods("GET")
```

## Database Schema

### Key Tables

```sql
-- Global roles
roles (id, name, description, is_global, is_active, level)
permissions (id, name, description, resource, action)
role_permissions (role_id, permission_id)

-- Business verticals and roles
business_verticals (id, name, code, description, is_active)
business_roles (id, name, display_name, business_vertical_id, level)
business_role_permissions (business_role_id, permission_id)

-- User assignments
users (id, name, email, phone, role_id, business_vertical_id)
user_business_roles (id, user_id, business_role_id, is_active, assigned_by)
```

## Migration & Seeding

### Initial Setup

Run migrations to create tables and seed default data:

```go
// In main.go or migration runner
config.Migrations(db)           // Create tables
config.SeedPermissions()        // Seed permissions and global roles
config.SeedBusinessVerticals()  // Seed business verticals and roles
config.MigrateToNewRBAC()      // Migrate existing users
```

### Verification

```go
config.VerifyRBACMigration()   // Check migration success
```

## User Scenarios

### Scenario 1: Super Admin

```json
{
  "id": "uuid",
  "name": "Super Admin",
  "role_id": "super_admin_role_uuid",
  "global_role": "super_admin",
  "permissions": ["*:*:*"],
  "business_roles": []
}
```

- Has access to **everything** via wildcard permission
- Can manage all users, roles, and business verticals
- Can assign any role to any user

### Scenario 2: Multi-Vertical Admin

```json
{
  "id": "uuid",
  "name": "Multi Admin",
  "role_id": null,
  "global_role": null,
  "permissions": ["project:create", "project:read", ...],
  "business_roles": [
    {
      "role_name": "Water Admin",
      "vertical_code": "WATER",
      "level": 1
    },
    {
      "role_name": "Solar Admin",
      "vertical_code": "SOLAR",
      "level": 1
    }
  ]
}
```

- No global role
- Has admin roles in **multiple** business verticals
- Can manage users within WATER and SOLAR verticals
- Cannot manage HO or CONTRACTORS

### Scenario 3: Vertical-Specific User

```json
{
  "id": "uuid",
  "name": "Water Engineer",
  "role_id": null,
  "global_role": null,
  "permissions": ["project:read", "inventory:create", ...],
  "business_roles": [
    {
      "role_name": "Water Engineer",
      "vertical_code": "WATER",
      "level": 4
    }
  ]
}
```

- Only has access to WATER vertical
- Can perform engineer-level tasks
- Cannot assign roles or manage other verticals

## Best Practices

1. **Minimal Permissions**: Assign only necessary permissions to each role
2. **Level Hierarchy**: Respect the level-based hierarchy for role assignments
3. **Business Context**: Always use business-specific middleware for vertical routes
4. **Permission Naming**: Use consistent `resource:action` format
5. **Audit Trail**: Track role assignments via `AssignedBy` field
6. **Active Status**: Check `IsActive` before granting permissions
7. **Token Refresh**: Ensure tokens contain updated role information

## Related Files

- [models/user.go](../models/user.go) - User model with role relationships
- [models/permission.go](../models/permission.go) - Global roles and permissions
- [models/business.go](../models/business.go) - Business verticals and roles
- [middleware/authorization.go](../middleware/authorization.go) - Permission checking
- [middleware/role_level.go](../middleware/role_level.go) - Role hierarchy validation
- [handlers/role_assignment.go](../handlers/role_assignment.go) - Role assignment handlers
- [config/permissions.go](../config/permissions.go) - Permission and role seeding

## Migration Notes

The system has migrated away from string-based roles (`user.Role` field) to the new RBAC system. All legacy references have been removed as of this documentation.

**Breaking Changes**:
- `user.Role` field removed from User model
- Registration now accepts `role_id` instead of `role` string
- Login response returns `role_id` and `global_role` instead of `role`
- GetCurrentUser returns structured permissions and business_roles

**Backward Compatibility**: None - all clients must update to use new role structure.

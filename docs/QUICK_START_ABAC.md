# ABAC Quick Start Guide

## Step-by-Step Setup

### 1. Run Database Migrations

The migrations will run automatically when you start the server:

```bash
cd d:\Maheshwari\UGCL\backend\v1
go run main.go
```

This will create the following new tables:
- `attributes`
- `user_attributes`
- `resource_attributes`
- `policies`
- `policy_rules`
- `policy_evaluations`

### 2. Seed ABAC Data

Add the following to your `main.go` file (or create a separate seed command):

```go
package main

import (
    "log"
    "p9e.in/ugcl/config"
    // ... other imports
)

func main() {
    // ... existing initialization code ...

    config.Connect()

    // Run migrations
    if err := config.Migrations(config.DB); err != nil {
        log.Fatalf("could not run migrations: %v", err)
    }

    // NEW: Seed ABAC data
    if err := config.RunABACSeeding(config.DB); err != nil {
        log.Printf("Warning: ABAC seeding failed: %v", err)
    } else {
        log.Println("✅ ABAC data seeded successfully")
    }

    // ... rest of your main function ...
}
```

Or create a separate seed command:

```go
// cmd/seed/main.go
package main

import (
    "log"
    "p9e.in/ugcl/config"
)

func main() {
    config.Connect()

    log.Println("Starting ABAC seeding...")
    if err := config.RunABACSeeding(config.DB); err != nil {
        log.Fatalf("Seeding failed: %v", err)
    }
    log.Println("✅ Seeding completed successfully!")
}
```

Run it:
```bash
go run cmd/seed/main.go
```

### 3. Verify Installation

Check that tables were created:

```sql
-- In your PostgreSQL database
\dt
```

You should see the new tables:
- attributes
- user_attributes
- resource_attributes
- policies
- policy_rules
- policy_evaluations

Check seeded data:

```sql
-- Check attributes
SELECT name, display_name, type FROM attributes;

-- Check policies
SELECT name, display_name, status FROM policies;

-- Check permissions
SELECT name FROM permissions WHERE name LIKE 'manage_%';
```

### 4. Test API Endpoints

#### Get all attributes:
```bash
curl -X GET http://localhost:8080/api/v1/attributes \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

#### Get all policies:
```bash
curl -X GET http://localhost:8080/api/v1/policies \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

#### Assign an attribute to a user:
```bash
# First, get attribute IDs
curl -X GET http://localhost:8080/api/v1/attributes?type=user \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"

# Then assign (replace UUIDs with actual values)
curl -X POST http://localhost:8080/api/v1/users/USER_UUID/attributes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "attribute_id": "ATTRIBUTE_UUID",
    "value": "engineering"
  }'
```

#### Test a policy:
```bash
# Get a policy ID first
curl -X GET http://localhost:8080/api/v1/policies \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"

# Test it
curl -X POST http://localhost:8080/api/v1/policies/POLICY_UUID/test \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "user_id": "USER_UUID",
    "action": "purchase:create",
    "resource_type": "purchase",
    "user_attributes": {
      "user.role": "manager"
    },
    "environment": {
      "environment.day_of_week": "Saturday"
    }
  }'
```

### 5. Grant ABAC Permissions to Admins

Update your super_admin and system_admin roles to include the new ABAC permissions.

You can do this via SQL or through your application:

```sql
-- Get permission IDs
SELECT id, name FROM permissions WHERE name IN (
    'manage_policies',
    'manage_attributes',
    'manage_user_attributes',
    'manage_resource_attributes',
    'view_policy_evaluations'
);

-- Get super_admin role ID
SELECT id FROM roles WHERE name = 'super_admin';

-- Assign permissions to super_admin (replace UUIDs)
INSERT INTO role_permissions (role_id, permission_id)
VALUES
    ('SUPER_ADMIN_ROLE_ID', 'MANAGE_POLICIES_PERMISSION_ID'),
    ('SUPER_ADMIN_ROLE_ID', 'MANAGE_ATTRIBUTES_PERMISSION_ID'),
    ('SUPER_ADMIN_ROLE_ID', 'MANAGE_USER_ATTRIBUTES_PERMISSION_ID'),
    ('SUPER_ADMIN_ROLE_ID', 'MANAGE_RESOURCE_ATTRIBUTES_PERMISSION_ID'),
    ('SUPER_ADMIN_ROLE_ID', 'VIEW_POLICY_EVALUATIONS_PERMISSION_ID')
ON CONFLICT DO NOTHING;
```

### 6. Create Your First Policy

```bash
curl -X POST http://localhost:8080/api/v1/policies \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "name": "my_first_policy",
    "display_name": "My First Policy",
    "description": "A simple test policy",
    "effect": "ALLOW",
    "priority": 50,
    "status": "draft",
    "actions": ["read"],
    "resources": ["*"],
    "conditions": {
      "AND": [
        {
          "attribute": "user.department",
          "operator": "=",
          "value": "engineering"
        }
      ]
    }
  }'
```

Then activate it:

```bash
curl -X POST http://localhost:8080/api/v1/policies/POLICY_UUID/activate \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

---

## Common Issues & Solutions

### Issue: Migrations fail

**Solution**: Check your database connection and ensure you have the latest schema.

```bash
# Check connection
psql -U postgres -d ugcl -c "SELECT 1;"

# If needed, manually run migrations
psql -U postgres -d ugcl < migrations.sql
```

### Issue: Seeding fails with "attribute already exists"

**Solution**: This is normal if you run seeding multiple times. The seeding is idempotent.

### Issue: Permission denied when accessing endpoints

**Solution**: Make sure your JWT token has the required permissions. Super admins bypass all checks.

### Issue: Policy evaluation returns unexpected results

**Solution**: Use the `/policies/{id}/test` endpoint to debug. Check:
1. Policy status is "active"
2. Policy conditions match your test data
3. Attribute values are assigned correctly

---

## What's Next?

1. **Assign Attributes**: Give users clearance levels, departments, etc.
2. **Create Policies**: Define your access control rules
3. **Test Thoroughly**: Use the test endpoint before activating policies
4. **Monitor Evaluations**: Check the `policy_evaluations` table for audit logs
5. **Build Frontend UI**: Create admin interfaces for policy management

---

## Quick Reference

### Pre-seeded System Attributes

**User Attributes:**
- `user.department` - Department (engineering, hr, finance, operations, management)
- `user.clearance_level` - Security clearance (1-5)
- `user.employment_type` - Employment type (permanent, contract, consultant, intern)
- `user.location` - Geographic location
- `user.manager_id` - Manager's user ID
- `user.years_of_service` - Years employed

**Resource Attributes:**
- `resource.sensitivity` - Data sensitivity (public, internal, confidential, secret)
- `resource.owner_id` - Resource owner user ID
- `resource.project_id` - Associated project ID
- `resource.cost_center` - Cost center
- `resource.amount` - Monetary value

**Environment Attributes:**
- `environment.time_of_day` - Current time
- `environment.day_of_week` - Day of week
- `environment.ip_address` - Request IP
- `environment.location` - Request location
- `environment.device_type` - Device type

**Action Attributes:**
- `action.operation_type` - Operation type (read, create, update, delete, approve, reject)
- `action.risk_level` - Risk level (low, medium, high, critical)

### Pre-seeded Sample Policies

1. **restrict_high_value_purchases_after_hours** - Deny purchases >₹100k outside 9-5
2. **allow_resource_owner_full_access** - Owners have full access
3. **restrict_confidential_data_by_clearance** - Clearance level < 3 denied confidential data
4. **allow_manager_approve_subordinate_requests** - Managers approve their team
5. **restrict_weekend_operations** - No critical ops on weekends

### Supported Operators

- Comparison: `=`, `!=`, `>`, `<`, `>=`, `<=`
- Set: `IN`, `NOT_IN`
- Pattern: `CONTAINS`, `MATCHES`, `STARTS_WITH`, `ENDS_WITH`
- Range: `BETWEEN`, `NOT_BETWEEN`
- Logical: `AND`, `OR`, `NOT`

---

**You're all set! 🎉**

For detailed documentation, see [ABAC_IMPLEMENTATION_GUIDE.md](./ABAC_IMPLEMENTATION_GUIDE.md)

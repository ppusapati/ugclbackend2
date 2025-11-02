# ABAC & Policy-Based Authorization Schema

## Database Schema Overview

This document outlines the new database schema for Attribute-Based Access Control (ABAC) and Policy-Based Authorization.

## New Tables

### 1. attributes
Defines all available attributes that can be used in policies.

```sql
CREATE TABLE attributes (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,           -- e.g., "user.department"
    display_name VARCHAR(100) NOT NULL,          -- e.g., "User Department"
    description VARCHAR(500),
    type VARCHAR(50) NOT NULL,                   -- user, resource, environment, action
    data_type VARCHAR(50) NOT NULL,              -- string, integer, float, boolean, datetime, json, array
    is_system BOOLEAN DEFAULT false,             -- System-defined or user-defined
    is_active BOOLEAN DEFAULT true,
    metadata JSONB,                              -- Additional config (allowed_values, validation_rules)
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE INDEX idx_attributes_type ON attributes(type);
CREATE INDEX idx_attributes_is_active ON attributes(is_active);
```

### 2. user_attributes
Stores attribute values assigned to users.

```sql
CREATE TABLE user_attributes (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    attribute_id UUID NOT NULL REFERENCES attributes(id),
    value TEXT NOT NULL,                         -- Stored as string, parsed by data_type
    is_active BOOLEAN DEFAULT true,
    valid_from TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    valid_until TIMESTAMP NULL,                  -- Optional expiration
    assigned_by UUID REFERENCES users(id),
    created_at TIMESTAMP,
    updated_at TIMESTAMP,

    CONSTRAINT unique_user_attribute UNIQUE(user_id, attribute_id)
);

CREATE INDEX idx_user_attr ON user_attributes(user_id, attribute_id);
CREATE INDEX idx_user_attr_active ON user_attributes(is_active);
CREATE INDEX idx_user_attr_valid ON user_attributes(valid_from, valid_until);
```

### 3. resource_attributes
Stores attribute values assigned to resources.

```sql
CREATE TABLE resource_attributes (
    id UUID PRIMARY KEY,
    resource_type VARCHAR(50) NOT NULL,          -- e.g., "project", "report", "site"
    resource_id UUID NOT NULL,                   -- ID of the resource
    attribute_id UUID NOT NULL REFERENCES attributes(id),
    value TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    valid_from TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    valid_until TIMESTAMP NULL,
    assigned_by UUID REFERENCES users(id),
    created_at TIMESTAMP,
    updated_at TIMESTAMP,

    CONSTRAINT unique_resource_attribute UNIQUE(resource_type, resource_id, attribute_id)
);

CREATE INDEX idx_resource_attr ON resource_attributes(resource_type, resource_id, attribute_id);
CREATE INDEX idx_resource_attr_type ON resource_attributes(resource_type);
CREATE INDEX idx_resource_attr_active ON resource_attributes(is_active);
```

### 4. policies
Defines access control policies with conditions.

```sql
CREATE TABLE policies (
    id UUID PRIMARY KEY,
    name VARCHAR(200) NOT NULL UNIQUE,
    display_name VARCHAR(200) NOT NULL,
    description TEXT,
    effect VARCHAR(10) NOT NULL,                 -- ALLOW or DENY
    priority INTEGER DEFAULT 0,                  -- Higher priority evaluated first
    status VARCHAR(20) DEFAULT 'draft',          -- active, inactive, draft, archived
    business_vertical_id UUID REFERENCES business_verticals(id),  -- NULL = global policy
    conditions JSONB NOT NULL,                   -- Complex condition tree
    actions JSONB,                               -- Array of actions
    resources JSONB,                             -- Array of resource patterns
    metadata JSONB,
    valid_from TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    valid_until TIMESTAMP NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE INDEX idx_policies_status ON policies(status);
CREATE INDEX idx_policies_business ON policies(business_vertical_id);
CREATE INDEX idx_policies_priority ON policies(priority DESC);
CREATE INDEX idx_policies_effect ON policies(effect);
CREATE INDEX idx_policies_valid ON policies(valid_from, valid_until);
```

### 5. policy_rules
Individual rules within a policy (optional, for complex policies).

```sql
CREATE TABLE policy_rules (
    id UUID PRIMARY KEY,
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    condition JSONB NOT NULL,                    -- Single condition
    is_active BOOLEAN DEFAULT true,
    "order" INTEGER DEFAULT 0,                   -- Evaluation order
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE INDEX idx_policy_rules_policy ON policy_rules(policy_id);
CREATE INDEX idx_policy_rules_order ON policy_rules(policy_id, "order");
```

### 6. policy_evaluations
Audit log of policy evaluation results.

```sql
CREATE TABLE policy_evaluations (
    id UUID PRIMARY KEY,
    policy_id UUID NOT NULL REFERENCES policies(id),
    user_id UUID NOT NULL REFERENCES users(id),
    resource_type VARCHAR(50),
    resource_id UUID,
    action VARCHAR(100) NOT NULL,
    effect VARCHAR(10) NOT NULL,                 -- Final decision: ALLOW or DENY
    context JSONB,                               -- Context at evaluation time
    matched_conditions JSONB,                    -- Which conditions matched
    evaluation_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(50),
    user_agent VARCHAR(500),
    request_path VARCHAR(500),
    evaluation_duration_ms INTEGER DEFAULT 0     -- Performance metric
);

CREATE INDEX idx_policy_eval_policy ON policy_evaluations(policy_id);
CREATE INDEX idx_policy_eval_user ON policy_evaluations(user_id);
CREATE INDEX idx_policy_eval_time ON policy_evaluations(evaluation_time DESC);
CREATE INDEX idx_policy_eval_resource ON policy_evaluations(resource_type, resource_id);
CREATE INDEX idx_policy_eval_effect ON policy_evaluations(effect);
```

## Attribute Categories

### User Attributes (user.*)
- `user.department` - Department the user belongs to
- `user.clearance_level` - Security clearance (1-5)
- `user.employment_type` - permanent, contract, consultant, intern
- `user.location` - Geographic location
- `user.manager_id` - User's manager UUID
- `user.years_of_service` - Employment duration

### Resource Attributes (resource.*)
- `resource.sensitivity` - public, internal, confidential, secret
- `resource.owner_id` - Resource owner UUID
- `resource.project_id` - Associated project UUID
- `resource.cost_center` - Financial cost center
- `resource.amount` - Monetary value

### Environment Attributes (environment.*)
- `environment.time_of_day` - Current time (HH:MM)
- `environment.day_of_week` - Monday-Sunday
- `environment.ip_address` - Request IP
- `environment.location` - Request geo location
- `environment.device_type` - mobile, desktop, tablet

### Action Attributes (action.*)
- `action.operation_type` - read, create, update, delete, approve, reject
- `action.risk_level` - low, medium, high, critical

## Policy Condition Structure

Policies support complex conditional logic using JSON:

### Simple Condition
```json
{
  "attribute": "user.department",
  "operator": "=",
  "value": "engineering"
}
```

### Complex Condition Tree
```json
{
  "AND": [
    {
      "attribute": "resource.amount",
      "operator": ">",
      "value": 100000
    },
    {
      "OR": [
        {
          "attribute": "environment.hour",
          "operator": "<",
          "value": 9
        },
        {
          "attribute": "environment.hour",
          "operator": ">=",
          "value": 17
        }
      ]
    }
  ]
}
```

## Supported Operators

- **Comparison**: `=`, `!=`, `>`, `<`, `>=`, `<=`
- **Set Operations**: `IN`, `NOT_IN`
- **Pattern Matching**: `CONTAINS`, `MATCHES` (regex), `STARTS_WITH`, `ENDS_WITH`
- **Range**: `BETWEEN`, `NOT_BETWEEN`
- **Geo**: `WITHIN_RADIUS`, `IN_REGION`
- **Logical**: `AND`, `OR`, `NOT`

## Example Policies

### 1. Restrict High-Value Purchases After Hours
```json
{
  "name": "restrict_high_value_purchases_after_hours",
  "effect": "DENY",
  "priority": 100,
  "conditions": {
    "AND": [
      {"attribute": "resource.amount", "operator": ">", "value": 100000},
      {"attribute": "environment.hour", "operator": "NOT_BETWEEN", "value": [9, 17]},
      {"attribute": "user.role", "operator": "!=", "value": "super_admin"}
    ]
  },
  "actions": ["purchase:create", "purchase:approve"],
  "resources": ["purchase"]
}
```

### 2. Allow Resource Owner Full Access
```json
{
  "name": "allow_resource_owner_full_access",
  "effect": "ALLOW",
  "priority": 90,
  "conditions": {
    "AND": [
      {"attribute": "user.id", "operator": "=", "value": "{{resource.owner_id}}"}
    ]
  },
  "actions": ["*"],
  "resources": ["*"]
}
```

### 3. Restrict by Clearance Level
```json
{
  "name": "restrict_confidential_data_by_clearance",
  "effect": "DENY",
  "priority": 80,
  "conditions": {
    "AND": [
      {"attribute": "resource.sensitivity", "operator": "IN", "value": ["confidential", "secret"]},
      {"attribute": "user.clearance_level", "operator": "<", "value": 3}
    ]
  },
  "actions": ["read", "update", "delete"],
  "resources": ["*"]
}
```

## Integration with Existing RBAC

The ABAC system enhances existing RBAC:

1. **RBAC First**: Check role-based permissions (fast)
2. **ABAC Second**: Evaluate attribute-based policies (context-aware)
3. **Policy Evaluation**: Apply policy rules (complex business logic)
4. **Final Decision**:
   - Any DENY policy → Access Denied
   - At least one ALLOW policy + No DENY → Access Allowed
   - No matching policies → Defer to RBAC decision

## Performance Considerations

- **Caching**: Policy evaluation results cached for 5 minutes
- **Indexing**: All foreign keys and filter fields indexed
- **Lazy Evaluation**: Short-circuit on first DENY
- **Priority Ordering**: High-priority policies evaluated first
- **Audit Async**: Policy evaluations logged asynchronously

## Migration Path

1. ✅ Create new tables (attributes, user_attributes, resource_attributes, policies, policy_rules, policy_evaluations)
2. ✅ Seed system attributes
3. ✅ Create sample policies
4. ⏳ Implement policy engine
5. ⏳ Add ABAC middleware
6. ⏳ Update frontend for policy management
7. ⏳ Gradually migrate critical permissions to policies
8. ⏳ Monitor and optimize policy performance

## Next Steps

- Implement policy evaluation engine
- Create policy management API endpoints
- Build frontend UI for policy creation and testing
- Add caching layer for performance
- Set up audit logging and monitoring

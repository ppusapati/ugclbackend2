# ABAC Policy Enforcement - How It Works

## Quick Answer

**No, ABAC policies are NOT checked on every request by default.**

ABAC policies are only evaluated when you explicitly use the ABAC middleware (`RequireABACPolicy` or `RequireHybridAuth`) on specific routes.

## Current Implementation

### What's Happening Now

Looking at your codebase:

1. **ABAC Routes** ([routes/abac_routes.go](../routes/abac_routes.go))
   - These routes are for **managing** ABAC policies (CRUD operations)
   - They use **RBAC** (RequirePermission) for authorization
   - They do NOT enforce ABAC policies themselves

2. **ABAC Middleware** ([middleware/abac_middleware.go](../middleware/abac_middleware.go))
   - Exists and ready to use
   - But **NOT currently applied** to any application routes
   - Only available for opt-in usage

### Example: ABAC Route Setup

```go
// From abac_routes.go - These routes MANAGE policies
policyRouter.Handle("",
    middleware.RequirePermission("manage_policies")(
        http.HandlerFunc(handlers.ListPolicies))).Methods("GET")

// This uses RBAC, not ABAC!
// It checks if user has "manage_policies" permission
// It does NOT evaluate ABAC policies
```

### Current Authorization Flow

```
Request
  ↓
JWT Middleware (authentication)
  ↓
RequirePermission("manage_policies") ← RBAC only
  ↓
Handler (manages policies)
```

## How ABAC Works (When Used)

### Option 1: ABAC Only

```go
// Apply ABAC middleware to a route
r.Handle("/api/v1/sensitive-data",
    middleware.RequireABACPolicy("read", "sensitive_data")(
        http.HandlerFunc(handlers.GetSensitiveData))).Methods("GET")
```

**Flow:**
```
Request
  ↓
JWT Middleware
  ↓
RequireABACPolicy
  ↓
  1. Load user attributes from database
  2. Load resource attributes (if resource specified)
  3. Gather environment attributes (IP, user agent, etc.)
  4. Evaluate ALL active policies for this action+resource
  5. Make decision (allow/deny)
  ↓
Handler (only if allowed)
```

### Option 2: Hybrid RBAC + ABAC

```go
// First check RBAC permission, then ABAC policies
r.Handle("/api/v1/reports",
    middleware.RequireHybridAuth("read_reports", "read", "report")(
        http.HandlerFunc(handlers.GetReports))).Methods("GET")
```

**Flow:**
```
Request
  ↓
JWT Middleware
  ↓
RequirePermission("read_reports") ← RBAC first
  ↓ (only if RBAC passes)
RequireABACPolicy("read", "report") ← ABAC second
  ↓ (only if ABAC allows)
Handler
```

## Performance Impact

### If ABAC is NOT used (Current State)
- ✅ **Zero overhead** - policies not evaluated
- ✅ **Fast** - only RBAC permission checks
- ✅ **Simple** - just database lookup for user permissions

### If ABAC IS used on every route
- ⚠️ **Database queries** for each request:
  - User attributes lookup
  - Resource attributes lookup (if applicable)
  - Active policies retrieval
  - Policy evaluation history (optional logging)
- ⚠️ **Policy evaluation** - complex condition checking
- ⚠️ **Slower** - could add 50-200ms per request
- ⚠️ **Database load** - significant increase

## When to Use ABAC

### Use ABAC When You Need:

1. **Fine-grained, context-aware access control**
   ```go
   // Example: Only allow access during business hours
   middleware.RequireABACPolicy("read", "financial_report")

   // Policy checks:
   // - User department
   // - Report classification level
   // - Current time
   // - User's clearance level
   ```

2. **Complex conditional logic**
   ```go
   // Example: Dynamic approval workflows
   // - Manager can approve < $10,000
   // - Director can approve < $50,000
   // - VP needed for > $50,000
   middleware.RequireABACPolicy("approve", "purchase_order")
   ```

3. **Attribute-based decisions**
   ```go
   // Example: Data sovereignty
   // Users can only access data from their region
   middleware.RequireABACPolicy("read", "customer_data")
   ```

4. **Temporary access**
   ```go
   // Example: Time-limited access
   // Contractor has access only during project duration
   middleware.RequireABACPolicy("read", "project_data")
   ```

### Use RBAC When You Need:

1. **Simple role-based permissions**
   ```go
   // Admins can manage users
   middleware.RequirePermission("manage_users")
   ```

2. **Fast, simple checks**
   - User has permission or doesn't
   - No complex conditions

3. **Most common cases** (80% of authorization needs)

## Recommended Approach

### Layered Authorization Strategy

```
┌─────────────────────────────────────────────────┐
│  Layer 1: JWT Authentication (ALL routes)      │
│  - Verify user is logged in                     │
└─────────────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────────────┐
│  Layer 2: RBAC - Role-Based (MOST routes)      │
│  - Fast permission checks                       │
│  - Check user role and permissions             │
└─────────────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────────────┐
│  Layer 3: ABAC - Attribute-Based (FEW routes)  │
│  - Fine-grained, context-aware                 │
│  - Only for sensitive/complex scenarios        │
└─────────────────────────────────────────────────┘
```

### Example Implementation

```go
// PUBLIC - No auth
r.HandleFunc("/health", handlers.Health).Methods("GET")

// AUTHENTICATED - JWT only
api := r.PathPrefix("/api/v1").Subrouter()
api.Use(middleware.JWTMiddleware)

// SIMPLE RBAC - Most routes (fast)
api.Handle("/users",
    middleware.RequirePermission("read_users")(handler))

// HYBRID - Important data (RBAC + ABAC)
api.Handle("/financial-reports",
    middleware.RequireHybridAuth(
        "read_financial_reports",  // RBAC first
        "read",                     // ABAC action
        "financial_report",         // ABAC resource type
    )(handler))

// ABAC ONLY - Very sensitive (full policy evaluation)
api.Handle("/classified-data",
    middleware.RequireABACPolicy("read", "classified_data")(handler))
```

## Performance Optimization

### 1. Selective ABAC Usage
```go
// ✅ Good - ABAC only on sensitive routes
api.Handle("/regular-data",
    middleware.RequirePermission("read_data")(handler))

api.Handle("/sensitive-data",
    middleware.RequireABACPolicy("read", "sensitive_data")(handler))

// ❌ Bad - ABAC on everything
api.Use(middleware.RequireABACPolicy("*", "*")) // Don't do this!
```

### 2. Caching (Future Enhancement)
```go
// Cache policy decisions for same user+resource
// Cache user attributes (invalidate on change)
// Cache active policies
```

### 3. Lazy Evaluation
```go
// Only load resource attributes if policies need them
// Only evaluate policies that match the resource type
```

## How Policy Evaluation Works

### Step-by-Step Process

1. **Gather Attributes**
   ```go
   // User attributes from database
   userAttrs := {
       "user.department": "engineering",
       "user.role": "developer",
       "user.level": "senior",
       "user.clearance": "confidential"
   }

   // Resource attributes (if applicable)
   resourceAttrs := {
       "resource.classification": "confidential",
       "resource.owner_department": "engineering"
   }

   // Environment attributes
   envAttrs := {
       "environment.ip_address": "192.168.1.100",
       "environment.time": "2025-10-30T14:30:00Z",
       "environment.day_of_week": "Thursday"
   }
   ```

2. **Find Matching Policies**
   ```sql
   SELECT * FROM policies
   WHERE is_active = true
     AND (resource_type = 'report' OR resource_type = '*')
     AND (action = 'read' OR action = '*')
   ```

3. **Evaluate Each Policy**
   ```go
   for each policy {
       // Check conditions
       if policy.conditions.match(userAttrs, resourceAttrs, envAttrs) {
           if policy.effect == "deny" {
               return DENY  // Explicit deny wins
           }
           foundAllow = true
       }
   }

   // Default deny unless explicit allow
   return foundAllow ? ALLOW : DENY
   ```

4. **Make Decision**
   ```go
   decision := {
       Allowed: true/false,
       Reason: "Policy XYZ matched",
       Effect: "allow/deny",
       MatchedPolicies: [...]
   }
   ```

## Database Queries per Request

### RBAC Only (Current - Fast)
```
1 query: Load user with permissions
```

### ABAC (If Used - Slower)
```
1 query: Load user with permissions
1 query: Load user attributes
1 query: Load resource attributes (if resource specified)
1 query: Load active policies
1 query: Save evaluation log (optional)
---
4-5 queries total
```

## Configuration Examples

### Scenario 1: E-Commerce Application

```go
// Public routes - No auth
r.HandleFunc("/products", handlers.ListProducts).Methods("GET")

// Customer routes - RBAC
api.Handle("/orders",
    middleware.RequirePermission("create_order")(handler))

// Admin routes - RBAC
admin.Handle("/users",
    middleware.RequirePermission("manage_users")(handler))

// Sensitive operations - Hybrid RBAC + ABAC
admin.Handle("/refunds",
    middleware.RequireHybridAuth(
        "process_refund",      // Must have permission
        "approve",             // AND pass ABAC policies
        "refund",              // For refund resource
    )(handler))

// ABAC policy checks:
// - Refund amount (manager < $500, director < $5000)
// - Customer account status
// - Recent refund history
// - User's approval authority
```

### Scenario 2: Healthcare Application

```go
// Public info - No auth
r.HandleFunc("/clinic-hours", handlers.GetHours).Methods("GET")

// Staff access - RBAC
api.Handle("/appointments",
    middleware.RequirePermission("view_appointments")(handler))

// Patient records - ABAC (HIPAA compliance)
api.Handle("/patient-records/{id}",
    middleware.RequireABACPolicy("read", "patient_record")(handler))

// ABAC policy checks:
// - User is assigned to patient
// - User department matches patient department
// - Record access level vs user clearance
// - Audit trail requirements
// - Time-based access restrictions
```

### Scenario 3: Financial System

```go
// Market data - RBAC
api.Handle("/market-data",
    middleware.RequirePermission("read_market_data")(handler))

// Trading - Hybrid
api.Handle("/trades",
    middleware.RequireHybridAuth(
        "execute_trade",
        "create",
        "trade",
    )(handler))

// ABAC policy checks:
// - Trading hours
// - Account balance
// - Risk limits
// - Compliance rules
// - Regulatory requirements
```

## Migration from RBAC to ABAC

### Phase 1: Identify Candidates
Look for routes with:
- Complex conditional logic in handlers
- Multiple permission checks
- Context-dependent access
- Temporary access needs

### Phase 2: Create Policies
```go
// Instead of code in handler:
if user.Department == resource.Department &&
   user.Level >= resource.RequiredLevel &&
   businessHours() {
    // allow
}

// Create ABAC policy:
{
    "name": "Department-Level-Time Access",
    "effect": "allow",
    "conditions": {
        "user.department": {"equals": "resource.department"},
        "user.level": {"greaterThanOrEqual": "resource.required_level"},
        "environment.business_hours": {"equals": "true"}
    }
}
```

### Phase 3: Apply Middleware
```go
// Replace complex handler logic
api.Handle("/data",
    middleware.RequireABACPolicy("read", "data")(handler))
```

### Phase 4: Monitor Performance
- Track policy evaluation times
- Monitor database load
- Optimize as needed

## Best Practices

### ✅ DO

1. **Use ABAC selectively** - Only where needed
2. **Cache policy results** - When possible
3. **Index attributes** - For fast lookups
4. **Monitor performance** - Track evaluation times
5. **Log decisions** - For audit and debugging
6. **Test policies** - Before deploying
7. **Version policies** - Track changes

### ❌ DON'T

1. **Don't use ABAC everywhere** - RBAC is faster for simple cases
2. **Don't create overly complex policies** - Keep them maintainable
3. **Don't forget caching** - Attribute lookups are expensive
4. **Don't skip testing** - Policy bugs are hard to debug
5. **Don't ignore performance** - Monitor database impact

## Summary

| Aspect | Current State | If ABAC Used Everywhere |
|--------|---------------|------------------------|
| **Performance** | Fast ✅ | Slow ⚠️ |
| **Database Load** | Low ✅ | High ⚠️ |
| **Flexibility** | Basic ⚠️ | Advanced ✅ |
| **Complexity** | Simple ✅ | Complex ⚠️ |
| **Maintenance** | Easy ✅ | Moderate ⚠️ |

### Recommendation

**Use a hybrid approach:**
- 80% of routes: RBAC only (fast, simple)
- 15% of routes: Hybrid RBAC + ABAC (important data)
- 5% of routes: ABAC only (highly sensitive)

This gives you:
- ✅ Good performance for most requests
- ✅ Fine-grained control where needed
- ✅ Reasonable database load
- ✅ Maintainable codebase

Your current implementation is **optimal** - ABAC is available when needed but not enforced on every request, avoiding unnecessary performance overhead.

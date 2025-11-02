# Authorization Decision Tree

## Quick Decision Guide: When to Use Which Authorization?

```
START: New API Endpoint
    │
    ├─ Is it public? (health check, login, etc.)
    │   └─ YES → No middleware needed
    │
    ├─ Does it need authentication?
    │   └─ YES → Apply JWTMiddleware
    │       │
    │       ├─ Simple permission check? (user has role/permission)
    │       │   └─ YES → Use RequirePermission() [RBAC]
    │       │       │
    │       │       └─ DONE ✅ (80% of cases)
    │       │
    │       ├─ Complex conditions? (time, location, attributes)
    │       │   └─ YES → Consider ABAC
    │       │       │
    │       │       ├─ Still need basic permission?
    │       │       │   └─ YES → Use RequireHybridAuth() [RBAC + ABAC]
    │       │       │       │
    │       │       │       └─ DONE ✅ (15% of cases)
    │       │       │
    │       │       └─ No basic permission needed?
    │       │           └─ Use RequireABACPolicy() [ABAC only]
    │       │               │
    │       │               └─ DONE ✅ (5% of cases)
    │       │
    │       └─ Business vertical scoped?
    │           └─ YES → Use RequireBusinessPermission()
    │               │
    │               └─ DONE ✅
```

## Authorization Types Comparison

### 1. No Authorization (Public)
```go
r.HandleFunc("/health", handlers.Health).Methods("GET")
r.HandleFunc("/login", handlers.Login).Methods("POST")
```

**When to use:**
- Health checks
- Login/signup endpoints
- Public information
- Static assets

**Performance:** ⚡⚡⚡⚡⚡ (Fastest)

---

### 2. JWT Only (Authenticated)
```go
api.Use(middleware.JWTMiddleware)
api.HandleFunc("/profile", handlers.GetProfile).Methods("GET")
```

**When to use:**
- User's own data
- Any authenticated user can access
- No specific permissions needed

**Performance:** ⚡⚡⚡⚡ (Very Fast)

**Database Queries:** 0 (token validation only)

---

### 3. RBAC - Simple Permission
```go
middleware.RequirePermission("read_users")
```

**When to use:**
- Role-based access
- Simple yes/no permission
- Most common cases

**Performance:** ⚡⚡⚡ (Fast)

**Database Queries:** 1 (load user with permissions)

**Example:**
- "Can this user read users?" → Check permission → Allow/Deny

---

### 4. RBAC - Business Permission
```go
middleware.RequireBusinessPermission("read_reports")
```

**When to use:**
- Multi-tenant applications
- Business vertical scoping
- Department-specific access

**Performance:** ⚡⚡⚡ (Fast)

**Database Queries:** 1 (load user with business roles)

**Example:**
- "Can this user read reports in THIS business?" → Check → Allow/Deny

---

### 5. RBAC - Multiple Options
```go
middleware.RequireAnyPermission([]string{
    "create_reports",
    "create_materials"
})
```

**When to use:**
- User needs ANY of several permissions
- Flexible permission requirements

**Performance:** ⚡⚡⚡ (Fast)

**Database Queries:** 1

**Example:**
- "Can user upload file?" → Check if has create_reports OR create_materials

---

### 6. Hybrid - RBAC + ABAC
```go
middleware.RequireHybridAuth(
    "read_financial_reports",  // RBAC
    "read",                     // ABAC action
    "financial_report"          // ABAC resource
)
```

**When to use:**
- Need both permission AND conditions
- Important/sensitive data
- Compliance requirements

**Performance:** ⚡⚡ (Moderate)

**Database Queries:** 4-5
- User permissions (RBAC)
- User attributes (ABAC)
- Resource attributes (ABAC)
- Active policies (ABAC)

**Example:**
```
Step 1 (RBAC): Does user have "read_financial_reports" permission?
    ├─ NO → Deny ❌
    └─ YES → Continue to Step 2

Step 2 (ABAC): Evaluate policies
    - User department = Finance? ✅
    - Report classification ≤ User clearance? ✅
    - Business hours? ✅
    - All conditions met? ✅ → Allow ✅
```

---

### 7. ABAC Only - Full Policy Evaluation
```go
middleware.RequireABACPolicy("read", "classified_data")
```

**When to use:**
- Very sensitive data
- Complex conditional logic
- Dynamic access rules
- Compliance/regulatory requirements

**Performance:** ⚡ (Slower)

**Database Queries:** 4-5

**Example:**
```
Evaluate ALL active policies for "read" + "classified_data"

Policy 1: "Department Match"
    - user.department == resource.department? ✅

Policy 2: "Clearance Level"
    - user.clearance >= resource.classification? ✅

Policy 3: "Time Window"
    - current_time within business_hours? ✅

Policy 4: "Deny Contractors"
    - user.employment_type != "contractor"? ✅

All policies evaluated → Decision: ALLOW ✅
```

---

## Real-World Examples

### Example 1: Basic CRUD API

```go
// LIST - Anyone with read permission
api.Handle("/users",
    middleware.RequirePermission("read_users")(
        http.HandlerFunc(handlers.GetUsers))).Methods("GET")

// CREATE - Anyone with create permission
api.Handle("/users",
    middleware.RequirePermission("create_users")(
        http.HandlerFunc(handlers.CreateUser))).Methods("POST")

// UPDATE - Anyone with update permission
api.Handle("/users/{id}",
    middleware.RequirePermission("update_users")(
        http.HandlerFunc(handlers.UpdateUser))).Methods("PUT")

// DELETE - Anyone with delete permission
api.Handle("/users/{id}",
    middleware.RequirePermission("delete_users")(
        http.HandlerFunc(handlers.DeleteUser))).Methods("DELETE")
```

**Authorization:** RBAC only
**Performance:** Fast ⚡⚡⚡
**Use Case:** 80% of APIs

---

### Example 2: Multi-Tenant SaaS

```go
// Business-scoped reports
api.Handle("/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("read_reports")(
        http.HandlerFunc(handlers.GetReports))).Methods("GET")

// Site-scoped data
api.Handle("/business/{businessCode}/sites/{siteId}/data",
    middleware.RequireBusinessAccess()(
        middleware.RequireSiteAccess()(
            http.HandlerFunc(handlers.GetSiteData)))).Methods("GET")
```

**Authorization:** RBAC with business scoping
**Performance:** Fast ⚡⚡⚡
**Use Case:** Multi-tenant applications

---

### Example 3: Financial Application

```go
// View balance - Simple permission
api.Handle("/accounts/balance",
    middleware.RequirePermission("view_balance")(handler))

// Transfer money - Hybrid (permission + policies)
api.Handle("/transfers",
    middleware.RequireHybridAuth(
        "create_transfer",     // Must have permission
        "create",              // Action
        "transfer",            // Resource type
    )(handler))

// ABAC policies check:
// - Sufficient balance
// - Transfer limit not exceeded
// - Recipient account valid
// - Not in restricted countries
// - Business hours (for large amounts)
```

**Authorization:** Mixed RBAC + Hybrid
**Performance:** Moderate ⚡⚡
**Use Case:** Financial operations

---

### Example 4: Healthcare System

```go
// Staff directory - Simple RBAC
api.Handle("/staff",
    middleware.RequirePermission("view_staff")(handler))

// Patient records - ABAC only
api.Handle("/patients/{id}/records",
    middleware.RequireABACPolicy("read", "patient_record")(handler))

// ABAC policies check:
// - User assigned to patient
// - Correct department
// - Active shift
// - Emergency access override
// - Break-glass audit logging
```

**Authorization:** ABAC for sensitive data
**Performance:** Slower ⚡
**Use Case:** HIPAA compliance

---

## Performance Comparison

### Request Time Breakdown

#### RBAC Only
```
Total: ~5-10ms
├─ JWT Validation: 1-2ms
├─ DB Query (user+permissions): 3-5ms
├─ Permission Check: <1ms
└─ Handler: 2-5ms
```

#### Hybrid RBAC + ABAC
```
Total: ~50-150ms
├─ JWT Validation: 1-2ms
├─ DB Query (user+permissions): 3-5ms
├─ Permission Check: <1ms
├─ ABAC Evaluation:
│   ├─ Load user attributes: 5-10ms
│   ├─ Load resource attributes: 5-10ms
│   ├─ Load policies: 10-20ms
│   └─ Evaluate conditions: 5-15ms
└─ Handler: 20-80ms
```

#### ABAC Only
```
Total: ~45-140ms
├─ JWT Validation: 1-2ms
├─ ABAC Evaluation:
│   ├─ Load user attributes: 5-10ms
│   ├─ Load resource attributes: 5-10ms
│   ├─ Load policies: 10-20ms
│   └─ Evaluate conditions: 5-15ms
└─ Handler: 20-80ms
```

---

## Cost Analysis (Database Load)

### 1000 Requests/Second

| Authorization Type | DB Queries/sec | Impact |
|-------------------|----------------|---------|
| RBAC Only | 1,000 | Low ✅ |
| Hybrid RBAC+ABAC | 4,000-5,000 | High ⚠️ |
| ABAC Only | 4,000-5,000 | High ⚠️ |

### Recommendation
```
90% routes: RBAC → 900 req/s × 1 query = 900 queries/s
10% routes: ABAC → 100 req/s × 5 queries = 500 queries/s
────────────────────────────────────────────────────────
Total: 1,400 queries/s (manageable ✅)

vs.

100% routes: ABAC → 1000 req/s × 5 queries = 5,000 queries/s
(may need caching/optimization ⚠️)
```

---

## Decision Matrix

| Requirement | Use This |
|-------------|----------|
| Public access | No middleware |
| Just need authentication | JWTMiddleware only |
| Simple role check | RequirePermission() |
| Business-scoped | RequireBusinessPermission() |
| Multiple permission options | RequireAnyPermission() |
| Time-based access | ABAC |
| Location-based access | ABAC |
| Attribute-based rules | ABAC |
| Compliance requirements | Hybrid or ABAC |
| Dynamic conditions | ABAC |
| Temporary access | ABAC |
| Complex approval workflows | ABAC |
| Permission + Conditions | Hybrid (RBAC + ABAC) |

---

## Migration Path

### Current State (Good!)
```
99% RBAC → Fast, simple, works well ✅
1% ABAC infrastructure → Ready when needed ✅
```

### If Needed in Future
```
Step 1: Identify routes needing ABAC
    ├─ Complex conditional logic
    ├─ Compliance requirements
    └─ Dynamic access rules

Step 2: Create policies
    ├─ Define attributes
    ├─ Write policy conditions
    └─ Test thoroughly

Step 3: Apply middleware
    ├─ Start with one route
    ├─ Monitor performance
    └─ Expand gradually

Step 4: Optimize
    ├─ Add caching
    ├─ Index attributes
    └─ Tune queries
```

---

## Summary

### Your Current Setup ✅

```go
// ABAC routes for managing policies (using RBAC)
RegisterABACRoutes(api)

// ABAC middleware exists but not enforced globally
// This is OPTIMAL! 🎯
```

**Why it's optimal:**
- ✅ ABAC infrastructure ready when needed
- ✅ No performance overhead on regular routes
- ✅ Can enable ABAC selectively
- ✅ Flexibility to grow

### When to Add ABAC

Only add ABAC middleware when you have:
1. Complex conditional logic
2. Compliance requirements (HIPAA, SOX, etc.)
3. Dynamic access rules
4. Attribute-based decisions
5. Temporary/time-based access

### Golden Rule

**"Use the simplest authorization that meets your needs"**

```
If RBAC works → Use RBAC ✅
If you need complex conditions → Use ABAC ✅
If unsure → Start with RBAC, migrate to ABAC later ✅
```

Your system is well-designed with the flexibility to use both! 🚀

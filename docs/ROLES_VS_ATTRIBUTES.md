# User-Role Mapping vs User-Attribute Mapping

## Direct Answer: **NO, you DON'T need User-Attribute mapping!**

You already have User-Role mapping which handles your authorization needs. User-Attribute mapping is ONLY needed if you use ABAC policies.

## What You Currently Have ✅

### 1. User-Role Mapping (RBAC)

```go
// From models/user.go and business.go

type User struct {
    RoleID            *uuid.UUID         // Global role
    RoleModel         *Role              // → Permissions
    UserBusinessRoles []UserBusinessRole // → Business-specific roles
}

type UserBusinessRole struct {
    UserID         uuid.UUID
    BusinessRoleID uuid.UUID
    BusinessRole   BusinessRole  // → Permissions
    IsActive       bool
}
```

**What this gives you:**
```
User → Role → Permissions
User → BusinessRole → Permissions (per business vertical)
User → SiteAccess → Site-level permissions
```

**Example:**
```
John (User)
  ├─ Global Role: "manager"
  │    └─ Permissions: [read_users, create_users, read_reports]
  │
  ├─ Business Role: "coal_mining_manager" (in Coal Mining vertical)
  │    └─ Permissions: [read_reports, create_reports, approve_expenses]
  │
  └─ Site Access: [Site A, Site B]
       └─ Permissions: [read, create, update] per site
```

### 2. User-Attribute Mapping (ABAC - Available but OPTIONAL)

```go
// From models/attribute.go

type UserAttribute struct {
    UserID      uuid.UUID
    AttributeID uuid.UUID
    Value       string     // The actual attribute value
    IsActive    bool
    ValidUntil  *time.Time // Can expire
}
```

**What this gives you:**
```
User → Attributes (key-value pairs)
```

**Example:**
```
John (User)
  └─ Attributes:
       ├─ department: "engineering"
       ├─ clearance_level: "3"
       ├─ approval_limit: "50000"
       ├─ region: "north"
       └─ employment_type: "full-time"
```

## Key Differences

| Aspect | User-Role Mapping | User-Attribute Mapping |
|--------|-------------------|------------------------|
| **Purpose** | Authorization (what can user do?) | Context/Properties (who is the user?) |
| **Structure** | User → Role → Permissions | User → Attribute → Value |
| **Usage** | Permission checks | Policy conditions |
| **Example** | User has "read_users" permission | User has clearance_level = 3 |
| **When checked** | Every protected route (RBAC) | Only when ABAC middleware used |
| **Performance** | Fast (1 query) | Slower (multiple queries) |
| **Complexity** | Simple | Complex |
| **Required?** | **YES** ✅ | **NO** ❌ (unless using ABAC) |

## How They Work Together

### Scenario 1: RBAC Only (Your Current Setup - Recommended) ✅

```go
// Route definition
api.Handle("/users",
    middleware.RequirePermission("read_users")(handler))

// What happens:
// 1. Load user with roles: User → Role → Permissions
// 2. Check if "read_users" in permissions
// 3. Allow or deny

// Uses: User-Role mapping ✅
// Uses: User-Attribute mapping ❌ (not needed)
```

### Scenario 2: ABAC Only (Complex Conditions)

```go
// Route definition
api.Handle("/sensitive-data",
    middleware.RequireABACPolicy("read", "sensitive_data")(handler))

// What happens:
// 1. Load user attributes: User → Attributes (department, clearance, etc.)
// 2. Load resource attributes: Resource → Attributes (classification, etc.)
// 3. Evaluate policies with conditions
// 4. Allow or deny

// Uses: User-Role mapping ❌ (not needed)
// Uses: User-Attribute mapping ✅
```

### Scenario 3: Hybrid RBAC + ABAC (Best of Both)

```go
// Route definition
api.Handle("/financial-reports",
    middleware.RequireHybridAuth(
        "read_financial_reports",  // RBAC check
        "read",                     // ABAC action
        "financial_report",         // ABAC resource
    )(handler))

// What happens:
// 1. RBAC: User → Role → Check "read_financial_reports" permission
// 2. If passed, ABAC: User → Attributes → Evaluate policies
// 3. Allow or deny

// Uses: User-Role mapping ✅
// Uses: User-Attribute mapping ✅
```

## Real-World Examples

### Example 1: Simple Authorization (RBAC - No Attributes Needed)

**Requirement:** Only admins can manage users

```go
// Using Roles (simple)
api.Handle("/users",
    middleware.RequirePermission("manage_users")(handler))

// User-Role mapping:
Admin Role → [manage_users, ...]
Manager Role → [read_users, ...]
Employee Role → [read_profile, ...]
```

**No attributes needed!** ✅

---

### Example 2: Business Scoping (RBAC - No Attributes Needed)

**Requirement:** User can only access their business vertical

```go
// Using Business Roles
api.Handle("/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("read_reports")(handler))

// User-BusinessRole mapping:
John → Coal Mining Business Role → [read_reports, create_reports]
Jane → Power Business Role → [read_reports]
```

**No attributes needed!** ✅

---

### Example 3: Approval Limits (ABAC - Attributes Needed)

**Requirement:**
- Supervisor can approve up to $1,000
- Manager can approve up to $10,000
- Director can approve up to $100,000

```go
// Using Attributes + Policies
api.Handle("/expenses/{id}/approve",
    middleware.RequireHybridAuth(
        "approve_expense",  // Must have base permission (RBAC)
        "approve",          // Action
        "expense",          // Resource
    )(handler))

// User-Attribute mapping:
Supervisor → approval_limit: "1000"
Manager → approval_limit: "10000"
Director → approval_limit: "100000"

// ABAC Policy:
{
  "condition": {
    "user.approval_limit": {
      "greaterThanOrEqual": "resource.amount"
    }
  }
}
```

**Attributes needed!** ✅ But only if you have this requirement

---

### Example 4: Department Matching (ABAC - Attributes Needed)

**Requirement:** Users can only view reports from their department

```go
// Using Attributes + Policies
api.Handle("/reports/{id}",
    middleware.RequireHybridAuth(
        "read_reports",     // Base permission
        "read",             // Action
        "report",           // Resource
    )(handler))

// User-Attribute mapping:
John → department: "engineering"
Jane → department: "finance"

// Resource-Attribute mapping:
Report A → department: "engineering"
Report B → department: "finance"

// ABAC Policy:
{
  "condition": {
    "user.department": {
      "equals": "resource.department"
    }
  }
}
```

**Attributes needed!** ✅ But only if you have this requirement

---

### Example 5: Geofencing (ABAC - Attributes Needed)

**Requirement:** User must be within site geofence to check in

```go
// Using Attributes + Policies
api.Handle("/sites/{id}/checkin",
    middleware.RequireABACPolicy("checkin", "site")(handler))

// User-Attribute mapping:
John → current_location: "lat:12.34, lng:56.78"

// Resource-Attribute mapping:
Site A → geofence: "POLYGON(...)"

// ABAC Policy:
{
  "condition": {
    "user.current_location": {
      "within": "resource.geofence"
    }
  }
}
```

**Attributes needed!** ✅ If you implement geofence checkin

---

## When Do You Need Each?

### ✅ Always Need: User-Role Mapping

**Required for:**
- All RBAC authorization
- Permission checks
- Role-based access control
- Business scoping
- Site access

**Examples:**
- Can user read reports?
- Can user manage users?
- Can user access this business?
- Can user create sites?

**Performance:** Fast (1 DB query)

---

### ❓ Maybe Need: User-Attribute Mapping

**Required ONLY for:**
- ABAC policies with user attributes
- Dynamic attribute-based conditions
- Complex business rules

**Examples:**
- User's approval limit vs expense amount
- User's department vs resource department
- User's clearance level vs data classification
- User's location vs site geofence

**Performance:** Slower (4-5 DB queries)

**Use when:** You have complex, dynamic, context-based rules

---

## Database Schema Comparison

### User-Role Mapping (What You Have)

```sql
-- Users table
users
  ├─ id
  ├─ name
  ├─ role_id → roles (global role)

-- Roles table
roles
  ├─ id
  ├─ name (admin, manager, employee)

-- Role-Permission mapping
role_permissions
  ├─ role_id → roles
  ├─ permission_id → permissions

-- Permissions table
permissions
  ├─ id
  ├─ name (read_users, create_reports)

-- Business roles
user_business_roles
  ├─ user_id → users
  ├─ business_role_id → business_roles
  ├─ is_active

-- Site access
user_site_access
  ├─ user_id → users
  ├─ site_id → sites
  ├─ can_read, can_create, can_update, can_delete
```

**Query to check permission:**
```sql
SELECT 1 FROM users
JOIN roles ON users.role_id = roles.id
JOIN role_permissions ON roles.id = role_permissions.role_id
JOIN permissions ON role_permissions.permission_id = permissions.id
WHERE users.id = ? AND permissions.name = ?;
```

**Fast!** ✅

---

### User-Attribute Mapping (Optional - ABAC Only)

```sql
-- Attributes table (defines what attributes exist)
attributes
  ├─ id
  ├─ name (department, clearance_level, approval_limit)
  ├─ type (user, resource, environment)
  ├─ data_type (string, integer, boolean)

-- User-Attribute mapping (stores values)
user_attributes
  ├─ user_id → users
  ├─ attribute_id → attributes
  ├─ value (the actual value)
  ├─ is_active
  ├─ valid_until (can expire)

-- Resource-Attribute mapping
resource_attributes
  ├─ resource_type (report, expense, site)
  ├─ resource_id
  ├─ attribute_id → attributes
  ├─ value
```

**Query to get user attributes:**
```sql
SELECT a.name, ua.value
FROM user_attributes ua
JOIN attributes a ON ua.attribute_id = a.id
WHERE ua.user_id = ?
  AND ua.is_active = true
  AND (ua.valid_until IS NULL OR ua.valid_until > NOW());
```

**Multiple queries needed!** ⚠️

---

## Decision Guide

### Should You Add User-Attribute Mapping?

**Ask yourself these questions:**

1. **Do you have approval amount limits based on user attributes?**
   - No → Don't need attributes ✅
   - Yes → Need attributes ⚠️

2. **Do you have data classification levels to match against user clearance?**
   - No → Don't need attributes ✅
   - Yes → Need attributes ⚠️

3. **Do you need to match user properties with resource properties?**
   (e.g., user.department = resource.department)
   - No → Don't need attributes ✅
   - Yes → Need attributes ⚠️

4. **Do you have location-based/geofencing requirements?**
   - No → Don't need attributes ✅
   - Yes → Need attributes ⚠️ (you have geofencing!)

5. **Do you have time-bound or temporary access?**
   - No → Don't need attributes ✅
   - Yes → Need attributes ⚠️

6. **Do you have complex compliance requirements?**
   - No → Don't need attributes ✅
   - Yes → Need attributes ⚠️

### If you answered "No" to all → Stick with User-Role mapping! ✅

### If you answered "Yes" to 1+ → Consider User-Attribute mapping ⚠️

---

## Recommendation for Your Project

### Current State: ✅ PERFECT!

You have **User-Role mapping** which handles:
- ✅ Global permissions
- ✅ Business-scoped permissions
- ✅ Site-level access
- ✅ Role hierarchy
- ✅ Multi-tenant support

This is **excellent** and handles 95% of authorization needs!

---

### Future (If Needed): Add User-Attributes Selectively

**Only add User-Attribute mapping if you need:**

1. **Geofencing** (you have this feature!)
   ```go
   // User attribute: current_location
   // Site attribute: geofence polygon
   // Policy: location within geofence
   ```

2. **Approval Workflows** (if you have amount-based limits)
   ```go
   // User attribute: approval_limit
   // Expense attribute: amount
   // Policy: approval_limit >= amount
   ```

3. **Data Classification** (if you have sensitivity levels)
   ```go
   // User attribute: clearance_level
   // Report attribute: classification
   // Policy: clearance_level >= classification
   ```

---

## Summary Table

| Feature | User-Role Mapping | User-Attribute Mapping |
|---------|-------------------|------------------------|
| **What it does** | Defines what user can DO | Defines WHO the user IS |
| **Used for** | Permissions | ABAC policy conditions |
| **Required?** | YES ✅ | NO ❌ (unless using ABAC) |
| **Performance** | Fast ⚡⚡⚡ | Slower ⚡ |
| **Complexity** | Simple 🟢 | Complex 🔴 |
| **Your current state** | Implemented ✅ | Implemented but not used ✅ |
| **Recommendation** | **Keep using!** ✅ | **Only use if needed!** ⚠️ |

---

## Final Answer

### **Question:** "Do we need user and attributes mapping as we have user and role mapping?"

### **Answer:**

**NO, you DON'T need User-Attribute mapping!** ❌

**Reason:**
- ✅ User-Role mapping handles all your authorization needs
- ✅ User-Attribute mapping is ONLY for ABAC policies
- ✅ You're not using ABAC policies currently
- ✅ RBAC (roles + permissions) is sufficient for 95% of cases

**When you WOULD need it:**
- ⚠️ Only if you implement ABAC policies with attribute-based conditions
- ⚠️ Examples: approval limits, geofencing, data classification

**Current recommendation:**
- ✅ Keep your User-Role mapping (it's excellent!)
- ✅ User-Attribute mapping infrastructure exists if needed
- ✅ Don't add attributes unless you have specific ABAC requirements

**Your system is already optimal!** 🎉

---

## Quick Decision Flowchart

```
Do you need authorization?
  ↓
  YES → Use User-Role mapping ✅
        (what you currently have)

  ↓

Do you need complex attribute-based conditions?
  ├─ NO → Done! User-Role is sufficient ✅
  │
  └─ YES → Add User-Attribute mapping ⚠️
            (optional ABAC layer)

Examples of "complex conditions":
- Approval based on amount AND user limit
- Access based on user department = resource department
- Location-based access (geofencing)
- Clearance level vs data classification
- Time-bound temporary access
```

**For your project: User-Role mapping is all you need!** ✅

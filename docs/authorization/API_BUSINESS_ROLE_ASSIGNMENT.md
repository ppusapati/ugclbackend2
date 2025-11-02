# Business Role Assignment API Guide

## Overview

This guide shows how to assign users to business roles (mapping users to business_roles via the `user_business_roles` junction table).

## Database Structure

```
users
  └─> user_business_roles (junction table)
       ├─ user_id (FK → users.id)
       ├─ business_role_id (FK → business_roles.id)
       ├─ is_active
       ├─ assigned_at
       └─ assigned_by

business_roles
  ├─ id
  ├─ name
  ├─ display_name
  ├─ business_vertical_id (FK → business_verticals.id)
  └─> business_role_permissions (junction table)
       ├─ business_role_id (FK → business_roles.id)
       └─ permission_id (FK → permissions.id)
```

---

## API Endpoints

### 1. Assign User to Business Role

**Endpoint:** `POST /api/v1/business/{businessCode}/users/assign`

**Description:** Assigns a user to a specific business role within a business vertical.

**Headers:**
```json
{
  "Authorization": "Bearer YOUR_JWT_TOKEN",
  "Content-Type": "application/json"
}
```

**Request Body:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "business_role_id": "8dd00b3d-046b-4210-91ba-9af00a8d7a2f"
}
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/api/v1/business/WATER/users/assign \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "business_role_id": "8dd00b3d-046b-4210-91ba-9af00a8d7a2f"
  }'
```

**Success Response (200 OK):**
```json
{
  "message": "user assigned to role successfully"
}
```

**Error Responses:**

- **400 Bad Request:**
  ```json
  "invalid business identifier"
  "invalid JSON"
  "invalid user ID"
  "invalid role ID"
  ```

- **404 Not Found:**
  ```json
  "user not found"
  "role not found in this business"
  ```

- **409 Conflict:**
  ```json
  "user already has this role"
  ```

---

### 2. Get Business Roles (to find role IDs)

**Endpoint:** `GET /api/v1/business/{businessCode}/roles`

**Description:** Lists all roles available in a business vertical (to get the `business_role_id` for assignment).

**Example Request:**
```bash
curl -X GET http://localhost:8080/api/v1/business/WATER/roles \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response:**
```json
[
  {
    "id": "8dd00b3d-046b-4210-91ba-9af00a8d7a2f",
    "name": "Water_Admin",
    "display_name": "Water Works Admin",
    "description": "Full control within Water vertical",
    "level": 1,
    "business_vertical_id": "d8346d46-6604-431c-b14a-9617218b7c5b",
    "business_vertical_name": "Water Works",
    "permissions": [
      {
        "id": "91b8c7e8-99a5-4998-9dd8-2964685095be",
        "name": "project:read",
        "description": "View project details",
        "resource": "project",
        "action": "read"
      },
      {
        "id": "e65247fc-534d-44bd-921b-45299059d84a",
        "name": "planning:update",
        "description": "Update plans",
        "resource": "planning",
        "action": "update"
      }
      // ... more permissions
    ],
    "user_count": 5
  },
  {
    "id": "b772897d-17a2-4af1-9b3e-737198eb6c54",
    "name": "Engineer",
    "display_name": "Water Engineer",
    "description": "Execute tasks, manage water system & inventory",
    "level": 4,
    "business_vertical_id": "d8346d46-6604-431c-b14a-9617218b7c5b",
    "business_vertical_name": "Water Works",
    "permissions": [/* ... */],
    "user_count": 12
  }
]
```

---

### 3. Get Users in Business Vertical

**Endpoint:** `GET /api/v1/business/{businessCode}/users`

**Description:** Lists all users assigned to roles in this business vertical.

**Example Request:**
```bash
curl -X GET http://localhost:8080/api/v1/business/WATER/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "John Doe",
    "email": "john.doe@company.com",
    "phone": "+1234567890",
    "roles": [
      {
        "id": "8dd00b3d-046b-4210-91ba-9af00a8d7a2f",
        "name": "Water_Admin",
        "display_name": "Water Works Admin",
        "level": 1,
        "assigned_at": "2025-10-18T10:30:00Z"
      }
    ]
  },
  {
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "name": "Jane Smith",
    "email": "jane.smith@company.com",
    "phone": "+1234567891",
    "roles": [
      {
        "id": "b772897d-17a2-4af1-9b3e-737198eb6c54",
        "name": "Engineer",
        "display_name": "Water Engineer",
        "level": 4,
        "assigned_at": "2025-10-15T14:20:00Z"
      },
      {
        "id": "b289d6b1-2d59-4afb-8daa-40032fd78368",
        "name": "Supervisor",
        "display_name": "Water Supervisor",
        "level": 4,
        "assigned_at": "2025-10-16T09:00:00Z"
      }
    ]
  }
]
```

---

### 4. Get User's Business Access

**Endpoint:** `GET /api/v1/user/business-access`

**Description:** Returns all business verticals the current user can access with their roles.

**Example Request:**
```bash
curl -X GET http://localhost:8080/api/v1/user/business-access \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_name": "John Doe",
  "user_role": "user",
  "is_super_admin": false,
  "accessible_businesses": [
    {
      "id": "d8346d46-6604-431c-b14a-9617218b7c5b",
      "name": "Water Works",
      "code": "WATER",
      "description": "Water supply and distribution management",
      "access_type": "business_role",
      "roles": ["Water Works Admin", "Water Supervisor"],
      "permissions": []
    },
    {
      "id": "2cfcaa43-92a4-41cb-a2fc-d22e7b73be68",
      "name": "Solar Works",
      "code": "SOLAR",
      "description": "Solar energy generation and maintenance operations",
      "access_type": "business_role",
      "roles": ["Solar Sr Engineer"],
      "permissions": []
    }
  ],
  "total_businesses": 2
}
```

---

## Complete Workflow Example

### Scenario: Assign a new user to the Water Works as an Engineer

#### Step 1: Get available business roles for Water Works

```bash
curl -X GET http://localhost:8080/api/v1/business/WATER/roles \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

Look for the role you want in the response:
```json
{
  "id": "b772897d-17a2-4af1-9b3e-737198eb6c54",  // ← This is the business_role_id
  "name": "Engineer",
  "display_name": "Water Engineer"
}
```

#### Step 2: Get the user ID (or create a new user first)

Assume you have a user with ID: `550e8400-e29b-41d4-a716-446655440000`

#### Step 3: Assign the user to the role

```bash
curl -X POST http://localhost:8080/api/v1/business/WATER/users/assign \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "business_role_id": "b772897d-17a2-4af1-9b3e-737198eb6c54"
  }'
```

#### Step 4: Verify the assignment

```bash
curl -X GET http://localhost:8080/api/v1/business/WATER/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

You should see the user in the response with their assigned role.

---

## Multiple Role Assignment

### Scenario: Assign user to multiple roles in the same business

You can assign the same user to multiple roles by calling the assign endpoint multiple times:

```bash
# Assign as Water Engineer
curl -X POST http://localhost:8080/api/v1/business/WATER/users/assign \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "business_role_id": "b772897d-17a2-4af1-9b3e-737198eb6c54"
  }'

# Also assign as Water Supervisor
curl -X POST http://localhost:8080/api/v1/business/WATER/users/assign \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "business_role_id": "b289d6b1-2d59-4afb-8daa-40032fd78368"
  }'
```

The user will now have both roles in the Water Works business vertical.

---

## Cross-Business Assignment

### Scenario: Assign user to roles in multiple business verticals

```bash
# Assign to Water Works as Engineer
curl -X POST http://localhost:8080/api/v1/business/WATER/users/assign \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "business_role_id": "b772897d-17a2-4af1-9b3e-737198eb6c54"
  }'

# Assign to Solar Works as Sr Engineer
curl -X POST http://localhost:8080/api/v1/business/SOLAR/users/assign \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "business_role_id": "b08e8a3a-72ae-49d3-9f90-43bb5c2d01b4"
  }'
```

Now the user has roles in both Water Works AND Solar Works!

---

## Database Queries (For Verification)

### Check user_business_roles table

```sql
-- See all role assignments for a user
SELECT
    u.name as user_name,
    br.display_name as role_name,
    bv.name as business_name,
    ubr.assigned_at,
    ubr.is_active
FROM user_business_roles ubr
JOIN users u ON ubr.user_id = u.id
JOIN business_roles br ON ubr.business_role_id = br.id
JOIN business_verticals bv ON br.business_vertical_id = bv.id
WHERE u.id = '550e8400-e29b-41d4-a716-446655440000'
  AND ubr.is_active = true;
```

### Get permissions for a user in a specific business

```sql
-- Get all permissions a user has in Water Works
SELECT DISTINCT
    p.name as permission_name,
    p.description,
    p.resource,
    p.action
FROM user_business_roles ubr
JOIN business_roles br ON ubr.business_role_id = br.id
JOIN business_verticals bv ON br.business_vertical_id = bv.id
JOIN business_role_permissions brp ON br.id = brp.business_role_id
JOIN permissions p ON brp.permission_id = p.id
WHERE ubr.user_id = '550e8400-e29b-41d4-a716-446655440000'
  AND bv.code = 'WATER'
  AND ubr.is_active = true;
```

---

## JSON Schema Reference

### AssignUserRoleRequest

```json
{
  "type": "object",
  "required": ["user_id", "business_role_id"],
  "properties": {
    "user_id": {
      "type": "string",
      "format": "uuid",
      "description": "The UUID of the user to assign"
    },
    "business_role_id": {
      "type": "string",
      "format": "uuid",
      "description": "The UUID of the business role to assign"
    }
  }
}
```

### Example Valid Payloads

```json
// Minimal assignment
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "business_role_id": "8dd00b3d-046b-4210-91ba-9af00a8d7a2f"
}
```

---

## Common Issues & Solutions

### Issue 1: "user already has this role"

**Cause:** The user is already assigned to this role and it's active.

**Solution:** If you need to reassign, first check if the assignment exists but is inactive. The API will automatically reactivate it.

### Issue 2: "role not found in this business"

**Cause:** The `business_role_id` doesn't belong to the business vertical specified in the URL.

**Solution:** Make sure the business role ID matches a role in that specific business vertical. Use `GET /api/v1/business/{code}/roles` to get the correct role IDs.

### Issue 3: Authorization errors

**Cause:** Current user doesn't have permission to assign roles.

**Solution:** Ensure the logged-in user has `business_admin` permission or is a super admin.

---

## Testing with Postman

### Collection Setup

1. **Create Environment Variables:**
   - `baseUrl`: `http://localhost:8080`
   - `token`: Your JWT token
   - `userId`: User ID to assign
   - `businessCode`: Business code (WATER, SOLAR, etc.)

2. **Request Collection:**

**A. Get Roles:**
```
GET {{baseUrl}}/api/v1/business/{{businessCode}}/roles
Headers:
  Authorization: Bearer {{token}}
```

**B. Assign User:**
```
POST {{baseUrl}}/api/v1/business/{{businessCode}}/users/assign
Headers:
  Authorization: Bearer {{token}}
  Content-Type: application/json
Body (raw JSON):
{
  "user_id": "{{userId}}",
  "business_role_id": "PASTE_ROLE_ID_HERE"
}
```

**C. Verify Assignment:**
```
GET {{baseUrl}}/api/v1/business/{{businessCode}}/users
Headers:
  Authorization: Bearer {{token}}
```

---

## Summary

### Quick Reference

| Action | Method | Endpoint | Body |
|--------|--------|----------|------|
| Assign user to role | POST | `/api/v1/business/{code}/users/assign` | `{"user_id": "...", "business_role_id": "..."}` |
| List business roles | GET | `/api/v1/business/{code}/roles` | N/A |
| List business users | GET | `/api/v1/business/{code}/users` | N/A |
| Get user's access | GET | `/api/v1/user/business-access` | N/A |

### Data Flow

```
1. Client → POST /api/v1/business/WATER/users/assign
            {user_id, business_role_id}

2. Handler validates:
   - User exists ✓
   - Business role exists in WATER ✓
   - Not already assigned ✓

3. Creates UserBusinessRole record:
   user_business_roles {
     user_id: "550e8400-...",
     business_role_id: "8dd00b3d-...",
     is_active: true,
     assigned_at: NOW(),
     assigned_by: current_user_id
   }

4. User can now access WATER business vertical
   with permissions from the assigned role
```

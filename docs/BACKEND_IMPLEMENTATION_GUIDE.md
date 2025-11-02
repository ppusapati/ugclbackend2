# Backend Implementation Guide - Unified Form System

## ✅ What Has Been Implemented

### Files Created

1. **Migration**
   - `migrations/000011_create_unified_form_config.up.sql` - Creates `modules` and `app_forms` tables
   - `migrations/000011_create_unified_form_config.down.sql` - Rollback script

2. **Models**
   - `models/app_form.go` - Complete models for `Module` and `AppForm`

3. **Handlers**
   - `handlers/app_forms.go` - All API endpoints for forms

4. **Routes**
   - Updated `routes/business_routes.go` with new endpoints

5. **Example Form**
   - `form_definitions/water.json` - Example JSON form definition

## API Endpoints Available

### User Endpoints (Mobile App)

#### 1. Get Forms for Vertical
```
GET /api/v1/business/{vertical}/forms
Authorization: Bearer <token>
X-API-KEY: <mobile_app_key>
```

**Example Request:**
```bash
curl -X GET "http://localhost:8080/api/v1/business/SOLAR/forms" \
  -H "Authorization: Bearer eyJhbGc..." \
  -H "X-API-KEY: your-mobile-app-key"
```

**Example Response:**
```json
{
  "forms": [
    {
      "code": "hr_nmr",
      "title": "NMR Form",
      "description": "Report daily worker deployment",
      "module": "hr",
      "route": "/hr/nmr",
      "icon": "supervised_user_circle",
      "required_permission": "hr:create",
      "accessible_verticals": ["WATER", "SOLAR", "HO"],
      "display_order": 1
    },
    {
      "code": "vehicle_nmr",
      "title": "NMR Vehicle",
      "description": "Record daily vehicle deployment",
      "module": "vehicle",
      "route": "/vehicles/nmrVehicle",
      "icon": "car_repair",
      "required_permission": "hr:create",
      "accessible_verticals": ["WATER", "SOLAR", "HO"],
      "display_order": 1
    }
  ],
  "modules": {
    "hr": [
      {
        "code": "hr_nmr",
        "title": "NMR Form",
        ...
      }
    ],
    "vehicle": [
      {
        "code": "vehicle_nmr",
        "title": "NMR Vehicle",
        ...
      }
    ]
  }
}
```

#### 2. Get Specific Form with Schema
```
GET /api/v1/business/{vertical}/forms/{code}
Authorization: Bearer <token>
```

**Example:**
```bash
curl -X GET "http://localhost:8080/api/v1/business/WATER/forms/water" \
  -H "Authorization: Bearer eyJhbGc..."
```

**Response includes full form schema:**
```json
{
  "code": "water",
  "title": "Water",
  "description": "Record daily supply of water",
  "module": "project",
  "route": "/projects/water",
  "icon": "water_drop",
  "form_schema": {
    "type": "multi_step",
    "steps": [...]
  },
  "steps": [...],
  ...
}
```

#### 3. Get All Modules
```
GET /api/v1/modules
Authorization: Bearer <token>
```

**Response:**
```json
{
  "modules": [
    {
      "id": "uuid",
      "code": "project",
      "name": "Projects",
      "description": "Project management and execution",
      "icon": "work",
      "route": "/projects",
      "display_order": 1,
      "is_active": true
    },
    ...
  ],
  "count": 5
}
```

### Admin Endpoints

#### 4. Get All Forms (Admin)
```
GET /api/v1/admin/app-forms
Authorization: Bearer <admin-token>
Permission Required: admin_all
```

#### 5. Create New Form (Admin)
```
POST /api/v1/admin/app-forms
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "code": "new_form",
  "title": "New Form",
  "description": "Description",
  "module_id": "module-uuid",
  "route": "/projects/new",
  "icon": "new_icon",
  "required_permission": "project:create",
  "accessible_verticals": ["WATER", "HO"],
  "display_order": 10
}
```

#### 6. Update Form Vertical Access (Admin)
```
POST /api/v1/admin/app-forms/{formCode}/verticals
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "vertical_codes": ["WATER", "SOLAR", "HO"]
}
```

**Example:**
```bash
curl -X POST "http://localhost:8080/api/v1/admin/app-forms/water/verticals" \
  -H "Authorization: Bearer admin-token" \
  -H "Content-Type: application/json" \
  -d '{"vertical_codes": ["WATER", "HO"]}'
```

## Implementation Steps

### Step 1: Run Migration

```bash
cd backend/v1

# Check database connection
psql "postgresql://user:password@localhost:5432/ugcl"

# Run migration
migrate -path migrations -database "postgresql://user:password@localhost:5432/ugcl?sslmode=disable" up

# Verify tables created
psql -c "\dt modules"
psql -c "\dt app_forms"

# Check data
psql -c "SELECT code, name FROM modules ORDER BY display_order;"
psql -c "SELECT code, title, accessible_verticals FROM app_forms ORDER BY display_order;"
```

**Expected Output:**
```
modules table created ✓
app_forms table created ✓

5 modules seeded:
- project
- hr
- finance
- inventory
- vehicle

15 forms seeded with vertical access configured
```

### Step 2: Restart Backend

```bash
cd backend/v1

# If using go run
go run main.go

# If using compiled binary
./ugcl-backend

# If using systemd
sudo systemctl restart ugcl-backend
```

### Step 3: Test API Endpoints

#### Test 1: Get Forms for WATER Vertical
```bash
# Get auth token first
TOKEN=$(curl -X POST "http://localhost:8080/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@sreeugcl.com","password":"your-password"}' \
  | jq -r '.access_token')

# Get forms for WATER
curl -X GET "http://localhost:8080/api/v1/business/WATER/forms" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-API-KEY: your-mobile-app-key" \
  | jq '.'
```

**Expected:** All 15 forms (WATER has access to everything)

#### Test 2: Get Forms for SOLAR Vertical
```bash
curl -X GET "http://localhost:8080/api/v1/business/SOLAR/forms" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-API-KEY: your-mobile-app-key" \
  | jq '.'
```

**Expected:** Only 3 forms (hr_nmr, vehicle_nmr, vehicle_log)

#### Test 3: Get Forms for CONTRACTORS Vertical
```bash
curl -X GET "http://localhost:8080/api/v1/business/CONTRACTORS/forms" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-API-KEY: your-mobile-app-key" \
  | jq '.'
```

**Expected:** Only 3 forms (contractor, site_dpr, tasks)

#### Test 4: Get Specific Form
```bash
curl -X GET "http://localhost:8080/api/v1/business/WATER/forms/water" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-API-KEY: your-mobile-app-key" \
  | jq '.'
```

**Expected:** Full water form details

#### Test 5: Get Modules
```bash
curl -X GET "http://localhost:8080/api/v1/modules" \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.'
```

**Expected:** All 5 modules (project, hr, finance, inventory, vehicle)

### Step 4: Verify Data in Database

```sql
-- Check modules
SELECT code, name, display_order FROM modules ORDER BY display_order;

-- Check forms with vertical access
SELECT
    code,
    title,
    accessible_verticals,
    (SELECT code FROM modules WHERE id = app_forms.module_id) as module
FROM app_forms
ORDER BY module_id, display_order;

-- Check forms for SOLAR vertical
SELECT code, title
FROM app_forms
WHERE accessible_verticals @> '["SOLAR"]'::jsonb;

-- Check forms by module
SELECT
    m.name as module,
    COUNT(f.id) as form_count
FROM modules m
LEFT JOIN app_forms f ON f.module_id = m.id
GROUP BY m.id, m.name
ORDER BY m.display_order;
```

## Adding New Forms

### Method 1: Via SQL

```sql
-- Get module ID
SELECT id FROM modules WHERE code = 'project';

-- Insert new form
INSERT INTO app_forms (
    code, title, description, module_id, route, icon,
    required_permission, accessible_verticals, display_order
)
VALUES (
    'solar_panel',
    'Solar Panel Inspection',
    'Record solar panel inspection details',
    (SELECT id FROM modules WHERE code = 'project'),
    '/projects/solar-panel',
    'solar_power',
    'project:create',
    '["SOLAR", "HO"]'::jsonb,
    8
);
```

### Method 2: Via API (Admin)

```bash
curl -X POST "http://localhost:8080/api/v1/admin/app-forms" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "solar_panel",
    "title": "Solar Panel Inspection",
    "description": "Record solar panel inspection details",
    "module_id": "project-module-uuid",
    "route": "/projects/solar-panel",
    "icon": "solar_power",
    "required_permission": "project:create",
    "accessible_verticals": ["SOLAR", "HO"],
    "display_order": 8
  }'
```

## Updating Vertical Access

### Make a form accessible in more verticals:

```bash
curl -X POST "http://localhost:8080/api/v1/admin/app-forms/water/verticals" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "vertical_codes": ["WATER", "SOLAR", "HO"]
  }'
```

Or via SQL:
```sql
UPDATE app_forms
SET accessible_verticals = '["WATER", "SOLAR", "HO"]'::jsonb
WHERE code = 'water';
```

## Adding Form Schema (JSON Definition)

```sql
UPDATE app_forms
SET form_schema = '{
  "type": "multi_step",
  "steps": [
    {
      "id": "basic_info",
      "title": "Basic Information",
      "fields": [
        {
          "id": "site",
          "type": "dropdown",
          "label": "Site Name",
          "required": true,
          "dataSource": "api",
          "apiEndpoint": "business/{vertical}/sites/my-access"
        }
      ]
    }
  ]
}'::jsonb
WHERE code = 'water';
```

## Troubleshooting

### Issue 1: Migration Fails
```bash
# Check current version
migrate -path migrations -database "postgresql://..." version

# Force to specific version
migrate -path migrations -database "postgresql://..." force 10

# Try again
migrate -path migrations -database "postgresql://..." up
```

### Issue 2: Forms Not Appearing
```sql
-- Check if forms exist
SELECT COUNT(*) FROM app_forms WHERE is_active = true;

-- Check vertical access
SELECT code, accessible_verticals FROM app_forms;

-- Check if vertical code matches
SELECT DISTINCT jsonb_array_elements_text(accessible_verticals) as vertical
FROM app_forms;
```

### Issue 3: Permission Denied
```sql
-- Check user permissions
SELECT permissions FROM users WHERE email = 'your-email';

-- Check form required permission
SELECT code, required_permission FROM app_forms WHERE code = 'form-code';
```

## Performance Optimization

### Add GIN Index for JSONB Queries
```sql
CREATE INDEX idx_app_forms_verticals_gin ON app_forms USING GIN(accessible_verticals);
```

### Query Performance Test
```sql
EXPLAIN ANALYZE
SELECT * FROM app_forms
WHERE accessible_verticals @> '["SOLAR"]'::jsonb
  AND is_active = true;
```

## Next Steps

1. ✅ Run migration
2. ✅ Test API endpoints
3. ⏳ Update mobile app to use new endpoints
4. ⏳ Remove hardcoded `vertical_form_mapping.dart`
5. ⏳ Create admin panel UI for form management
6. ⏳ Add form schema validation
7. ⏳ Implement JSON form renderer in mobile app

## Support

For issues:
1. Check backend logs: `tail -f backend.log`
2. Check database: `psql -c "SELECT * FROM app_forms"`
3. Test API with curl commands above
4. Review `UNIFIED_FORM_SYSTEM.md` for architecture details

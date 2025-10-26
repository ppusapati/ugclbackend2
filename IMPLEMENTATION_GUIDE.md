# Project Management System - Implementation Guide

## Quick Start Guide

This guide will help you integrate and deploy the project management system.

---

## Step 1: Database Setup

### Run Migration

```bash
# Navigate to backend directory
cd D:\Maheshwari\UGCL\backend\v1

# Run the migration
psql -U postgres -d ugcl -f migrations/010_create_project_management_tables.sql
```

**Verify Migration:**
```sql
-- Check tables were created
\dt *project*
\dt *task*
\dt *budget*
\dt *zone*
\dt *node*

-- Check PostGIS extension
SELECT PostGIS_version();
```

---

## Step 2: Install Go Dependencies

Add these dependencies to your project:

```bash
go get github.com/paulmach/orb
go get github.com/paulmach/orb/geojson
go get github.com/gin-gonic/gin
go get gorm.io/gorm
```

Update `go.mod`:
```go
require (
    github.com/paulmach/orb v0.11.0
    github.com/paulmach/orb/geojson v0.11.0
    // ... other dependencies
)
```

Run:
```bash
go mod tidy
go mod download
```

---

## Step 3: Update Main Application

### Update `main.go`

Add project routes to your main application:

```go
package main

import (
    "p9e.in/ugcl/routes"
    "p9e.in/ugcl/config"
    // ... other imports
)

func main() {
    // Initialize database
    config.InitDB()

    // Initialize Gin router
    router := gin.Default()

    // Register existing routes
    // ... your existing route registrations

    // Register project management routes
    api := router.Group("/api/v1")
    api.Use(middleware.JWTMiddleware) // Assuming you have JWT middleware

    routes.RegisterProjectRoutes(api)

    // Start server
    router.Run(":8080")
}
```

---

## Step 4: Update Config for Models

### Update `config/migrations.go`

Add project models to auto-migration:

```go
package config

import (
    "p9e.in/ugcl/models"
    // ... other imports
)

func RunMigrations() {
    DB.AutoMigrate(
        // ... existing models
        &models.Project{},
        &models.Zone{},
        &models.Node{},
        &models.Task{},
        &models.TaskAssignment{},
        &models.BudgetAllocation{},
        &models.TaskAuditLog{},
        &models.TaskComment{},
        &models.TaskAttachment{},
        &models.ProjectRole{},
        &models.UserProjectRole{},
    )
}
```

---

## Step 5: Seed Default Data

### Create Default Workflow

Run this API call to create the task approval workflow:

```bash
curl -X POST http://localhost:8080/api/v1/workflows/task-approval \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json"
```

**Or create programmatically:**

```go
// In config/site_seeding.go or similar
func SeedProjectData() {
    workflowHandler := handlers.NewProjectWorkflowHandler()
    // Call CreateTaskApprovalWorkflow
}
```

---

## Step 6: Update Permissions

### Add Project Permissions to Existing Roles

Update `config/permissions.go` or your permission seeding file:

```go
// Add project permissions to admin role
adminPermissions := []string{
    // ... existing permissions
    "project:create",
    "project:read",
    "project:update",
    "project:delete",
    "task:create",
    "task:read",
    "task:update",
    "task:delete",
    "task:assign",
    "task:approve",
    "task:submit",
    "task:execute",
    "task:verify",
    "task:comment",
    "budget:view",
    "budget:allocate",
    "budget:manage",
    "user:assign",
    "admin_all",
}
```

---

## Step 7: Test the API

### 1. Create a Project

```bash
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "PROJ-001",
    "name": "Test Pipeline Project",
    "description": "Test project for pipeline installation",
    "business_vertical_id": "YOUR_BUSINESS_VERTICAL_ID",
    "total_budget": 1000000,
    "currency": "INR"
  }'
```

### 2. Upload KMZ File

```bash
curl -X POST http://localhost:8080/api/v1/projects/PROJECT_ID/kmz \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "kmz_file=@/path/to/your/file.kmz"
```

### 3. List Projects

```bash
curl -X GET http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 4. Get Project Nodes

```bash
curl -X GET http://localhost:8080/api/v1/projects/PROJECT_ID/nodes \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5. Create a Task

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "TASK-001",
    "title": "Install Pipeline Segment 1",
    "description": "Install pipeline from node A to node B",
    "project_id": "PROJECT_ID",
    "start_node_id": "START_NODE_ID",
    "stop_node_id": "STOP_NODE_ID",
    "priority": "high",
    "allocated_budget": 50000
  }'
```

### 6. Assign Task to Users

```bash
curl -X POST http://localhost:8080/api/v1/tasks/TASK_ID/assign \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "assignments": [
      {
        "user_id": "USER_ID_1",
        "user_name": "John Doe",
        "user_type": "employee",
        "role": "worker",
        "can_edit": true
      },
      {
        "user_id": "USER_ID_2",
        "user_name": "Jane Smith",
        "user_type": "employee",
        "role": "supervisor",
        "can_approve": true
      }
    ]
  }'
```

---

## Step 8: Frontend Integration

### Install Frontend Dependencies

```bash
npm install leaflet
npm install chart.js
npm install axios
```

### Example: Display Map with Zones and Nodes

```javascript
import L from 'leaflet';

// Initialize map
const map = L.map('map').setView([lat, lon], 13);
L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png').addTo(map);

// Fetch project GeoJSON
fetch('/api/v1/projects/PROJECT_ID/geojson', {
  headers: { 'Authorization': `Bearer ${token}` }
})
.then(res => res.json())
.then(geojson => {
  L.geoJSON(geojson, {
    style: feature => {
      // Style zones
      if (feature.properties.type === 'zone') {
        return { color: 'blue', fillOpacity: 0.3 };
      }
    },
    pointToLayer: (feature, latlng) => {
      // Style nodes based on type
      const color = feature.properties.node_type === 'start' ? 'green' : 'red';
      return L.circleMarker(latlng, {
        radius: 8,
        fillColor: color,
        color: '#000',
        weight: 1,
        opacity: 1,
        fillOpacity: 0.8
      });
    },
    onEachFeature: (feature, layer) => {
      // Add popups
      if (feature.properties.name) {
        layer.bindPopup(feature.properties.name);
      }
    }
  }).addTo(map);
});
```

### Example: Display Task List

```javascript
// Fetch tasks
fetch('/api/v1/tasks?project_id=PROJECT_ID', {
  headers: { 'Authorization': `Bearer ${token}` }
})
.then(res => res.json())
.then(data => {
  data.tasks.forEach(task => {
    console.log(`Task: ${task.title}, Status: ${task.status}, Progress: ${task.progress}%`);
  });
});
```

### Example: Create Budget Chart

```javascript
import { Chart } from 'chart.js';

// Fetch budget summary
fetch('/api/v1/budget/projects/PROJECT_ID/summary', {
  headers: { 'Authorization': `Bearer ${token}` }
})
.then(res => res.json())
.then(data => {
  const ctx = document.getElementById('budgetChart');
  new Chart(ctx, {
    type: 'bar',
    data: {
      labels: data.category_breakdown.map(c => c.category),
      datasets: [{
        label: 'Planned Amount',
        data: data.category_breakdown.map(c => c.planned_amount),
        backgroundColor: 'rgba(54, 162, 235, 0.5)'
      }, {
        label: 'Actual Amount',
        data: data.category_breakdown.map(c => c.actual_amount),
        backgroundColor: 'rgba(255, 99, 132, 0.5)'
      }]
    }
  });
});
```

---

## Step 9: Configure File Upload

### Update File Handler

Ensure your file handler supports KMZ uploads:

```go
// In handlers/file_handler.go or similar
func UploadFile(c *gin.Context) {
    file, err := c.FormFile("kmz_file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
        return
    }

    // Save file
    uploadPath := fmt.Sprintf("./uploads/kmz/%s", file.Filename)
    if err := c.SaveUploadedFile(file, uploadPath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"path": uploadPath})
}
```

---

## Step 10: Monitoring & Logging

### Add Logging

```go
import "log"

// In handlers
log.Printf("✅ Created project: %s (ID: %s)", project.Name, project.ID)
log.Printf("⚠️  Task rejected: %s by %s", taskID, userName)
log.Printf("❌ Failed to create task: %v", err)
```

### Monitor Database

```sql
-- Check project statistics
SELECT
    COUNT(*) as total_projects,
    SUM(total_budget) as total_budget,
    SUM(spent_budget) as spent_budget
FROM projects
WHERE deleted_at IS NULL;

-- Check task distribution
SELECT
    status,
    COUNT(*) as count
FROM tasks
WHERE deleted_at IS NULL
GROUP BY status;

-- Check budget utilization
SELECT
    p.name,
    p.total_budget,
    p.spent_budget,
    ROUND((p.spent_budget / p.total_budget) * 100, 2) as utilization_percent
FROM projects p
WHERE deleted_at IS NULL
ORDER BY utilization_percent DESC;
```

---

## Common Issues & Solutions

### Issue 1: PostGIS Not Installed
**Error:** `type "geometry" does not exist`

**Solution:**
```sql
CREATE EXTENSION IF NOT EXISTS postgis;
```

### Issue 2: Import Errors
**Error:** `cannot find package "github.com/paulmach/orb"`

**Solution:**
```bash
go get github.com/paulmach/orb
go mod tidy
```

### Issue 3: Permission Denied
**Error:** `Permission denied: requires 'project:create'`

**Solution:**
- Verify user has required permission in their role
- Check JWT token contains correct permissions
- Update role permissions in database

### Issue 4: KMZ Parse Failure
**Error:** `Failed to parse KMZ: no KML file found`

**Solution:**
- Verify KMZ file is a valid ZIP archive
- Ensure it contains a .kml file
- Check file is not corrupted

---

## Production Checklist

- [ ] Database migration completed successfully
- [ ] PostGIS extension enabled
- [ ] All Go dependencies installed
- [ ] Default workflow created
- [ ] Project permissions added to roles
- [ ] API endpoints tested
- [ ] File upload directory created with proper permissions
- [ ] Frontend integrated and tested
- [ ] Logging configured
- [ ] Error handling tested
- [ ] Backup strategy in place
- [ ] Security review completed

---

## Next Steps

1. **Custom Workflows**: Create additional workflows for specific project types
2. **Reports**: Build custom reports for project progress and budget analysis
3. **Notifications**: Implement email/SMS notifications for task assignments and approvals
4. **Mobile App**: Develop mobile app for field workers
5. **Integration**: Integrate with existing ERP/accounting systems
6. **Analytics**: Add advanced analytics and dashboards

---

## Support

For technical support or questions:
- Check the main documentation: `PROJECT_MANAGEMENT_DOCUMENTATION.md`
- Review API examples in this guide
- Contact development team

**Version**: 1.0.0
**Last Updated**: 2025-10-25

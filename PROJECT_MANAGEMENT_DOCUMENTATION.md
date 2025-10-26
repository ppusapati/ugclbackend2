# Project Management System - Complete Documentation

## Table of Contents
1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Database Schema](#database-schema)
4. [KMZ File Handling](#kmz-file-handling)
5. [API Endpoints](#api-endpoints)
6. [Workflow System](#workflow-system)
7. [Role-Based Access Control](#role-based-access-control)
8. [Budget Management](#budget-management)
9. [Frontend Integration](#frontend-integration)
10. [Deployment Guide](#deployment-guide)

---

## Overview

The Project Management System is a comprehensive solution for managing infrastructure projects with geographic data (KMZ files), task allocation, budget tracking, and workflow approvals.

### Key Features
- **KMZ File Upload & Processing**: Upload KMZ files containing zones, nodes, and geographic data
- **Project Management**: Create and manage projects with timelines and budgets
- **Task Allocation**: Assign tasks between start and stop nodes to multiple users
- **Multi-User Support**: Assign workers, supervisors, and approvers to tasks
- **Workflow Integration**: Built-in approval workflows for task management
- **Budget Tracking**: Track planned vs actual costs at project and task levels
- **Role-Based Permissions**: Fine-grained project-specific permissions
- **Audit Logging**: Complete audit trail for all task changes
- **PostGIS Integration**: Geographic data storage and querying

---

## Architecture

### Technology Stack
- **Backend**: Go (Golang) with Gin framework
- **Database**: PostgreSQL with PostGIS extension
- **File Processing**: KMZ/KML parsing with GeoJSON conversion
- **Authentication**: JWT-based authentication
- **Authorization**: Permission-based access control

### System Components

```
┌─────────────────────────────────────────────────────────┐
│                    Frontend Application                  │
└────────────────────┬────────────────────────────────────┘
                     │
                     │ HTTP/REST API
                     │
┌────────────────────▼────────────────────────────────────┐
│                   API Gateway & Routes                   │
│  ┌──────────┬──────────┬──────────┬───────────────────┐ │
│  │ Projects │  Tasks   │  Budget  │  Roles & Workflow │ │
│  └──────────┴──────────┴──────────┴───────────────────┘ │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│                    Business Logic Layer                  │
│  ┌──────────────────────────────────────────────────┐   │
│  │  Handlers                                        │   │
│  │  • ProjectHandler                                │   │
│  │  • TaskHandler                                   │   │
│  │  • BudgetHandler                                 │   │
│  │  • ProjectRoleHandler                            │   │
│  │  • ProjectWorkflowHandler                        │   │
│  │  • KMZParser                                     │   │
│  └──────────────────────────────────────────────────┘   │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│                   Data Access Layer                      │
│  ┌──────────────────────────────────────────────────┐   │
│  │  Models (GORM)                                   │   │
│  │  • Project, Zone, Node                           │   │
│  │  • Task, TaskAssignment                          │   │
│  │  • BudgetAllocation                              │   │
│  │  • ProjectRole, UserProjectRole                  │   │
│  │  • TaskAuditLog, TaskComment                     │   │
│  └──────────────────────────────────────────────────┘   │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│             PostgreSQL with PostGIS                      │
│  • Relational data storage                               │
│  • Geospatial data (zones, nodes)                       │
│  • JSONB for flexible metadata                          │
└──────────────────────────────────────────────────────────┘
```

---

## Database Schema

### Core Tables

#### 1. Projects
Stores project information and uploaded KMZ data.

```sql
CREATE TABLE projects (
    id UUID PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    business_vertical_id UUID NOT NULL,

    -- KMZ file info
    kmz_file_name VARCHAR(255),
    kmz_file_path VARCHAR(500),
    kmz_uploaded_at TIMESTAMP,
    geojson_data JSONB DEFAULT '{}',

    -- Timeline
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    actual_start_date TIMESTAMP,
    actual_end_date TIMESTAMP,

    -- Budget
    total_budget DECIMAL(15,2) DEFAULT 0,
    allocated_budget DECIMAL(15,2) DEFAULT 0,
    spent_budget DECIMAL(15,2) DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'INR',

    -- Status
    status VARCHAR(50) DEFAULT 'draft',
    progress DECIMAL(5,2) DEFAULT 0,

    -- Workflow
    workflow_id UUID,

    -- Audit
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);
```

#### 2. Zones
Geographic zones extracted from KMZ files.

```sql
CREATE TABLE zones (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id),

    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    description TEXT,
    label VARCHAR(255),

    -- PostGIS geometry
    geometry GEOMETRY(Geometry, 4326),
    centroid GEOMETRY(Point, 4326),
    area DECIMAL(15,2),

    -- GeoJSON representation
    geojson JSONB DEFAULT '{}',
    properties JSONB DEFAULT '{}',

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);
```

#### 3. Nodes
Points/nodes within zones (start, stop, waypoint).

```sql
CREATE TABLE nodes (
    id UUID PRIMARY KEY,
    zone_id UUID NOT NULL REFERENCES zones(id),
    project_id UUID NOT NULL REFERENCES projects(id),

    name VARCHAR(255) NOT NULL,
    code VARCHAR(50),
    description TEXT,
    label VARCHAR(255),
    node_type VARCHAR(50) NOT NULL, -- start, stop, waypoint

    -- PostGIS location
    location GEOMETRY(Point, 4326) NOT NULL,
    latitude DECIMAL(10,8),
    longitude DECIMAL(11,8),
    elevation DECIMAL(10,2),

    -- GeoJSON representation
    geojson JSONB DEFAULT '{}',
    properties JSONB DEFAULT '{}',

    status VARCHAR(50) DEFAULT 'available', -- available, allocated, in-progress, completed

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);
```

#### 4. Tasks
Work tasks allocated between start and stop nodes.

```sql
CREATE TABLE tasks (
    id UUID PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,

    -- Project context
    project_id UUID NOT NULL REFERENCES projects(id),
    zone_id UUID REFERENCES zones(id),

    -- Node references
    start_node_id UUID NOT NULL REFERENCES nodes(id),
    stop_node_id UUID NOT NULL REFERENCES nodes(id),

    -- Timeline
    planned_start_date TIMESTAMP,
    planned_end_date TIMESTAMP,
    actual_start_date TIMESTAMP,
    actual_end_date TIMESTAMP,

    -- Budget
    allocated_budget DECIMAL(15,2) DEFAULT 0,
    labor_cost DECIMAL(15,2) DEFAULT 0,
    material_cost DECIMAL(15,2) DEFAULT 0,
    equipment_cost DECIMAL(15,2) DEFAULT 0,
    other_cost DECIMAL(15,2) DEFAULT 0,
    total_cost DECIMAL(15,2) DEFAULT 0,

    -- Status
    status VARCHAR(50) DEFAULT 'pending',
    progress DECIMAL(5,2) DEFAULT 0,
    priority VARCHAR(20) DEFAULT 'medium',

    -- Workflow
    workflow_id UUID,
    current_state VARCHAR(50),
    form_submission_id UUID,

    metadata JSONB DEFAULT '{}',

    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);
```

#### 5. Task Assignments
User assignments to tasks with roles.

```sql
CREATE TABLE task_assignments (
    id UUID PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id),

    -- User info
    user_id VARCHAR(255) NOT NULL,
    user_name VARCHAR(255),
    user_type VARCHAR(50) NOT NULL, -- employee, contractor, supervisor
    role VARCHAR(50) NOT NULL, -- worker, supervisor, manager, approver

    -- Assignment details
    assigned_by VARCHAR(255) NOT NULL,
    assigned_at TIMESTAMP NOT NULL,
    start_date TIMESTAMP,
    end_date TIMESTAMP,

    -- Status
    status VARCHAR(50) DEFAULT 'active',
    is_active BOOLEAN DEFAULT true,

    -- Permissions
    can_edit BOOLEAN DEFAULT false,
    can_approve BOOLEAN DEFAULT false,

    notes TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);
```

#### 6. Budget Allocations
Budget allocations at project and task levels.

```sql
CREATE TABLE budget_allocations (
    id UUID PRIMARY KEY,

    -- Reference (project OR task)
    project_id UUID REFERENCES projects(id),
    task_id UUID REFERENCES tasks(id),

    -- Budget details
    category VARCHAR(50) NOT NULL, -- labor, material, equipment, overhead, contingency
    description TEXT,
    planned_amount DECIMAL(15,2) NOT NULL,
    actual_amount DECIMAL(15,2) DEFAULT 0,
    currency VARCHAR(10) DEFAULT 'INR',

    -- Timeline
    allocation_date TIMESTAMP NOT NULL,
    start_date TIMESTAMP,
    end_date TIMESTAMP,

    -- Status
    status VARCHAR(50) DEFAULT 'allocated',

    -- Approval
    approved_by VARCHAR(255),
    approved_at TIMESTAMP,

    notes TEXT,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);
```

---

## KMZ File Handling

### KMZ File Structure
KMZ files are ZIP archives containing KML (Keyhole Markup Language) files with geographic data.

### Parsing Process

1. **Upload**: User uploads KMZ file for a project
2. **Extraction**: System extracts KML from KMZ archive
3. **Parsing**: KML is parsed to extract:
   - Placemarks (points, lines, polygons)
   - Extended data and properties
   - Coordinate information
4. **Categorization**: Data is categorized into:
   - **Zones**: Polygons representing geographic areas
   - **Nodes**: Points representing locations (start/stop/waypoint)
   - **Labels**: Lines and other geometric features
5. **Storage**: Data is stored in:
   - PostGIS geometry columns for spatial queries
   - GeoJSON format for API responses
   - JSONB for flexible metadata

### KMZ Parser Usage

```go
// Initialize parser
parser := handlers.NewKMZParser()

// Parse KMZ file
parsedData, err := parser.ParseKMZToStructuredData(kmzData)

// Access parsed data
fmt.Printf("Total features: %d\n", parsedData.TotalFeatures)
fmt.Printf("Zones: %d\n", len(parsedData.Zones))
fmt.Printf("Nodes: %d\n", len(parsedData.Nodes))

// Store in database
for _, zoneData := range parsedData.Zones {
    // Create zone record
}

for _, nodeData := range parsedData.Nodes {
    // Create node record
}
```

### Supported Data Types

| KML Type | Converted To | Stored As |
|----------|-------------|-----------|
| Point | Node (start/stop/waypoint) | POINT geometry + GeoJSON |
| LineString | Label | LINESTRING geometry + GeoJSON |
| Polygon | Zone | POLYGON geometry + GeoJSON |
| MultiGeometry | Multiple features | Respective geometries + GeoJSON |

---

## API Endpoints

### Projects

| Method | Endpoint | Permission | Description |
|--------|----------|-----------|-------------|
| POST | `/api/v1/projects` | `project:create` | Create new project |
| GET | `/api/v1/projects` | `project:read` | List all projects |
| GET | `/api/v1/projects/:id` | `project:read` | Get project details |
| PUT | `/api/v1/projects/:id` | `project:update` | Update project |
| DELETE | `/api/v1/projects/:id` | `project:delete` | Delete project |
| POST | `/api/v1/projects/:id/kmz` | `project:update` | Upload KMZ file |
| GET | `/api/v1/projects/:id/geojson` | `project:read` | Get GeoJSON data |
| GET | `/api/v1/projects/:id/zones` | `project:read` | Get project zones |
| GET | `/api/v1/projects/:id/nodes` | `project:read` | Get project nodes |
| GET | `/api/v1/projects/:id/stats` | `project:read` | Get project statistics |

### Tasks

| Method | Endpoint | Permission | Description |
|--------|----------|-----------|-------------|
| POST | `/api/v1/tasks` | `task:create` | Create new task |
| GET | `/api/v1/tasks` | `task:read` | List all tasks |
| GET | `/api/v1/tasks/:id` | `task:read` | Get task details |
| PUT | `/api/v1/tasks/:id` | `task:update` | Update task |
| POST | `/api/v1/tasks/:id/assign` | `task:assign` | Assign users to task |
| PUT | `/api/v1/tasks/:id/status` | `task:update` | Update task status |
| POST | `/api/v1/tasks/:id/comments` | `task:comment` | Add comment |
| GET | `/api/v1/tasks/:id/comments` | `task:read` | Get task comments |
| GET | `/api/v1/tasks/:id/audit` | `task:read` | Get audit log |

### Task Workflow

| Method | Endpoint | Permission | Description |
|--------|----------|-----------|-------------|
| POST | `/api/v1/tasks/:id/submit` | `task:submit` | Submit for approval |
| POST | `/api/v1/tasks/:id/approve` | `task:approve` | Approve task |
| POST | `/api/v1/tasks/:id/reject` | `task:approve` | Reject task |
| POST | `/api/v1/tasks/:id/complete` | `task:execute` | Mark as completed |
| GET | `/api/v1/tasks/:id/workflow/history` | `task:read` | Get workflow history |
| GET | `/api/v1/tasks/:id/workflow/actions` | `task:read` | Get available actions |

### Budget Management

| Method | Endpoint | Permission | Description |
|--------|----------|-----------|-------------|
| POST | `/api/v1/budget/allocations` | `budget:allocate` | Create allocation |
| GET | `/api/v1/budget/allocations` | `budget:view` | List allocations |
| GET | `/api/v1/budget/allocations/:id` | `budget:view` | Get allocation |
| PUT | `/api/v1/budget/allocations/:id` | `budget:manage` | Update allocation |
| DELETE | `/api/v1/budget/allocations/:id` | `budget:manage` | Delete allocation |
| POST | `/api/v1/budget/allocations/:id/approve` | `budget:manage` | Approve allocation |
| GET | `/api/v1/budget/projects/:id/summary` | `budget:view` | Project budget summary |
| GET | `/api/v1/budget/tasks/:id/summary` | `budget:view` | Task budget summary |

### Project Roles

| Method | Endpoint | Permission | Description |
|--------|----------|-----------|-------------|
| POST | `/api/v1/project-roles` | `admin_all` | Create role |
| GET | `/api/v1/project-roles` | `project:read` | List roles |
| GET | `/api/v1/project-roles/:id` | `project:read` | Get role |
| PUT | `/api/v1/project-roles/:id` | `admin_all` | Update role |
| DELETE | `/api/v1/project-roles/:id` | `admin_all` | Delete role |
| POST | `/api/v1/project-roles/assign` | `user:assign` | Assign role to user |
| DELETE | `/api/v1/project-roles/assignments/:id` | `user:assign` | Revoke role |
| GET | `/api/v1/project-roles/permissions` | `project:read` | List permissions |

---

## Workflow System

### Task Approval Workflow

The system uses a state-machine-based workflow for task approvals:

```
┌──────┐  submit   ┌───────────┐  approve  ┌──────────┐
│ Draft│─────────▶│ Submitted │─────────▶│ Approved │
└──────┘           └───────────┘           └──────────┘
                        │                        │
                        │ reject                 │ start
                        ▼                        ▼
                   ┌──────────┐           ┌────────────┐
                   │ Rejected │           │ In Progress│
                   └──────────┘           └────────────┘
                        │                        │
                        │ revise                 │ complete
                        ▼                        ▼
                   ┌──────┐               ┌───────────┐
                   │ Draft│               │ Completed │
                   └──────┘               └───────────┘
                                                 │
                                                 │ verify
                                                 ▼
                                            ┌──────────┐
                                            │ Verified │
                                            └──────────┘
```

### Workflow States

| State | Description | Available Actions |
|-------|-------------|-------------------|
| draft | Initial state | submit |
| submitted | Pending approval | approve, reject |
| approved | Approved, ready to start | start |
| rejected | Rejected, needs revision | revise |
| in_progress | Work in progress | complete |
| completed | Work completed | verify, return |
| verified | Verified and finalized | None (final state) |

### Workflow Transitions

```json
{
  "transitions": [
    {"from": "draft", "to": "submitted", "action": "submit", "permission": "task:submit"},
    {"from": "submitted", "to": "approved", "action": "approve", "permission": "task:approve"},
    {"from": "submitted", "to": "rejected", "action": "reject", "permission": "task:approve"},
    {"from": "rejected", "to": "draft", "action": "revise", "permission": "task:update"},
    {"from": "approved", "to": "in_progress", "action": "start", "permission": "task:execute"},
    {"from": "in_progress", "to": "completed", "action": "complete", "permission": "task:execute"},
    {"from": "completed", "to": "verified", "action": "verify", "permission": "task:verify"},
    {"from": "completed", "to": "in_progress", "action": "return", "permission": "task:verify"}
  ]
}
```

---

## Role-Based Access Control

### Default Project Roles

| Role | Level | Permissions |
|------|-------|-------------|
| Project Administrator | 100 | All permissions (`project:*`, `task:*`, `budget:*`, `user:*`) |
| Project Manager | 80 | Create/update projects, manage tasks, allocate budget |
| Supervisor | 60 | View projects, update tasks, assign tasks (limited) |
| Worker | 40 | View tasks, update own tasks, add comments |
| Viewer | 20 | View-only access to projects and tasks |

### Permission Structure

Permissions follow the format: `resource:action`

**Resources**: project, task, budget, user, report

**Actions**: create, read, update, delete, assign, approve, verify, execute, manage

**Examples**:
- `project:create` - Create new projects
- `task:update` - Update task information
- `budget:view` - View budget information
- `task:approve` - Approve tasks
- `admin_all` - All permissions

### Role Assignment

Roles are assigned per project, allowing the same user to have different roles in different projects:

```json
{
  "user_id": "user123",
  "project_id": "proj-001",
  "role_id": "supervisor-role",
  "assigned_by": "admin",
  "assigned_at": "2025-10-25T10:00:00Z",
  "is_active": true
}
```

---

## Budget Management

### Budget Hierarchy

```
Project Budget (Total)
    ├── Project-level Allocations
    │   ├── Labor
    │   ├── Material
    │   ├── Equipment
    │   ├── Overhead
    │   └── Contingency
    │
    └── Task Budgets (Allocated)
        └── Task-level Allocations
            ├── Labor Cost
            ├── Material Cost
            ├── Equipment Cost
            └── Other Cost
```

### Budget Categories

1. **Labor**: Human resource costs
2. **Material**: Material and supplies
3. **Equipment**: Equipment rental/purchase
4. **Overhead**: Administrative and indirect costs
5. **Contingency**: Reserve for unforeseen expenses

### Budget Tracking

**At Project Level:**
- `total_budget`: Overall project budget
- `allocated_budget`: Sum of all allocations
- `spent_budget`: Sum of actual expenditures

**At Task Level:**
- `allocated_budget`: Budget allocated to task
- `labor_cost`: Actual labor costs
- `material_cost`: Actual material costs
- `equipment_cost`: Actual equipment costs
- `other_cost`: Other actual costs
- `total_cost`: Sum of all actual costs

### Budget Allocation Workflow

1. Create budget allocation (project or task level)
2. Submit for approval (optional)
3. Approve allocation
4. Track actual expenditures
5. Update actual amounts
6. Monitor variance (planned vs actual)

---

## Frontend Integration

### Map Visualization

The frontend should display:
- **Zones**: As polygons on the map
- **Nodes**: As markers (different colors for start/stop/waypoint)
- **Tasks**: Lines connecting start and stop nodes
- **Task Status**: Color-coded based on status

### Recommended Libraries

- **Leaflet.js** or **Mapbox GL JS**: For map rendering
- **GeoJSON**: For geographic data display
- **Chart.js**: For budget charts and Gantt charts
- **FullCalendar**: For timeline view

### API Response Example

**Project with GeoJSON:**
```json
{
  "id": "uuid",
  "name": "Pipeline Project 2025",
  "geojson": {
    "type": "FeatureCollection",
    "features": [
      {
        "type": "Feature",
        "geometry": {
          "type": "Polygon",
          "coordinates": [[[lon1, lat1], [lon2, lat2], ...]]
        },
        "properties": {
          "name": "Zone A",
          "type": "zone"
        }
      },
      {
        "type": "Feature",
        "geometry": {
          "type": "Point",
          "coordinates": [lon, lat]
        },
        "properties": {
          "name": "Node Start 1",
          "node_type": "start",
          "status": "allocated"
        }
      }
    ]
  }
}
```

### Gantt Chart Data

```json
{
  "tasks": [
    {
      "id": "task-001",
      "title": "Install Pipeline Segment 1",
      "start": "2025-11-01",
      "end": "2025-11-15",
      "progress": 45,
      "status": "in-progress",
      "assignees": ["John Doe", "Jane Smith"],
      "dependencies": []
    }
  ]
}
```

---

## Deployment Guide

### Prerequisites

1. **PostgreSQL 13+** with PostGIS extension
2. **Go 1.21+**
3. **Git**

### Database Setup

```bash
# Install PostGIS
sudo apt-get install postgresql-13-postgis-3

# Create database
createdb ugcl

# Connect and enable PostGIS
psql ugcl
CREATE EXTENSION IF NOT EXISTS postgis;
```

### Run Migrations

```bash
# Navigate to backend directory
cd backend/v1

# Run migration
psql -U postgres -d ugcl -f migrations/010_create_project_management_tables.sql
```

### Configuration

Update `config/config.go` with database credentials:

```go
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=ugcl
```

### Install Dependencies

```bash
# Install required Go packages
go get github.com/paulmach/orb
go get github.com/paulmach/orb/geojson
```

### Build and Run

```bash
# Build
go build -o ugcl_backend .

# Run
./ugcl_backend
```

### API Testing

```bash
# Create a project
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "code": "PROJ-001",
    "name": "Test Project",
    "business_vertical_id": "vertical-uuid"
  }'

# Upload KMZ
curl -X POST http://localhost:8080/api/v1/projects/:id/kmz \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "kmz_file=@path/to/file.kmz"
```

### Seed Default Data

```bash
# Create default project roles and workflow
curl -X POST http://localhost:8080/api/v1/workflows/task-approval \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

---

## Best Practices

### 1. Task Management
- Always assign at least one supervisor to tasks
- Set realistic planned dates
- Update progress regularly
- Add comments for important milestones

### 2. Budget Tracking
- Allocate budgets before work starts
- Update actual costs weekly
- Review variance reports monthly
- Approve all allocations before use

### 3. Workflow Usage
- Submit tasks for approval before starting work
- Provide detailed comments during approvals/rejections
- Verify task completion before final closure

### 4. Role Assignment
- Assign roles based on least privilege principle
- Review and revoke inactive role assignments
- Document role assignment reasons in notes

### 5. Data Management
- Upload high-quality KMZ files
- Validate node connections before task creation
- Regularly backup project data
- Archive completed projects

---

## Troubleshooting

### Common Issues

**Issue: KMZ upload fails**
- Solution: Ensure KMZ file contains valid KML
- Check file size limits
- Verify PostGIS is installed

**Issue: Tasks cannot be assigned**
- Solution: Verify nodes are in "available" status
- Check user has `task:assign` permission
- Ensure project exists and is active

**Issue: Budget allocation fails**
- Solution: Verify project/task exists
- Check allocated amount doesn't exceed project budget
- Ensure user has `budget:allocate` permission

**Issue: Workflow transition fails**
- Solution: Check current state and available actions
- Verify user has required permission for action
- Ensure workflow is properly configured

---

## API Authentication

All API endpoints (except login/register) require JWT authentication.

**Header:**
```
Authorization: Bearer <JWT_TOKEN>
```

**Token Structure:**
```json
{
  "user_id": "user123",
  "name": "John Doe",
  "role": "project_manager",
  "permissions": ["project:read", "task:create", "task:assign"],
  "exp": 1698765432
}
```

---

## Support & Maintenance

For issues, questions, or feature requests, please contact the development team or create an issue in the project repository.

**Version**: 1.0.0
**Last Updated**: 2025-10-25
**Author**: Claude Code (Anthropic)

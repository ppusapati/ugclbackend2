# Gorilla Mux Conversion Guide

## Issue
The handlers were initially created with `gin.Context` but your project uses **Gorilla Mux** with standard `http.ResponseWriter` and `*http.Request`.

## What Needs to Change

### ❌ Wrong (Gin):
```go
func (h *Handler) SomeMethod(c *gin.Context) {
    var req SomeRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID, _ := c.Get("user_id")

    c.JSON(http.StatusOK, gin.H{"data": result})
}
```

### ✅ Correct (Gorilla Mux):
```go
func (h *Handler) SomeMethod(w http.ResponseWriter, r *http.Request) {
    var req SomeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    claims := middleware.GetClaims(r)
    userID := claims.UserID

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "data": result,
    })
}
```

## Files That Need Updating

1. ✅ `handlers/project_management.go` - **DONE**
2. ❌ `handlers/task_management.go` - **NEEDS UPDATE**
3. ❌ `handlers/budget_management.go` - **NEEDS UPDATE**
4. ❌ `handlers/project_workflow.go` - **NEEDS UPDATE**
5. ❌ `handlers/project_roles.go` - **NEEDS UPDATE**

## Quick Fix Pattern

For each handler method, apply these changes:

### 1. Function Signature
```go
// FROM:
func (h *TaskHandler) CreateTask(c *gin.Context)

// TO:
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request)
```

### 2. JSON Decoding
```go
// FROM:
var req CreateTaskRequest
if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}

// TO:
var req CreateTaskRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, "Invalid JSON", http.StatusBadRequest)
    return
}
```

### 3. Get User from Context
```go
// FROM:
userID, _ := c.Get("user_id")
userName, _ := c.Get("user_name")

// TO:
claims := middleware.GetClaims(r)
userID := claims.UserID

user := middleware.GetUser(r)
userName := user.Name
```

### 4. Get URL Parameters
```go
// FROM:
taskID := c.Param("id")

// TO:
vars := mux.Vars(r)
taskID := vars["id"]
```

### 5. Get Query Parameters
```go
// FROM:
status := c.Query("status")

// TO:
status := r.URL.Query().Get("status")
```

### 6. JSON Response
```go
// FROM:
c.JSON(http.StatusOK, gin.H{
    "message": "Success",
    "data": result,
})

// TO:
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{
    "message": "Success",
    "data": result,
})
```

### 7. Error Response
```go
// FROM:
c.JSON(http.StatusBadRequest, gin.H{"error": "Error message"})

// TO:
http.Error(w, "Error message", http.StatusBadRequest)
```

## Complete Example Conversion

### Before (Gin):
```go
package handlers

import (
    "github.com/gin-gonic/gin"
)

type TaskHandler struct {
    db *gorm.DB
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
    var req CreateTaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID, _ := c.Get("user_id")

    task := models.Task{
        Title: req.Title,
        CreatedBy: userID.(string),
    }

    if err := h.db.Create(&task).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "message": "Task created",
        "task": task,
    })
}
```

### After (Gorilla Mux):
```go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/gorilla/mux"
    "p9e.in/ugcl/middleware"
)

type TaskHandler struct {
    db *gorm.DB
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
    var req CreateTaskRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    claims := middleware.GetClaims(r)
    userID := claims.UserID

    task := models.Task{
        Title: req.Title,
        CreatedBy: userID,
    }

    if err := h.db.Create(&task).Error; err != nil {
        http.Error(w, "Failed to create task", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Task created",
        "task": task,
    })
}
```

## Required Imports

Make sure each handler file has these imports:

```go
import (
    "encoding/json"
    "net/http"
    "time"
    "log"
    "fmt"

    "github.com/gorilla/mux"
    "github.com/google/uuid"
    "gorm.io/gorm"

    "p9e.in/ugcl/config"
    "p9e.in/ugcl/middleware"
    "p9e.in/ugcl/models"
)
```

## Automated Conversion Script

You can use this sed/find-replace to help with bulk conversion:

```bash
# Change function signatures
find . -name "*.go" -exec sed -i 's/func (h \*\(.*\)) \(.*\)(c \*gin.Context)/func (h *\1) \2(w http.ResponseWriter, r *http.Request)/g' {} \;

# But manual review is recommended for accuracy
```

## Testing After Conversion

1. Build the project:
   ```bash
   go build
   ```

2. Fix any compilation errors

3. Test each endpoint:
   ```bash
   curl -X POST http://localhost:8080/api/v1/projects \
     -H "Authorization: Bearer TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"code":"TEST","name":"Test Project"}'
   ```

## Status

- ✅ project_management.go - Converted
- ❌ task_management.go - Needs conversion
- ❌ budget_management.go - Needs conversion
- ❌ project_workflow.go - Needs conversion
- ❌ project_roles.go - Needs conversion

Would you like me to convert all the remaining files now?

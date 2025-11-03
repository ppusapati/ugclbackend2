# Report Builder Installation & Setup Guide

## Prerequisites

- Go 1.19+
- PostgreSQL 13+
- Node.js 16+ (for frontend)

## Backend Setup

### 1. Install Go Dependencies

```bash
cd backend/v1
go get github.com/xuri/excelize/v2
go mod tidy
```

### 2. Register Routes

Update [routes/routes_v2.go](../routes/routes_v2.go):

```go
package routes

import (
    "github.com/gin-gonic/gin"
)

func SetupRoutesV2(router *gin.Engine) {
    // ... existing routes ...

    // Report Builder Routes
    RegisterReportRoutes(router)
}
```

### 3. Run Database Migrations

The report builder tables will be created automatically when you run migrations:

```bash
go run main.go
```

The following tables will be created:
- `report_definitions`
- `report_executions`
- `report_widgets`
- `dashboards`
- `report_templates`
- `report_shares`

### 4. Start Report Scheduler (Optional)

To enable scheduled reports, start the scheduler in your `main.go`:

```go
package main

import (
    "p9e.in/ugcl/handlers"
    // ... other imports
)

func main() {
    // ... existing setup ...

    // Start report scheduler in background
    scheduler := handlers.NewReportScheduler()
    go scheduler.StartScheduler()

    // Start server
    router.Run(":8080")
}
```

### 5. Verify Installation

Test the API endpoints:

```bash
# Get available form tables
curl http://localhost:8080/api/v1/reports/forms/tables \
  -H "Authorization: Bearer YOUR_TOKEN"

# Create a test report
curl -X POST http://localhost:8080/api/v1/reports/definitions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "code": "test_report",
    "name": "Test Report",
    "report_type": "table",
    "business_vertical_id": "YOUR_VERTICAL_ID",
    "data_sources": [...],
    "fields": [...]
  }'
```

## Frontend Setup

### 1. Install NPM Packages

```bash
cd frontend
npm install react-dnd react-dnd-html5-backend
npm install react-grid-layout
npm install chart.js react-chartjs-2
npm install @tanstack/react-query  # For data fetching
npm install axios
```

### 2. Create API Service

Create `src/services/reportService.js`:

```javascript
import axios from 'axios';

const API_BASE = '/api/v1';

export const reportService = {
  // Get available tables
  getAvailableTables: () =>
    axios.get(`${API_BASE}/reports/forms/tables`),

  // Get table schema
  getTableFields: (tableName) =>
    axios.get(`${API_BASE}/reports/forms/tables/${tableName}/fields`),

  // Create report
  createReport: (config) =>
    axios.post(`${API_BASE}/reports/definitions`, config),

  // Execute report
  executeReport: (reportId, filters = []) =>
    axios.post(`${API_BASE}/reports/definitions/${reportId}/execute`, { filters }),

  // Get report list
  getReports: (params) =>
    axios.get(`${API_BASE}/reports/definitions`, { params }),

  // Export report
  exportToExcel: (reportId) =>
    `${API_BASE}/reports/definitions/${reportId}/export/excel`,

  exportToCSV: (reportId) =>
    `${API_BASE}/reports/definitions/${reportId}/export/csv`,

  exportToPDF: (reportId) =>
    `${API_BASE}/reports/definitions/${reportId}/export/pdf`,
};
```

### 3. Add Routes

Update your React Router configuration:

```jsx
import ReportBuilder from './components/ReportBuilder';
import ReportViewer from './components/ReportViewer';
import ReportList from './components/ReportList';
import DashboardBuilder from './components/Dashboard/DashboardBuilder';
import DashboardViewer from './components/Dashboard/DashboardViewer';

// In your router configuration
<Routes>
  <Route path="/reports" element={<ReportList />} />
  <Route path="/reports/builder" element={<ReportBuilder />} />
  <Route path="/reports/:id" element={<ReportViewer />} />
  <Route path="/dashboards" element={<DashboardList />} />
  <Route path="/dashboards/builder" element={<DashboardBuilder />} />
  <Route path="/dashboards/:id" element={<DashboardViewer />} />
</Routes>
```

## Configuration

### Environment Variables

Add to your `.env` file:

```env
# Report Builder Settings
REPORT_MAX_ROWS=10000
REPORT_CACHE_TTL=300
REPORT_EXPORT_PATH=/tmp/reports
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@example.com
SMTP_PASSWORD=your-password
```

### Database Indexes

For better performance, ensure these indexes exist:

```sql
-- Indexes on form submission tables
CREATE INDEX idx_form_submissions_business_vertical ON form_submissions(business_vertical_id);
CREATE INDEX idx_form_submissions_created_at ON form_submissions(created_at);
CREATE INDEX idx_form_submissions_current_state ON form_submissions(current_state);

-- Indexes on report tables
CREATE INDEX idx_report_definitions_business_vertical ON report_definitions(business_vertical_id);
CREATE INDEX idx_report_executions_report_id ON report_executions(report_id);
```

## Testing

### Backend Unit Tests

Create `handlers/report_engine_test.go`:

```go
package handlers

import (
    "testing"
    "p9e.in/ugcl/models"
)

func TestBuildQuery(t *testing.T) {
    engine := NewReportEngine()

    dataSources := []models.DataSource{
        {
            Alias: "inspections",
            TableName: "form_site_inspection",
        },
    }

    fields := []models.ReportField{
        {
            FieldName: "site_name",
            DataSource: "inspections",
            IsVisible: true,
        },
    }

    filters := []models.ReportFilter{
        {
            FieldName: "created_at",
            DataSource: "inspections",
            Operator: "this_month",
        },
    }

    query, args, err := engine.buildQuery(dataSources, fields, filters, nil, nil, nil)

    if err != nil {
        t.Errorf("buildQuery failed: %v", err)
    }

    if query == "" {
        t.Error("Query should not be empty")
    }

    t.Logf("Generated Query: %s", query)
    t.Logf("Args: %v", args)
}
```

Run tests:
```bash
go test ./handlers -v
```

### Frontend Component Tests

Create `src/components/ReportBuilder/__tests__/ReportBuilder.test.jsx`:

```jsx
import { render, screen } from '@testing-library/react';
import ReportBuilder from '../index';

test('renders report builder', () => {
  render(<ReportBuilder />);
  const headerElement = screen.getByText(/Report Name/i);
  expect(headerElement).toBeInTheDocument();
});
```

## Performance Optimization

### 1. Enable Query Caching

```go
// In report_engine.go
import "github.com/patrickmn/go-cache"

var queryCache = cache.New(5*time.Minute, 10*time.Minute)

func (re *ReportEngine) ExecuteReport(...) (*ReportResult, error) {
    // Generate cache key
    cacheKey := fmt.Sprintf("report:%s:%s", reportDef.ID, hashFilters(runtimeFilters))

    // Check cache
    if cached, found := queryCache.Get(cacheKey); found {
        return cached.(*ReportResult), nil
    }

    // Execute query...
    result, err := re.executeQuery(...)

    // Store in cache
    if err == nil {
        queryCache.Set(cacheKey, result, cache.DefaultExpiration)
    }

    return result, err
}
```

### 2. Add Pagination

```go
func (re *ReportEngine) ExecuteReport(..., page, limit int) (*ReportResult, error) {
    // Add LIMIT and OFFSET to query
    offset := (page - 1) * limit
    query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

    // ... execute query
}
```

### 3. Frontend Virtualization

For large datasets, use react-window:

```bash
npm install react-window
```

```jsx
import { FixedSizeList } from 'react-window';

const Row = ({ index, style, data }) => (
  <div style={style}>
    {data[index].map(cell => <span>{cell}</span>)}
  </div>
);

function VirtualizedTable({ data }) {
  return (
    <FixedSizeList
      height={600}
      itemCount={data.length}
      itemSize={35}
      itemData={data}
    >
      {Row}
    </FixedSizeList>
  );
}
```

## Troubleshooting

### Common Issues

1. **"Table not found" error**
   - Ensure form has `db_table_name` set
   - Verify table was created via FormTableManager
   - Check table exists: `\dt form_*` in psql

2. **Export fails**
   - Install excelize: `go get github.com/xuri/excelize/v2`
   - Check file permissions on export directory
   - Verify sufficient disk space

3. **Scheduler not running**
   - Check scheduler is started in main.go
   - Verify cron expressions are valid
   - Check logs for errors

4. **Slow query execution**
   - Add database indexes
   - Reduce date range
   - Limit number of joined tables
   - Enable query caching

### Debug Mode

Enable debug logging:

```go
// In report_engine.go
func (re *ReportEngine) buildQuery(...) {
    log.Printf("🔍 Building Query...")
    log.Printf("Data Sources: %+v", dataSources)
    log.Printf("Fields: %+v", fields)
    log.Printf("Filters: %+v", filters)

    // ... query building logic

    log.Printf("📝 Generated SQL:\n%s", query)
    log.Printf("📊 Args: %v", args)
}
```

## Security Checklist

- [ ] Validate all user inputs
- [ ] Use parameterized queries (already implemented)
- [ ] Enforce business_vertical_id filtering
- [ ] Implement row-level security
- [ ] Add rate limiting to API endpoints
- [ ] Sanitize file names for exports
- [ ] Validate email addresses for scheduled reports
- [ ] Implement access control for reports
- [ ] Audit report executions
- [ ] Encrypt sensitive data in exports

## Next Steps

1. **Implement Permissions**: Add ABAC/RBAC checks to report endpoints
2. **Add Report Sharing**: Implement public/private report links
3. **Create Templates**: Build pre-configured report templates
4. **Mobile Support**: Create mobile-responsive dashboard views
5. **Real-time Updates**: Add WebSocket support for live dashboards
6. **AI Insights**: Integrate ML for automated insights
7. **Data Connectors**: Add support for external data sources

## Support

For questions or issues:
- Check documentation in [docs/](../docs/)
- Review examples in [REPORT_BUILDER_GUIDE.md](./REPORT_BUILDER_GUIDE.md)
- Frontend guide: [FRONTEND_REPORT_BUILDER.md](./FRONTEND_REPORT_BUILDER.md)

## License

Copyright © 2025 UGCL. All rights reserved.

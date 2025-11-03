# Report Builder Quick Start Guide

Get up and running with the Report Builder in 5 minutes!

## 📋 Table of Contents
- [5-Minute Setup](#5-minute-setup)
- [Create Your First Report](#create-your-first-report)
- [Common Use Cases](#common-use-cases)
- [API Cheat Sheet](#api-cheat-sheet)

## 🚀 5-Minute Setup

### Backend (2 minutes)

1. **Install dependency**
```bash
go get github.com/xuri/excelize/v2
```

2. **Register routes** in `routes/routes_v2.go`:
```go
import "p9e.in/ugcl/routes"

func SetupRoutesV2(router *gin.Engine) {
    // ... existing routes ...
    routes.RegisterReportRoutes(router)
}
```

3. **Run migrations**
```bash
go run main.go  # Tables auto-created
```

### Frontend (3 minutes)

1. **Install packages**
```bash
npm install react-dnd react-dnd-html5-backend chart.js react-chartjs-2
```

2. **Copy components** from [FRONTEND_REPORT_BUILDER.md](./FRONTEND_REPORT_BUILDER.md)

3. **Add routes**
```jsx
<Route path="/reports/builder" element={<ReportBuilder />} />
<Route path="/reports/:id" element={<ReportViewer />} />
```

## 📊 Create Your First Report

### Using API (cURL)

```bash
# 1. Get available tables
curl http://localhost:8080/api/v1/reports/forms/tables \
  -H "Authorization: Bearer TOKEN"

# 2. Create a simple report
curl -X POST http://localhost:8080/api/v1/reports/definitions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "code": "my_first_report",
    "name": "My First Report",
    "report_type": "table",
    "business_vertical_id": "YOUR_BV_ID",
    "data_sources": [{
      "alias": "data",
      "table_name": "form_site_inspection",
      "form_code": "SITE_INSPECTION"
    }],
    "fields": [{
      "field_name": "site_name",
      "alias": "Site Name",
      "data_source": "data",
      "data_type": "text",
      "is_visible": true,
      "order": 1
    }]
  }'

# 3. Execute the report
curl -X POST http://localhost:8080/api/v1/reports/definitions/REPORT_ID/execute \
  -H "Authorization: Bearer TOKEN"

# 4. Export to Excel
curl http://localhost:8080/api/v1/reports/definitions/REPORT_ID/export/excel \
  -H "Authorization: Bearer TOKEN" \
  -o report.xlsx
```

### Using Frontend

```jsx
import { useState } from 'react';
import { reportService } from './services/reportService';

function QuickReport() {
  const [reportId, setReportId] = useState(null);
  const [data, setData] = useState(null);

  const createAndExecute = async () => {
    // 1. Create report
    const { data: report } = await reportService.createReport({
      code: 'quick_report',
      name: 'Quick Report',
      report_type: 'table',
      business_vertical_id: 'YOUR_BV_ID',
      data_sources: [{
        alias: 'data',
        table_name: 'form_site_inspection'
      }],
      fields: [{
        field_name: 'site_name',
        data_source: 'data',
        is_visible: true,
        order: 1
      }]
    });

    setReportId(report.report.id);

    // 2. Execute report
    const { data: result } = await reportService.executeReport(report.report.id);
    setData(result.result);
  };

  return (
    <div>
      <button onClick={createAndExecute}>Create & Run Report</button>
      {data && (
        <table>
          <thead>
            <tr>
              {data.headers.map(h => <th key={h.key}>{h.label}</th>)}
            </tr>
          </thead>
          <tbody>
            {data.data.map((row, i) => (
              <tr key={i}>
                {data.headers.map(h => <td key={h.key}>{row[h.key]}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

## 🎯 Common Use Cases

### 1. Simple List Report

Show all records from a table:

```json
{
  "name": "All Inspections",
  "report_type": "table",
  "data_sources": [{"alias": "i", "table_name": "form_site_inspection"}],
  "fields": [
    {"field_name": "site_name", "data_source": "i", "is_visible": true},
    {"field_name": "inspection_date", "data_source": "i", "is_visible": true},
    {"field_name": "status", "data_source": "i", "is_visible": true}
  ],
  "sorting": [{"field_name": "inspection_date", "direction": "DESC"}]
}
```

### 2. Filtered Report

Records from this month only:

```json
{
  "filters": [{
    "field_name": "created_at",
    "data_source": "i",
    "operator": "this_month"
  }]
}
```

### 3. Aggregation Report

Count by status:

```json
{
  "fields": [{
    "field_name": "status",
    "data_source": "i",
    "is_visible": true
  }],
  "groupings": [{
    "field_name": "status",
    "data_source": "i"
  }],
  "aggregations": [{
    "function": "COUNT",
    "field_name": "id",
    "data_source": "i",
    "alias": "total"
  }]
}
```

### 4. Chart Report

Bar chart of inspections by site:

```json
{
  "report_type": "chart",
  "chart_type": "bar",
  "fields": [
    {"field_name": "site_name", "data_source": "i", "is_visible": true},
    {"field_name": "id", "data_source": "i", "aggregation": "COUNT", "is_visible": true}
  ],
  "groupings": [{"field_name": "site_name", "data_source": "i"}]
}
```

### 5. Scheduled Daily Report

```json
{
  "is_scheduled": true,
  "schedule_config": {
    "frequency": "daily",
    "time": "08:00",
    "timezone": "Asia/Kolkata",
    "enabled": true
  },
  "recipients": ["manager@example.com"],
  "export_formats": ["excel", "pdf"]
}
```

## 📖 API Cheat Sheet

### Filter Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `eq` | Equals | `{"operator": "eq", "value": "completed"}` |
| `ne` | Not equals | `{"operator": "ne", "value": "draft"}` |
| `gt` | Greater than | `{"operator": "gt", "value": 100}` |
| `lt` | Less than | `{"operator": "lt", "value": 50}` |
| `gte` | Greater or equal | `{"operator": "gte", "value": 10}` |
| `lte` | Less or equal | `{"operator": "lte", "value": 100}` |
| `like` | Contains | `{"operator": "like", "value": "test"}` |
| `in` | In list | `{"operator": "in", "value": ["a", "b"]}` |
| `between` | Between | `{"operator": "between", "value": [1, 10]}` |
| `is_null` | Is null | `{"operator": "is_null"}` |
| `this_week` | This week | `{"operator": "this_week"}` |
| `this_month` | This month | `{"operator": "this_month"}` |
| `this_year` | This year | `{"operator": "this_year"}` |

### Aggregation Functions

| Function | Description |
|----------|-------------|
| `SUM` | Sum of values |
| `AVG` | Average |
| `COUNT` | Count rows |
| `COUNT_DISTINCT` | Count unique values |
| `MIN` | Minimum value |
| `MAX` | Maximum value |

### Report Types

| Type | Description | Use Case |
|------|-------------|----------|
| `table` | Tabular data | Lists, detailed reports |
| `chart` | Visual chart | Trends, comparisons |
| `pivot` | Pivot table | Cross-tabulation |
| `kpi` | KPI widget | Key metrics |
| `dashboard` | Multi-widget | Overview dashboards |

### Chart Types

- `bar` - Bar chart
- `line` - Line chart
- `pie` - Pie chart
- `doughnut` - Doughnut chart
- `area` - Area chart
- `scatter` - Scatter plot

## 🔧 Quick Fixes

### Report returns no data
```json
// Remove filters temporarily
{
  "filters": []
}
```

### Query too slow
```json
// Add date filter to reduce data
{
  "filters": [{
    "field_name": "created_at",
    "operator": "this_month"
  }]
}
```

### Export fails
```bash
# Install excelize
go get github.com/xuri/excelize/v2
```

## 💡 Pro Tips

1. **Start Simple**: Begin with a basic table report, then add filters
2. **Test Filters**: Use runtime filters to test before saving
3. **Use Aliases**: Give fields friendly names with `alias`
4. **Group Before Aggregate**: Always add grouping when using aggregations
5. **Cache Reports**: Frequently-used reports are auto-cached for 5 minutes
6. **Schedule Wisely**: Don't over-schedule; daily reports are usually sufficient

## 🎨 Example Dashboard

```jsx
function MyDashboard() {
  return (
    <div className="dashboard">
      <div className="row">
        <KPIWidget reportId="total-inspections" title="Total Inspections" />
        <KPIWidget reportId="pending-tasks" title="Pending Tasks" />
        <KPIWidget reportId="completion-rate" title="Completion Rate" />
      </div>
      <div className="row">
        <ChartView reportId="inspections-trend" chartType="line" />
        <ChartView reportId="status-breakdown" chartType="pie" />
      </div>
      <div className="row">
        <TableView reportId="recent-inspections" />
      </div>
    </div>
  );
}
```

## 📚 Learn More

- [Full Documentation](./REPORT_BUILDER_GUIDE.md)
- [Frontend Guide](./FRONTEND_REPORT_BUILDER.md)
- [Installation Guide](./REPORT_BUILDER_INSTALLATION.md)

## 🤝 Need Help?

Common questions:
1. How do I join multiple tables? → See "Multi-Table Reports" in full guide
2. How do I create custom calculations? → See "Advanced Features" section
3. How do I share reports? → Use the report sharing API endpoints

Happy reporting! 📊

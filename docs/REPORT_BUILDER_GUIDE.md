# Report Builder & Analytics System

A comprehensive drag-and-drop report builder for creating insights and analytics from form submission tables.

## Table of Contents
- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Backend API](#backend-api)
- [Frontend Integration](#frontend-integration)
- [Examples](#examples)
- [Best Practices](#best-practices)

## Overview

The Report Builder system allows you to create dynamic reports, dashboards, and analytics from your dedicated form submission tables. It provides:

- **Visual Query Builder**: Drag-and-drop interface for selecting fields, filters, and aggregations
- **Multi-Table Reports**: Join data from multiple form tables
- **Rich Visualizations**: Charts, KPIs, pivot tables, and more
- **Export Options**: Excel, CSV, PDF
- **Scheduled Reports**: Auto-generate and email reports
- **Interactive Dashboards**: Real-time analytics with drill-downs

## Features

### 1. Report Types

- **Table Reports**: Traditional tabular data display
- **Chart Reports**: Bar, line, pie, area, scatter charts
- **KPI Reports**: Key performance indicators with trends
- **Pivot Tables**: Cross-tabulation and aggregation
- **Dashboards**: Multi-widget analytics pages

### 2. Data Operations

- **Filtering**: Advanced filters with operators (eq, gt, lt, in, between, like, date ranges)
- **Grouping**: GROUP BY for aggregations
- **Aggregations**: SUM, AVG, COUNT, MIN, MAX, COUNT_DISTINCT
- **Sorting**: Multi-column sorting
- **Calculations**: Custom computed fields

### 3. Export & Scheduling

- **Formats**: Excel (.xlsx), CSV, PDF
- **Scheduling**: Daily, weekly, monthly automated reports
- **Email Distribution**: Send reports to multiple recipients

## Architecture

### Database Schema

```
report_definitions
├── id (UUID)
├── code (unique)
├── name
├── report_type (table, chart, pivot, dashboard, kpi)
├── data_sources (JSONB)
├── fields (JSONB)
├── filters (JSONB)
├── groupings (JSONB)
├── aggregations (JSONB)
├── sorting (JSONB)
└── schedule_config (JSONB)

dashboards
├── id (UUID)
├── code (unique)
├── name
└── widgets (relation to report_widgets)

report_executions (audit trail)
├── report_id
├── execution_type
├── status
└── row_count
```

### Component Structure

```
handlers/
├── report_engine.go       # SQL query builder & executor
├── report_handlers.go     # API endpoints
├── report_export.go       # Export to Excel/CSV/PDF
└── report_scheduler.go    # Scheduled execution

models/
└── report_builder.go      # Data models

utils/
└── analytics.go           # Statistical functions
```

## Backend API

### Report Definition Endpoints

#### Create Report
```http
POST /api/v1/reports/definitions
Content-Type: application/json

{
  "code": "monthly_site_inspections",
  "name": "Monthly Site Inspections Report",
  "description": "Site inspections completed this month",
  "report_type": "table",
  "business_vertical_id": "uuid",
  "data_sources": [
    {
      "alias": "inspections",
      "table_name": "form_site_inspection",
      "form_code": "SITE_INSPECTION",
      "form_id": "uuid"
    }
  ],
  "fields": [
    {
      "field_name": "site_name",
      "alias": "Site Name",
      "data_source": "inspections",
      "data_type": "text",
      "is_visible": true,
      "order": 1
    },
    {
      "field_name": "inspection_date",
      "alias": "Date",
      "data_source": "inspections",
      "data_type": "date",
      "is_visible": true,
      "order": 2
    },
    {
      "field_name": "status",
      "alias": "Status",
      "data_source": "inspections",
      "data_type": "text",
      "is_visible": true,
      "order": 3
    }
  ],
  "filters": [
    {
      "field_name": "created_at",
      "data_source": "inspections",
      "operator": "this_month",
      "logical_op": "AND"
    }
  ],
  "sorting": [
    {
      "field_name": "inspection_date",
      "data_source": "inspections",
      "direction": "DESC",
      "order": 1
    }
  ]
}
```

#### Execute Report
```http
POST /api/v1/reports/definitions/:id/execute
Content-Type: application/json

{
  "filters": [
    {
      "field_name": "site_id",
      "data_source": "inspections",
      "operator": "eq",
      "value": "uuid"
    }
  ]
}
```

**Response:**
```json
{
  "report": { /* report definition */ },
  "result": {
    "headers": [
      {
        "key": "site_name",
        "label": "Site Name",
        "data_type": "text",
        "sortable": true,
        "filterable": true
      }
    ],
    "data": [
      {
        "site_name": "Site A",
        "inspection_date": "2025-11-01",
        "status": "Completed"
      }
    ],
    "summary": {
      "total_records": 45
    },
    "metadata": {
      "total_rows": 45,
      "execution_time_ms": 125,
      "generated_at": "2025-11-03T10:30:00Z"
    }
  }
}
```

#### Export Report
```http
GET /api/v1/reports/definitions/:id/export/excel
GET /api/v1/reports/definitions/:id/export/csv
GET /api/v1/reports/definitions/:id/export/pdf
```

### Dashboard Endpoints

#### Create Dashboard
```http
POST /api/v1/dashboards
Content-Type: application/json

{
  "code": "operations_dashboard",
  "name": "Operations Dashboard",
  "description": "Daily operations overview",
  "business_vertical_id": "uuid",
  "layout": {
    "cols": 12,
    "rows": "auto"
  },
  "is_public": false
}
```

#### Add Widget to Dashboard
```http
POST /api/v1/dashboards/:id/widgets
Content-Type: application/json

{
  "report_id": "uuid",
  "title": "Total Inspections",
  "position": {
    "x": 0,
    "y": 0,
    "w": 6,
    "h": 4
  },
  "refresh_rate": 300
}
```

### Scheduling Endpoints

#### Schedule Report
```http
POST /api/v1/scheduled-reports/:id/schedule
Content-Type: application/json

{
  "frequency": "daily",
  "time": "08:00",
  "timezone": "Asia/Kolkata",
  "recipients": [
    "manager@example.com",
    "supervisor@example.com"
  ],
  "export_formats": ["excel", "pdf"]
}
```

For weekly reports:
```json
{
  "frequency": "weekly",
  "time": "09:00",
  "day_of_week": 1,
  "timezone": "Asia/Kolkata",
  "recipients": ["team@example.com"],
  "export_formats": ["excel"]
}
```

For monthly reports:
```json
{
  "frequency": "monthly",
  "time": "07:00",
  "day_of_month": 1,
  "timezone": "Asia/Kolkata",
  "recipients": ["director@example.com"],
  "export_formats": ["pdf", "excel"]
}
```

## Frontend Integration

### React Example (Basic Report Builder)

```jsx
import React, { useState, useEffect } from 'react';
import axios from 'axios';

function ReportBuilder() {
  const [tables, setTables] = useState([]);
  const [selectedTable, setSelectedTable] = useState(null);
  const [fields, setFields] = useState([]);
  const [selectedFields, setSelectedFields] = useState([]);
  const [filters, setFilters] = useState([]);

  // Fetch available form tables
  useEffect(() => {
    axios.get('/api/v1/reports/forms/tables')
      .then(res => setTables(res.data.tables));
  }, []);

  // Fetch fields when table is selected
  useEffect(() => {
    if (selectedTable) {
      axios.get(`/api/v1/reports/forms/tables/${selectedTable}/fields`)
        .then(res => setFields(res.data.fields));
    }
  }, [selectedTable]);

  const handleFieldDrop = (field) => {
    setSelectedFields([...selectedFields, field]);
  };

  const addFilter = (field, operator, value) => {
    setFilters([...filters, {
      field_name: field,
      data_source: selectedTable,
      operator,
      value
    }]);
  };

  const saveReport = async () => {
    const reportConfig = {
      code: `custom_${Date.now()}`,
      name: 'My Custom Report',
      report_type: 'table',
      business_vertical_id: 'your-vertical-id',
      data_sources: [{
        alias: selectedTable,
        table_name: selectedTable,
        form_code: 'FORM_CODE'
      }],
      fields: selectedFields.map((field, idx) => ({
        field_name: field.name,
        alias: field.name,
        data_source: selectedTable,
        data_type: field.type,
        is_visible: true,
        order: idx
      })),
      filters
    };

    try {
      const response = await axios.post('/api/v1/reports/definitions', reportConfig);
      console.log('Report saved:', response.data);
    } catch (error) {
      console.error('Error saving report:', error);
    }
  };

  return (
    <div className="report-builder">
      <div className="sidebar">
        <h3>Tables</h3>
        {tables.map(table => (
          <div key={table.table_name} onClick={() => setSelectedTable(table.table_name)}>
            {table.form_title}
          </div>
        ))}

        <h3>Fields</h3>
        {fields.map(field => (
          <div
            key={field.name}
            draggable
            onDragEnd={() => handleFieldDrop(field)}
          >
            {field.name} ({field.type})
          </div>
        ))}
      </div>

      <div className="canvas">
        <h2>Selected Fields</h2>
        {selectedFields.map((field, idx) => (
          <div key={idx}>{field.name}</div>
        ))}

        <button onClick={saveReport}>Save Report</button>
      </div>
    </div>
  );
}

export default ReportBuilder;
```

### Execute and Display Report

```jsx
function ReportViewer({ reportId }) {
  const [reportData, setReportData] = useState(null);
  const [loading, setLoading] = useState(false);

  const executeReport = async (runtimeFilters = []) => {
    setLoading(true);
    try {
      const response = await axios.post(
        `/api/v1/reports/definitions/${reportId}/execute`,
        { filters: runtimeFilters }
      );
      setReportData(response.data.result);
    } catch (error) {
      console.error('Error executing report:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    executeReport();
  }, [reportId]);

  if (loading) return <div>Loading...</div>;
  if (!reportData) return null;

  return (
    <div>
      <h2>Report Results</h2>
      <p>Total: {reportData.metadata.total_rows} rows in {reportData.metadata.execution_time_ms}ms</p>

      <table>
        <thead>
          <tr>
            {reportData.headers.map(header => (
              <th key={header.key}>{header.label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {reportData.data.map((row, idx) => (
            <tr key={idx}>
              {reportData.headers.map(header => (
                <td key={header.key}>{row[header.key]}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      <div className="export-buttons">
        <button onClick={() => window.open(`/api/v1/reports/definitions/${reportId}/export/excel`)}>
          Export to Excel
        </button>
        <button onClick={() => window.open(`/api/v1/reports/definitions/${reportId}/export/csv`)}>
          Export to CSV
        </button>
      </div>
    </div>
  );
}
```

### Chart Visualization

```jsx
import { Line, Bar, Pie } from 'react-chartjs-2';

function ChartReport({ reportId, chartType }) {
  const [chartData, setChartData] = useState(null);

  useEffect(() => {
    axios.post(`/api/v1/reports/definitions/${reportId}/execute`)
      .then(res => {
        const result = res.data.result;

        // Transform data for chart
        const chartConfig = {
          labels: result.data.map(row => row[result.headers[0].key]),
          datasets: [{
            label: result.headers[1].label,
            data: result.data.map(row => row[result.headers[1].key]),
            backgroundColor: 'rgba(54, 162, 235, 0.6)',
            borderColor: 'rgba(54, 162, 235, 1)',
            borderWidth: 1
          }]
        };

        setChartData(chartConfig);
      });
  }, [reportId]);

  if (!chartData) return <div>Loading chart...</div>;

  const ChartComponent = {
    line: Line,
    bar: Bar,
    pie: Pie
  }[chartType] || Bar;

  return <ChartComponent data={chartData} />;
}
```

## Examples

### Example 1: Aggregation Report

```json
{
  "name": "Inspections by Status",
  "report_type": "table",
  "data_sources": [
    {
      "alias": "inspections",
      "table_name": "form_site_inspection"
    }
  ],
  "fields": [
    {
      "field_name": "current_state",
      "alias": "Status",
      "data_source": "inspections",
      "data_type": "text",
      "is_visible": true
    }
  ],
  "groupings": [
    {
      "field_name": "current_state",
      "data_source": "inspections",
      "order": 1
    }
  ],
  "aggregations": [
    {
      "function": "COUNT",
      "field_name": "id",
      "data_source": "inspections",
      "alias": "total_count"
    }
  ]
}
```

### Example 2: Multi-Table Join Report

```json
{
  "name": "Sites with Latest Inspection",
  "report_type": "table",
  "data_sources": [
    {
      "alias": "sites",
      "table_name": "sites"
    },
    {
      "alias": "inspections",
      "table_name": "form_site_inspection",
      "join_type": "LEFT",
      "join_on": "inspections.site_id = sites.id"
    }
  ],
  "fields": [
    {
      "field_name": "name",
      "data_source": "sites",
      "alias": "Site Name",
      "is_visible": true
    },
    {
      "field_name": "inspection_date",
      "data_source": "inspections",
      "alias": "Last Inspection",
      "aggregation": "MAX",
      "is_visible": true
    }
  ],
  "groupings": [
    {
      "field_name": "name",
      "data_source": "sites"
    }
  ]
}
```

### Example 3: Date Range Report with Filters

```json
{
  "name": "This Month's Completed Work",
  "report_type": "table",
  "filters": [
    {
      "field_name": "created_at",
      "data_source": "work",
      "operator": "this_month"
    },
    {
      "field_name": "current_state",
      "data_source": "work",
      "operator": "eq",
      "value": "completed",
      "logical_op": "AND"
    }
  ]
}
```

## Best Practices

### 1. Performance Optimization

- **Use Indexes**: Ensure your form tables have indexes on frequently filtered columns
- **Limit Data**: Use filters to reduce dataset size
- **Pagination**: For large datasets, implement pagination in frontend
- **Caching**: Cache frequently accessed reports

### 2. Security

- **Access Control**: Verify business_vertical_id in filters
- **SQL Injection**: The query builder uses parameterized queries
- **Permissions**: Implement permission checks for sensitive reports

### 3. Report Design

- **Meaningful Names**: Use descriptive report names and codes
- **Categories**: Organize reports by category
- **Templates**: Create templates for common report patterns
- **Documentation**: Add descriptions to help users understand reports

### 4. Scheduling

- **Timezone**: Always specify timezone for scheduled reports
- **Recipients**: Use distribution lists for team reports
- **Format**: Choose appropriate export formats for audience
- **Frequency**: Don't over-schedule; use appropriate intervals

## Advanced Features

### Custom Calculations

```json
{
  "calculations": [
    {
      "name": "efficiency_rate",
      "expression": "(completed_count / total_count) * 100",
      "data_type": "decimal",
      "format": "0.00"
    }
  ]
}
```

### Pivot Tables

Use the analytics engine to create pivot tables:

```javascript
const analytics = new AnalyticsEngine();
const pivotData = analytics.PivotTable(
  reportData,
  'site_name',      // Row field
  'month',          // Column field
  'inspection_count', // Value field
  'sum'             // Aggregation
);
```

### KPI Tracking

```javascript
const kpi = analytics.CalculateKPI(
  currentMonthValue,
  previousMonthValue,
  targetValue
);

console.log(kpi);
// {
//   current_value: 150,
//   previous_value: 120,
//   change: 30,
//   change_percent: 25,
//   trend: "up",
//   status: "good",
//   target_progress: 75
// }
```

## Troubleshooting

### Common Issues

1. **Report execution fails**
   - Check table name exists
   - Verify field names match database columns
   - Ensure business_vertical_id is valid

2. **Empty results**
   - Review filter conditions
   - Check date ranges
   - Verify data exists in tables

3. **Slow performance**
   - Add database indexes
   - Reduce date range
   - Limit number of joined tables

## Next Steps

1. **Implement frontend drag-and-drop UI** using React DnD or similar
2. **Add more chart types** (scatter, heatmap, treemap)
3. **Implement report sharing** with public links
4. **Add data export to Google Sheets** or other destinations
5. **Create mobile-responsive dashboard views**

## Support

For issues or questions, please refer to:
- API Documentation: `/api/docs`
- GitHub Issues: [Project Repository]
- Email: support@example.com

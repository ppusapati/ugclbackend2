# Frontend Report Builder Implementation Guide

Complete guide for building a drag-and-drop report builder UI.

## Table of Contents
- [Component Architecture](#component-architecture)
- [Drag & Drop Implementation](#drag--drop-implementation)
- [Report Builder UI](#report-builder-ui)
- [Dashboard Builder](#dashboard-builder)
- [Visualization Components](#visualization-components)

## Component Architecture

```
src/
├── components/
│   ├── ReportBuilder/
│   │   ├── index.jsx                 # Main builder component
│   │   ├── DataSourceSelector.jsx   # Table selection
│   │   ├── FieldSelector.jsx        # Drag-drop field selection
│   │   ├── FilterBuilder.jsx        # Filter configuration
│   │   ├── AggregationBuilder.jsx   # Grouping & aggregations
│   │   ├── SortingBuilder.jsx       # Sort configuration
│   │   └── ReportPreview.jsx        # Live preview
│   ├── ReportViewer/
│   │   ├── index.jsx                 # Report display
│   │   ├── TableView.jsx            # Table renderer
│   │   ├── ChartView.jsx            # Chart renderer
│   │   ├── KPIView.jsx              # KPI widgets
│   │   └── PivotView.jsx            # Pivot table
│   ├── Dashboard/
│   │   ├── index.jsx                 # Dashboard container
│   │   ├── DashboardBuilder.jsx     # Dashboard editor
│   │   ├── WidgetContainer.jsx      # Widget wrapper
│   │   └── GridLayout.jsx           # Responsive grid
│   └── shared/
│       ├── ExportMenu.jsx           # Export options
│       ├── ScheduleConfig.jsx       # Schedule setup
│       └── FilterPanel.jsx          # Runtime filters
└── hooks/
    ├── useReportBuilder.js          # Report builder logic
    ├── useReportExecutor.js         # Report execution
    └── useDashboard.js              # Dashboard state
```

## Drag & Drop Implementation

### Using React DnD

```bash
npm install react-dnd react-dnd-html5-backend
```

### FieldSelector Component

```jsx
import React from 'react';
import { useDrag } from 'react-dnd';

const DraggableField = ({ field }) => {
  const [{ isDragging }, drag] = useDrag(() => ({
    type: 'FIELD',
    item: { field },
    collect: (monitor) => ({
      isDragging: !!monitor.isDragging(),
    }),
  }));

  return (
    <div
      ref={drag}
      className={`field-item ${isDragging ? 'dragging' : ''}`}
      style={{ opacity: isDragging ? 0.5 : 1 }}
    >
      <span className="field-icon">{getIconForType(field.type)}</span>
      <span className="field-name">{field.name}</span>
      <span className="field-type">{field.type}</span>
    </div>
  );
};

const FieldSelector = ({ tableName, onFieldSelect }) => {
  const [fields, setFields] = useState([]);

  useEffect(() => {
    if (tableName) {
      fetch(`/api/v1/reports/forms/tables/${tableName}/fields`)
        .then(res => res.json())
        .then(data => setFields(data.fields));
    }
  }, [tableName]);

  return (
    <div className="field-selector">
      <h3>Available Fields</h3>
      <div className="field-list">
        {fields.map(field => (
          <DraggableField key={field.name} field={field} />
        ))}
      </div>
    </div>
  );
};

function getIconForType(type) {
  const icons = {
    text: '📝',
    number: '🔢',
    date: '📅',
    boolean: '✓',
    json: '{}',
  };
  return icons[type] || '📄';
}

export default FieldSelector;
```

### Drop Zone for Selected Fields

```jsx
import { useDrop } from 'react-dnd';

const FieldDropZone = ({ selectedFields, onFieldDrop, onFieldRemove }) => {
  const [{ isOver }, drop] = useDrop(() => ({
    accept: 'FIELD',
    drop: (item) => onFieldDrop(item.field),
    collect: (monitor) => ({
      isOver: !!monitor.isOver(),
    }),
  }));

  return (
    <div
      ref={drop}
      className={`field-drop-zone ${isOver ? 'drop-active' : ''}`}
    >
      <h3>Selected Fields</h3>
      {selectedFields.length === 0 ? (
        <p className="empty-state">Drag fields here</p>
      ) : (
        <div className="selected-fields">
          {selectedFields.map((field, index) => (
            <div key={index} className="selected-field">
              <span className="drag-handle">⋮⋮</span>
              <span className="field-name">{field.name}</span>
              <select
                value={field.aggregation || ''}
                onChange={(e) => updateFieldAggregation(index, e.target.value)}
              >
                <option value="">No Aggregation</option>
                <option value="SUM">Sum</option>
                <option value="AVG">Average</option>
                <option value="COUNT">Count</option>
                <option value="MIN">Min</option>
                <option value="MAX">Max</option>
              </select>
              <button onClick={() => onFieldRemove(index)}>✕</button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
```

## Report Builder UI

### Main Report Builder Component

```jsx
import React, { useState } from 'react';
import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';
import DataSourceSelector from './DataSourceSelector';
import FieldSelector from './FieldSelector';
import FieldDropZone from './FieldDropZone';
import FilterBuilder from './FilterBuilder';
import ReportPreview from './ReportPreview';

const ReportBuilder = () => {
  const [reportConfig, setReportConfig] = useState({
    name: '',
    report_type: 'table',
    data_sources: [],
    fields: [],
    filters: [],
    groupings: [],
    aggregations: [],
    sorting: [],
  });

  const [previewData, setPreviewData] = useState(null);

  const handleDataSourceChange = (sources) => {
    setReportConfig(prev => ({ ...prev, data_sources: sources }));
  };

  const handleFieldDrop = (field) => {
    const newField = {
      field_name: field.name,
      alias: field.name,
      data_source: reportConfig.data_sources[0]?.alias || '',
      data_type: field.type,
      is_visible: true,
      order: reportConfig.fields.length,
    };
    setReportConfig(prev => ({
      ...prev,
      fields: [...prev.fields, newField]
    }));
  };

  const handleFieldRemove = (index) => {
    setReportConfig(prev => ({
      ...prev,
      fields: prev.fields.filter((_, i) => i !== index)
    }));
  };

  const handleAddFilter = (filter) => {
    setReportConfig(prev => ({
      ...prev,
      filters: [...prev.filters, filter]
    }));
  };

  const handlePreview = async () => {
    // Create temporary report
    const tempReport = {
      ...reportConfig,
      code: `preview_${Date.now()}`,
      business_vertical_id: 'your-vertical-id',
    };

    try {
      const createRes = await fetch('/api/v1/reports/definitions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(tempReport),
      });
      const { report } = await createRes.json();

      const executeRes = await fetch(`/api/v1/reports/definitions/${report.id}/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      const { result } = await executeRes.json();
      setPreviewData(result);
    } catch (error) {
      console.error('Preview error:', error);
    }
  };

  const handleSave = async () => {
    const finalReport = {
      ...reportConfig,
      code: reportConfig.name.toLowerCase().replace(/\s+/g, '_'),
      business_vertical_id: 'your-vertical-id',
    };

    try {
      const response = await fetch('/api/v1/reports/definitions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(finalReport),
      });
      const data = await response.json();
      console.log('Report saved:', data);
      // Navigate to report list or viewer
    } catch (error) {
      console.error('Save error:', error);
    }
  };

  return (
    <DndProvider backend={HTML5Backend}>
      <div className="report-builder-container">
        <div className="builder-header">
          <input
            type="text"
            placeholder="Report Name"
            value={reportConfig.name}
            onChange={(e) => setReportConfig(prev => ({ ...prev, name: e.target.value }))}
            className="report-name-input"
          />
          <div className="header-actions">
            <button onClick={handlePreview} className="btn-preview">Preview</button>
            <button onClick={handleSave} className="btn-save">Save Report</button>
          </div>
        </div>

        <div className="builder-content">
          <div className="builder-sidebar">
            <DataSourceSelector onChange={handleDataSourceChange} />
            <FieldSelector
              tableName={reportConfig.data_sources[0]?.table_name}
              onFieldSelect={handleFieldDrop}
            />
          </div>

          <div className="builder-main">
            <div className="config-section">
              <FieldDropZone
                selectedFields={reportConfig.fields}
                onFieldDrop={handleFieldDrop}
                onFieldRemove={handleFieldRemove}
              />
            </div>

            <div className="config-section">
              <FilterBuilder
                dataSource={reportConfig.data_sources[0]?.alias}
                fields={reportConfig.fields}
                filters={reportConfig.filters}
                onAddFilter={handleAddFilter}
              />
            </div>

            {previewData && (
              <div className="config-section">
                <ReportPreview data={previewData} />
              </div>
            )}
          </div>
        </div>
      </div>
    </DndProvider>
  );
};

export default ReportBuilder;
```

### Filter Builder Component

```jsx
const FilterBuilder = ({ dataSource, fields, filters, onAddFilter }) => {
  const [newFilter, setNewFilter] = useState({
    field_name: '',
    operator: 'eq',
    value: '',
  });

  const operators = [
    { value: 'eq', label: 'Equals' },
    { value: 'ne', label: 'Not Equals' },
    { value: 'gt', label: 'Greater Than' },
    { value: 'lt', label: 'Less Than' },
    { value: 'gte', label: 'Greater or Equal' },
    { value: 'lte', label: 'Less or Equal' },
    { value: 'like', label: 'Contains' },
    { value: 'in', label: 'In List' },
    { value: 'between', label: 'Between' },
    { value: 'is_null', label: 'Is Null' },
    { value: 'is_not_null', label: 'Is Not Null' },
    { value: 'this_week', label: 'This Week' },
    { value: 'this_month', label: 'This Month' },
    { value: 'this_year', label: 'This Year' },
  ];

  const handleAddFilter = () => {
    if (newFilter.field_name && newFilter.operator) {
      onAddFilter({
        ...newFilter,
        data_source: dataSource,
        logical_op: 'AND',
      });
      setNewFilter({ field_name: '', operator: 'eq', value: '' });
    }
  };

  return (
    <div className="filter-builder">
      <h3>Filters</h3>

      <div className="filter-form">
        <select
          value={newFilter.field_name}
          onChange={(e) => setNewFilter(prev => ({ ...prev, field_name: e.target.value }))}
        >
          <option value="">Select Field</option>
          {fields.map(field => (
            <option key={field.field_name} value={field.field_name}>
              {field.alias || field.field_name}
            </option>
          ))}
        </select>

        <select
          value={newFilter.operator}
          onChange={(e) => setNewFilter(prev => ({ ...prev, operator: e.target.value }))}
        >
          {operators.map(op => (
            <option key={op.value} value={op.value}>{op.label}</option>
          ))}
        </select>

        {!['is_null', 'is_not_null', 'this_week', 'this_month', 'this_year'].includes(newFilter.operator) && (
          <input
            type="text"
            value={newFilter.value}
            onChange={(e) => setNewFilter(prev => ({ ...prev, value: e.target.value }))}
            placeholder="Value"
          />
        )}

        <button onClick={handleAddFilter}>Add Filter</button>
      </div>

      <div className="active-filters">
        {filters.map((filter, index) => (
          <div key={index} className="filter-chip">
            {filter.field_name} {filter.operator} {filter.value}
            <button onClick={() => onRemoveFilter(index)}>✕</button>
          </div>
        ))}
      </div>
    </div>
  );
};
```

## Dashboard Builder

### Dashboard with React Grid Layout

```bash
npm install react-grid-layout
```

```jsx
import React, { useState } from 'react';
import GridLayout from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';

const DashboardBuilder = () => {
  const [widgets, setWidgets] = useState([]);
  const [layout, setLayout] = useState([]);

  const addWidget = (reportId, title) => {
    const newWidget = {
      i: `widget-${Date.now()}`,
      x: 0,
      y: Infinity, // puts it at the bottom
      w: 6,
      h: 4,
      reportId,
      title,
    };

    setWidgets([...widgets, newWidget]);
    setLayout([...layout, { i: newWidget.i, x: newWidget.x, y: newWidget.y, w: newWidget.w, h: newWidget.h }]);
  };

  const onLayoutChange = (newLayout) => {
    setLayout(newLayout);
  };

  const saveDashboard = async () => {
    const dashboardConfig = {
      code: 'my_dashboard',
      name: 'My Dashboard',
      business_vertical_id: 'your-vertical-id',
      layout: { cols: 12, rows: 'auto' },
    };

    // Create dashboard
    const dashRes = await fetch('/api/v1/dashboards', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(dashboardConfig),
    });
    const { dashboard } = await dashRes.json();

    // Add widgets
    for (const widget of widgets) {
      const widgetConfig = {
        report_id: widget.reportId,
        title: widget.title,
        position: layout.find(l => l.i === widget.i),
        refresh_rate: 300,
      };

      await fetch(`/api/v1/dashboards/${dashboard.id}/widgets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(widgetConfig),
      });
    }
  };

  return (
    <div className="dashboard-builder">
      <div className="dashboard-header">
        <h2>Dashboard Builder</h2>
        <button onClick={saveDashboard}>Save Dashboard</button>
      </div>

      <GridLayout
        className="layout"
        layout={layout}
        cols={12}
        rowHeight={30}
        width={1200}
        onLayoutChange={onLayoutChange}
        draggableHandle=".widget-header"
      >
        {widgets.map(widget => (
          <div key={widget.i} className="widget-container">
            <div className="widget-header">
              <h3>{widget.title}</h3>
              <button onClick={() => removeWidget(widget.i)}>✕</button>
            </div>
            <div className="widget-content">
              <ReportViewer reportId={widget.reportId} />
            </div>
          </div>
        ))}
      </GridLayout>

      <div className="widget-selector">
        <h3>Add Widget</h3>
        <ReportList onSelectReport={(reportId, title) => addWidget(reportId, title)} />
      </div>
    </div>
  );
};
```

## Visualization Components

### Chart Component with Chart.js

```bash
npm install chart.js react-chartjs-2
```

```jsx
import { Line, Bar, Pie, Doughnut } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend
);

const ChartView = ({ reportId, chartType = 'bar' }) => {
  const [chartData, setChartData] = useState(null);

  useEffect(() => {
    fetch(`/api/v1/reports/definitions/${reportId}/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    })
      .then(res => res.json())
      .then(({ result }) => {
        const transformed = {
          labels: result.data.map(row => row[result.headers[0].key]),
          datasets: [
            {
              label: result.headers[1].label,
              data: result.data.map(row => row[result.headers[1].key]),
              backgroundColor: generateColors(result.data.length),
              borderColor: generateBorderColors(result.data.length),
              borderWidth: 1,
            },
          ],
        };
        setChartData(transformed);
      });
  }, [reportId]);

  if (!chartData) return <div>Loading...</div>;

  const ChartComponent = {
    line: Line,
    bar: Bar,
    pie: Pie,
    doughnut: Doughnut,
  }[chartType] || Bar;

  const options = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: 'top' },
      title: { display: false },
    },
  };

  return (
    <div style={{ height: '300px' }}>
      <ChartComponent data={chartData} options={options} />
    </div>
  );
};

function generateColors(count) {
  const baseColors = [
    'rgba(54, 162, 235, 0.6)',
    'rgba(75, 192, 192, 0.6)',
    'rgba(255, 206, 86, 0.6)',
    'rgba(255, 99, 132, 0.6)',
    'rgba(153, 102, 255, 0.6)',
  ];
  return Array(count).fill(0).map((_, i) => baseColors[i % baseColors.length]);
}

function generateBorderColors(count) {
  const baseColors = [
    'rgba(54, 162, 235, 1)',
    'rgba(75, 192, 192, 1)',
    'rgba(255, 206, 86, 1)',
    'rgba(255, 99, 132, 1)',
    'rgba(153, 102, 255, 1)',
  ];
  return Array(count).fill(0).map((_, i) => baseColors[i % baseColors.length]);
}
```

### KPI Widget

```jsx
const KPIWidget = ({ reportId, title }) => {
  const [kpi, setKpi] = useState(null);

  useEffect(() => {
    fetch(`/api/v1/reports/definitions/${reportId}/execute`, {
      method: 'POST',
    })
      .then(res => res.json())
      .then(({ result }) => {
        if (result.data.length > 0) {
          const currentValue = result.data[0][result.headers[0].key];
          const previousValue = result.summary.previous_value || 0;

          setKpi({
            value: currentValue,
            change: currentValue - previousValue,
            changePercent: ((currentValue - previousValue) / previousValue * 100).toFixed(2),
            trend: currentValue > previousValue ? 'up' : 'down',
          });
        }
      });
  }, [reportId]);

  if (!kpi) return <div>Loading...</div>;

  return (
    <div className="kpi-widget">
      <h3>{title}</h3>
      <div className="kpi-value">{kpi.value.toLocaleString()}</div>
      <div className={`kpi-change ${kpi.trend}`}>
        <span className="arrow">{kpi.trend === 'up' ? '↑' : '↓'}</span>
        <span>{Math.abs(kpi.changePercent)}%</span>
        <span className="change-value">({kpi.change > 0 ? '+' : ''}{kpi.change})</span>
      </div>
    </div>
  );
};
```

## CSS Styling Example

```css
.report-builder-container {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.builder-header {
  padding: 1rem;
  background: #fff;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.builder-content {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.builder-sidebar {
  width: 300px;
  background: #f5f5f5;
  border-right: 1px solid #e0e0e0;
  overflow-y: auto;
  padding: 1rem;
}

.builder-main {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
}

.field-item {
  padding: 0.75rem;
  margin: 0.5rem 0;
  background: white;
  border: 1px solid #ddd;
  border-radius: 4px;
  cursor: move;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.field-item.dragging {
  opacity: 0.5;
}

.field-drop-zone {
  min-height: 200px;
  padding: 1rem;
  border: 2px dashed #ccc;
  border-radius: 8px;
  background: #fafafa;
}

.field-drop-zone.drop-active {
  border-color: #4caf50;
  background: #e8f5e9;
}

.selected-field {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  background: white;
  border: 1px solid #ddd;
  margin: 0.5rem 0;
  border-radius: 4px;
}

.kpi-widget {
  text-align: center;
  padding: 2rem;
}

.kpi-value {
  font-size: 3rem;
  font-weight: bold;
  color: #333;
}

.kpi-change {
  font-size: 1.2rem;
  margin-top: 0.5rem;
}

.kpi-change.up {
  color: #4caf50;
}

.kpi-change.down {
  color: #f44336;
}
```

This provides a complete foundation for building a professional drag-and-drop report builder!

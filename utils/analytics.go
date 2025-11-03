package utils

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// AnalyticsEngine provides advanced analytics and statistical functions
type AnalyticsEngine struct{}

// NewAnalyticsEngine creates a new analytics engine
func NewAnalyticsEngine() *AnalyticsEngine {
	return &AnalyticsEngine{}
}

// ChartData represents data formatted for charts
type ChartData struct {
	Labels   []string                 `json:"labels"`
	Datasets []Dataset                `json:"datasets"`
	Options  map[string]interface{}   `json:"options,omitempty"`
}

// Dataset represents a data series
type Dataset struct {
	Label           string        `json:"label"`
	Data            []interface{} `json:"data"`
	BackgroundColor interface{}   `json:"backgroundColor,omitempty"`
	BorderColor     interface{}   `json:"borderColor,omitempty"`
	Fill            bool          `json:"fill,omitempty"`
}

// KPIMetrics represents key performance indicators
type KPIMetrics struct {
	CurrentValue    float64 `json:"current_value"`
	PreviousValue   float64 `json:"previous_value"`
	Change          float64 `json:"change"`
	ChangePercent   float64 `json:"change_percent"`
	Trend           string  `json:"trend"` // up, down, stable
	Status          string  `json:"status"` // good, warning, critical
	Target          float64 `json:"target,omitempty"`
	TargetProgress  float64 `json:"target_progress,omitempty"`
}

// TimeSeriesData represents time-series analytics
type TimeSeriesData struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Label     string    `json:"label,omitempty"`
}

// StatisticalSummary provides statistical analysis
type StatisticalSummary struct {
	Count      int     `json:"count"`
	Sum        float64 `json:"sum"`
	Mean       float64 `json:"mean"`
	Median     float64 `json:"median"`
	Mode       float64 `json:"mode,omitempty"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Range      float64 `json:"range"`
	Variance   float64 `json:"variance"`
	StdDev     float64 `json:"std_dev"`
	Q1         float64 `json:"q1"` // First quartile
	Q3         float64 `json:"q3"` // Third quartile
	IQR        float64 `json:"iqr"` // Interquartile range
}

// TransformToChartData transforms raw data into chart-ready format
func (ae *AnalyticsEngine) TransformToChartData(
	data []map[string]interface{},
	chartType string,
	xField string,
	yFields []string,
) (*ChartData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data to transform")
	}

	chartData := &ChartData{
		Labels:   []string{},
		Datasets: []Dataset{},
		Options:  make(map[string]interface{}),
	}

	// Extract labels from x-axis field
	for _, row := range data {
		if label, ok := row[xField]; ok {
			chartData.Labels = append(chartData.Labels, fmt.Sprintf("%v", label))
		}
	}

	// Create datasets for each y-field
	colors := ae.getChartColors(len(yFields))

	for i, yField := range yFields {
		dataset := Dataset{
			Label: yField,
			Data:  []interface{}{},
		}

		// Extract values
		for _, row := range data {
			if val, ok := row[yField]; ok {
				dataset.Data = append(dataset.Data, val)
			} else {
				dataset.Data = append(dataset.Data, nil)
			}
		}

		// Apply chart-specific styling
		switch chartType {
		case "line":
			dataset.BorderColor = colors[i]
			dataset.BackgroundColor = ae.addTransparency(colors[i], 0.1)
			dataset.Fill = false

		case "area":
			dataset.BorderColor = colors[i]
			dataset.BackgroundColor = ae.addTransparency(colors[i], 0.3)
			dataset.Fill = true

		case "bar":
			dataset.BackgroundColor = colors[i]
			dataset.BorderColor = colors[i]

		case "pie", "doughnut":
			dataset.BackgroundColor = colors
			dataset.BorderColor = "#ffffff"
		}

		chartData.Datasets = append(chartData.Datasets, dataset)
	}

	return chartData, nil
}

// CalculateKPI calculates KPI metrics with trend analysis
func (ae *AnalyticsEngine) CalculateKPI(
	currentValue, previousValue, target float64,
) *KPIMetrics {
	kpi := &KPIMetrics{
		CurrentValue:  currentValue,
		PreviousValue: previousValue,
		Target:        target,
	}

	// Calculate change
	kpi.Change = currentValue - previousValue

	// Calculate percentage change
	if previousValue != 0 {
		kpi.ChangePercent = (kpi.Change / previousValue) * 100
	}

	// Determine trend
	if kpi.Change > 0 {
		kpi.Trend = "up"
	} else if kpi.Change < 0 {
		kpi.Trend = "down"
	} else {
		kpi.Trend = "stable"
	}

	// Calculate target progress
	if target != 0 {
		kpi.TargetProgress = (currentValue / target) * 100

		// Determine status based on target
		if kpi.TargetProgress >= 100 {
			kpi.Status = "good"
		} else if kpi.TargetProgress >= 70 {
			kpi.Status = "warning"
		} else {
			kpi.Status = "critical"
		}
	} else {
		// Status based on trend
		if kpi.Trend == "up" {
			kpi.Status = "good"
		} else if kpi.Trend == "down" {
			kpi.Status = "warning"
		} else {
			kpi.Status = "stable"
		}
	}

	return kpi
}

// CalculateStatistics calculates comprehensive statistical summary
func (ae *AnalyticsEngine) CalculateStatistics(values []float64) *StatisticalSummary {
	if len(values) == 0 {
		return nil
	}

	// Sort values for median, quartiles
	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sort.Float64s(sortedValues)

	summary := &StatisticalSummary{
		Count: len(values),
	}

	// Calculate sum
	for _, v := range values {
		summary.Sum += v
	}

	// Calculate mean
	summary.Mean = summary.Sum / float64(summary.Count)

	// Calculate min, max, range
	summary.Min = sortedValues[0]
	summary.Max = sortedValues[len(sortedValues)-1]
	summary.Range = summary.Max - summary.Min

	// Calculate median
	summary.Median = ae.calculateMedian(sortedValues)

	// Calculate mode (most frequent value)
	summary.Mode = ae.calculateMode(values)

	// Calculate variance and standard deviation
	var sumSquaredDiff float64
	for _, v := range values {
		diff := v - summary.Mean
		sumSquaredDiff += diff * diff
	}
	summary.Variance = sumSquaredDiff / float64(summary.Count)
	summary.StdDev = math.Sqrt(summary.Variance)

	// Calculate quartiles
	summary.Q1 = ae.calculatePercentile(sortedValues, 25)
	summary.Q3 = ae.calculatePercentile(sortedValues, 75)
	summary.IQR = summary.Q3 - summary.Q1

	return summary
}

// GroupByTimePeriod groups time-series data by period
func (ae *AnalyticsEngine) GroupByTimePeriod(
	data []map[string]interface{},
	dateField string,
	valueField string,
	period string, // hour, day, week, month, quarter, year
	aggregation string, // sum, avg, count, min, max
) ([]TimeSeriesData, error) {
	grouped := make(map[string][]float64)

	for _, row := range data {
		dateVal, ok := row[dateField]
		if !ok {
			continue
		}

		var t time.Time
		switch v := dateVal.(type) {
		case time.Time:
			t = v
		case string:
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				continue
			}
			t = parsed
		default:
			continue
		}

		// Group by period
		var key string
		switch period {
		case "hour":
			key = t.Format("2006-01-02 15:00")
		case "day":
			key = t.Format("2006-01-02")
		case "week":
			year, week := t.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", year, week)
		case "month":
			key = t.Format("2006-01")
		case "quarter":
			quarter := (int(t.Month()) - 1) / 3 + 1
			key = fmt.Sprintf("%d-Q%d", t.Year(), quarter)
		case "year":
			key = t.Format("2006")
		default:
			key = t.Format("2006-01-02")
		}

		// Extract value
		if val, ok := row[valueField]; ok {
			floatVal := ae.toFloat64(val)
			grouped[key] = append(grouped[key], floatVal)
		} else {
			// Count if no value field specified
			grouped[key] = append(grouped[key], 1)
		}
	}

	// Calculate aggregations
	var result []TimeSeriesData
	for key, values := range grouped {
		ts := TimeSeriesData{
			Label: key,
		}

		// Parse timestamp back
		var err error
		switch period {
		case "hour":
			ts.Timestamp, err = time.Parse("2006-01-02 15:00", key)
		case "day":
			ts.Timestamp, err = time.Parse("2006-01-02", key)
		case "month":
			ts.Timestamp, err = time.Parse("2006-01", key+"-01")
		case "year":
			ts.Timestamp, err = time.Parse("2006", key)
		default:
			ts.Timestamp, err = time.Parse("2006-01-02", key)
		}

		if err != nil {
			ts.Timestamp = time.Now()
		}

		// Apply aggregation
		switch aggregation {
		case "sum":
			for _, v := range values {
				ts.Value += v
			}
		case "avg", "average":
			var sum float64
			for _, v := range values {
				sum += v
			}
			ts.Value = sum / float64(len(values))
		case "count":
			ts.Value = float64(len(values))
		case "min":
			ts.Value = values[0]
			for _, v := range values {
				if v < ts.Value {
					ts.Value = v
				}
			}
		case "max":
			ts.Value = values[0]
			for _, v := range values {
				if v > ts.Value {
					ts.Value = v
				}
			}
		default:
			ts.Value = float64(len(values))
		}

		result = append(result, ts)
	}

	// Sort by timestamp
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result, nil
}

// CalculateGrowthRate calculates period-over-period growth rate
func (ae *AnalyticsEngine) CalculateGrowthRate(timeSeries []TimeSeriesData) []map[string]interface{} {
	if len(timeSeries) < 2 {
		return nil
	}

	result := []map[string]interface{}{}

	for i := 1; i < len(timeSeries); i++ {
		current := timeSeries[i]
		previous := timeSeries[i-1]

		var growthRate float64
		if previous.Value != 0 {
			growthRate = ((current.Value - previous.Value) / previous.Value) * 100
		}

		result = append(result, map[string]interface{}{
			"timestamp":   current.Timestamp,
			"label":       current.Label,
			"value":       current.Value,
			"growth_rate": growthRate,
			"trend":       ae.getTrend(growthRate),
		})
	}

	return result
}

// CalculateMovingAverage calculates moving average for smoothing trends
func (ae *AnalyticsEngine) CalculateMovingAverage(timeSeries []TimeSeriesData, window int) []TimeSeriesData {
	if window <= 0 || len(timeSeries) < window {
		return timeSeries
	}

	result := []TimeSeriesData{}

	for i := window - 1; i < len(timeSeries); i++ {
		var sum float64
		for j := i - window + 1; j <= i; j++ {
			sum += timeSeries[j].Value
		}

		avg := sum / float64(window)
		result = append(result, TimeSeriesData{
			Timestamp: timeSeries[i].Timestamp,
			Label:     timeSeries[i].Label,
			Value:     avg,
		})
	}

	return result
}

// Helper functions

func (ae *AnalyticsEngine) calculateMedian(sortedValues []float64) float64 {
	n := len(sortedValues)
	if n%2 == 0 {
		return (sortedValues[n/2-1] + sortedValues[n/2]) / 2
	}
	return sortedValues[n/2]
}

func (ae *AnalyticsEngine) calculateMode(values []float64) float64 {
	frequency := make(map[float64]int)
	for _, v := range values {
		frequency[v]++
	}

	var mode float64
	maxFreq := 0
	for val, freq := range frequency {
		if freq > maxFreq {
			maxFreq = freq
			mode = val
		}
	}

	return mode
}

func (ae *AnalyticsEngine) calculatePercentile(sortedValues []float64, percentile float64) float64 {
	if percentile <= 0 {
		return sortedValues[0]
	}
	if percentile >= 100 {
		return sortedValues[len(sortedValues)-1]
	}

	index := (percentile / 100) * float64(len(sortedValues)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sortedValues[lower]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return sortedValues[lower]*(1-weight) + sortedValues[upper]*weight
}

func (ae *AnalyticsEngine) toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	default:
		return 0
	}
}

func (ae *AnalyticsEngine) getTrend(growthRate float64) string {
	if growthRate > 0 {
		return "up"
	} else if growthRate < 0 {
		return "down"
	}
	return "stable"
}

func (ae *AnalyticsEngine) getChartColors(count int) []string {
	baseColors := []string{
		"#3B82F6", // Blue
		"#10B981", // Green
		"#F59E0B", // Amber
		"#EF4444", // Red
		"#8B5CF6", // Purple
		"#EC4899", // Pink
		"#14B8A6", // Teal
		"#F97316", // Orange
		"#6366F1", // Indigo
		"#84CC16", // Lime
	}

	colors := []string{}
	for i := 0; i < count; i++ {
		colors = append(colors, baseColors[i%len(baseColors)])
	}

	return colors
}

func (ae *AnalyticsEngine) addTransparency(color string, alpha float64) string {
	// Simple transparency addition for hex colors
	// In production, you'd want proper color parsing and RGBA conversion
	return color
}

// PivotTable creates a pivot table from data
func (ae *AnalyticsEngine) PivotTable(
	data []map[string]interface{},
	rowField string,
	colField string,
	valueField string,
	aggregation string,
) map[string]map[string]interface{} {
	pivot := make(map[string]map[string][]float64)

	// Group data
	for _, row := range data {
		rowKey := fmt.Sprintf("%v", row[rowField])
		colKey := fmt.Sprintf("%v", row[colField])
		value := ae.toFloat64(row[valueField])

		if pivot[rowKey] == nil {
			pivot[rowKey] = make(map[string][]float64)
		}
		pivot[rowKey][colKey] = append(pivot[rowKey][colKey], value)
	}

	// Apply aggregation
	result := make(map[string]map[string]interface{})
	for rowKey, cols := range pivot {
		result[rowKey] = make(map[string]interface{})
		for colKey, values := range cols {
			switch aggregation {
			case "sum":
				var sum float64
				for _, v := range values {
					sum += v
				}
				result[rowKey][colKey] = sum
			case "avg":
				var sum float64
				for _, v := range values {
					sum += v
				}
				result[rowKey][colKey] = sum / float64(len(values))
			case "count":
				result[rowKey][colKey] = len(values)
			case "min":
				min := values[0]
				for _, v := range values {
					if v < min {
						min = v
					}
				}
				result[rowKey][colKey] = min
			case "max":
				max := values[0]
				for _, v := range values {
					if v > max {
						max = v
					}
				}
				result[rowKey][colKey] = max
			}
		}
	}

	return result
}

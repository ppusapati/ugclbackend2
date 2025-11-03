package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/xuri/excelize/v2"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// ExportReportToExcel exports report data to Excel format
func ExportReportToExcel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	// Get report definition
	var report models.ReportDefinition
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	// Execute report
	engine := NewReportEngine()
	result, err := engine.ExecuteReport(&report, nil, userID.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create Excel file
	excelFile, err := createExcelFile(report.Name, result)
	if err != nil {
		http.Error(w, "Failed to generate Excel file", http.StatusInternalServerError)
		return
	}

	// Write to buffer
	buffer, err := excelFile.WriteToBuffer()
	if err != nil {
		http.Error(w, "Failed to write Excel file", http.StatusInternalServerError)
		return
	}

	// Set headers for download
	filename := fmt.Sprintf("%s_%s.xlsx", sanitizeFilename(report.Name), time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buffer.Len()))

	w.WriteHeader(http.StatusOK)
	w.Write(buffer.Bytes())
}

// ExportReportToCSV exports report data to CSV format
func ExportReportToCSV(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	// Get report definition
	var report models.ReportDefinition
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	// Execute report
	engine := NewReportEngine()
	result, err := engine.ExecuteReport(&report, nil, userID.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create CSV
	csvData, err := createCSVFile(result)
	if err != nil {
		http.Error(w, "Failed to generate CSV file", http.StatusInternalServerError)
		return
	}

	// Set headers for download
	filename := fmt.Sprintf("%s_%s.csv", sanitizeFilename(report.Name), time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(csvData)))

	w.WriteHeader(http.StatusOK)
	w.Write(csvData)
}

// ExportReportToPDF exports report data to PDF format
func ExportReportToPDF(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]
	userID := r.Context().Value("userID").(uuid.UUID)

	// Get report definition
	var report models.ReportDefinition
	if err := config.DB.Where("id = ? AND deleted_at IS NULL", reportID).First(&report).Error; err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	// Execute report
	engine := NewReportEngine()
	result, err := engine.ExecuteReport(&report, nil, userID.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate PDF
	pdfData, err := createPDFFile(report.Name, result)
	if err != nil {
		http.Error(w, "Failed to generate PDF file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers for download
	filename := fmt.Sprintf("%s_%s.pdf", sanitizeFilename(report.Name), time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfData)))

	w.WriteHeader(http.StatusOK)
	w.Write(pdfData)
}

// createExcelFile generates an Excel file from report results
func createExcelFile(reportName string, result *ReportResult) (*excelize.File, error) {
	f := excelize.NewFile()
	sheetName := "Report"

	// Create sheet
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(index)

	// Add title
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 16,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})
	f.SetCellValue(sheetName, "A1", reportName)
	f.SetCellStyle(sheetName, "A1", "A1", titleStyle)
	f.SetRowHeight(sheetName, 1, 30)

	// Add generation timestamp
	f.SetCellValue(sheetName, "A2", fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")))

	// Add headers (row 4)
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})

	for colIdx, header := range result.Headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 4)
		f.SetCellValue(sheetName, cell, header.Label)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
		f.SetColWidth(sheetName, columnIndexToLetter(colIdx+1), columnIndexToLetter(colIdx+1), 20)
	}

	// Add data rows
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})

	for rowIdx, row := range result.Data {
		for colIdx, header := range result.Headers {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+5)
			value := row[header.Key]

			// Format value based on data type
			switch header.DataType {
			case "date":
				if v, ok := value.(time.Time); ok {
					f.SetCellValue(sheetName, cell, v.Format("2006-01-02"))
				} else {
					f.SetCellValue(sheetName, cell, value)
				}
			case "datetime":
				if v, ok := value.(time.Time); ok {
					f.SetCellValue(sheetName, cell, v.Format("2006-01-02 15:04:05"))
				} else {
					f.SetCellValue(sheetName, cell, value)
				}
			default:
				f.SetCellValue(sheetName, cell, value)
			}

			f.SetCellStyle(sheetName, cell, cell, dataStyle)
		}
	}

	// Add summary section if exists
	if len(result.Summary) > 0 {
		summaryRow := len(result.Data) + 7
		summaryStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold: true,
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{"#E7E6E6"},
				Pattern: 1,
			},
		})

		cell, _ := excelize.CoordinatesToCellName(1, summaryRow)
		f.SetCellValue(sheetName, cell, "Summary")
		f.SetCellStyle(sheetName, cell, cell, summaryStyle)

		summaryRow++
		for key, value := range result.Summary {
			keyCell, _ := excelize.CoordinatesToCellName(1, summaryRow)
			valueCell, _ := excelize.CoordinatesToCellName(2, summaryRow)
			f.SetCellValue(sheetName, keyCell, key)
			f.SetCellValue(sheetName, valueCell, value)
			summaryRow++
		}
	}

	// Delete default Sheet1 if we created a new one
	f.DeleteSheet("Sheet1")

	return f, nil
}

// createCSVFile generates a CSV file from report results
func createCSVFile(result *ReportResult) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write headers
	headers := []string{}
	for _, header := range result.Headers {
		headers = append(headers, header.Label)
	}
	writer.Write(headers)

	// Write data rows
	for _, row := range result.Data {
		record := []string{}
		for _, header := range result.Headers {
			value := row[header.Key]
			record = append(record, fmt.Sprintf("%v", value))
		}
		writer.Write(record)
	}

	// Write summary section
	if len(result.Summary) > 0 {
		writer.Write([]string{}) // Empty row
		writer.Write([]string{"Summary"})
		for key, value := range result.Summary {
			writer.Write([]string{key, fmt.Sprintf("%v", value)})
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// createPDFFile generates a PDF file from report results
func createPDFFile(reportName string, result *ReportResult) ([]byte, error) {
	return nil, fmt.Errorf("PDF export requires additional PDF library setup. Consider using github.com/johnfercher/maroto or wkhtmltopdf")
}

// Helper functions

func sanitizeFilename(filename string) string {
	// Remove or replace characters that are invalid in filenames
	replacements := map[rune]rune{
		'/':  '_',
		'\\': '_',
		':':  '_',
		'*':  '_',
		'?':  '_',
		'"':  '_',
		'<':  '_',
		'>':  '_',
		'|':  '_',
		' ':  '_',
	}

	result := []rune{}
	for _, char := range filename {
		if replacement, exists := replacements[char]; exists {
			result = append(result, replacement)
		} else {
			result = append(result, char)
		}
	}

	return string(result)
}

func columnIndexToLetter(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+(col%26))) + result
		col /= 26
	}
	return result
}

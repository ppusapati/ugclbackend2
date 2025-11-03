package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// ReportScheduler handles scheduled report execution and distribution
type ReportScheduler struct {
	db           *gorm.DB
	reportEngine *ReportEngine
}

// NewReportScheduler creates a new report scheduler
func NewReportScheduler() *ReportScheduler {
	return &ReportScheduler{
		db:           config.DB,
		reportEngine: NewReportEngine(),
	}
}

// StartScheduler starts the report scheduling service
func (rs *ReportScheduler) StartScheduler() {
	log.Println("📅 Starting Report Scheduler...")

	// Run scheduler every minute to check for due reports
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rs.checkAndExecuteScheduledReports()
	}
}

// checkAndExecuteScheduledReports finds and executes reports that are due
func (rs *ReportScheduler) checkAndExecuteScheduledReports() {
	var reports []models.ReportDefinition

	// Find reports that are scheduled and due for execution
	now := time.Now()
	if err := rs.db.Where("is_scheduled = ? AND is_active = ? AND deleted_at IS NULL", true, true).
		Where("next_execution_at IS NULL OR next_execution_at <= ?", now).
		Find(&reports).Error; err != nil {
		log.Printf("⚠️  Failed to fetch scheduled reports: %v", err)
		return
	}

	log.Printf("🔍 Found %d scheduled reports to execute", len(reports))

	for _, report := range reports {
		go rs.executeScheduledReport(&report)
	}
}

// executeScheduledReport executes a scheduled report
func (rs *ReportScheduler) executeScheduledReport(report *models.ReportDefinition) {
	log.Printf("📊 Executing scheduled report: %s (%s)", report.Name, report.Code)

	// Parse schedule config
	var scheduleConfig models.ScheduleConfig
	if err := json.Unmarshal(report.ScheduleConfig, &scheduleConfig); err != nil {
		log.Printf("❌ Invalid schedule config for report %s: %v", report.Code, err)
		return
	}

	if !scheduleConfig.Enabled {
		log.Printf("⏸️  Report %s is not enabled for scheduling", report.Code)
		return
	}

	// Execute the report
	result, err := rs.reportEngine.ExecuteReport(report, nil, "system_scheduler")
	if err != nil {
		log.Printf("❌ Failed to execute scheduled report %s: %v", report.Code, err)
		return
	}

	log.Printf("✅ Report %s executed successfully with %d rows", report.Code, result.MetaData.TotalRows)

	// Generate export files
	files := rs.generateExportFiles(report, result)

	// Send to recipients
	if len(report.Recipients) > 0 {
		rs.sendReportToRecipients(report, result, files)
	}

	// Update next execution time
	rs.updateNextExecutionTime(report, &scheduleConfig)
}

// generateExportFiles creates export files in requested formats
func (rs *ReportScheduler) generateExportFiles(report *models.ReportDefinition, result *ReportResult) map[string][]byte {
	files := make(map[string][]byte)

	for _, format := range report.ExportFormats {
		switch format {
		case "excel", "xlsx":
			if excelFile, err := createExcelFile(report.Name, result); err == nil {
				if buffer, err := excelFile.WriteToBuffer(); err == nil {
					files["excel"] = buffer.Bytes()
				}
			}

		case "csv":
			if csvData, err := createCSVFile(result); err == nil {
				files["csv"] = csvData
			}

		case "pdf":
			if pdfData, err := createPDFFile(report.Name, result); err == nil {
				files["pdf"] = pdfData
			}
		}
	}

	return files
}

// sendReportToRecipients sends the report to configured recipients
func (rs *ReportScheduler) sendReportToRecipients(report *models.ReportDefinition, result *ReportResult, files map[string][]byte) {
	// This would integrate with your email service
	// For now, we'll log the action

	log.Printf("📧 Sending report %s to %d recipients", report.Code, len(report.Recipients))

	// Example email content
	// subject := fmt.Sprintf("Scheduled Report: %s", report.Name)
	// body := fmt.Sprintf(`
	// 	<h2>%s</h2>
	// 	<p>This is your scheduled report generated on %s</p>
	// 	<h3>Summary</h3>
	// 	<ul>
	// 		<li>Total Records: %d</li>
	// 		<li>Execution Time: %d ms</li>
	// 	</ul>
	// 	<p>Please find the report attached in the requested formats.</p>
	// `, report.Name, time.Now().Format("2006-01-02 15:04:05"), result.MetaData.TotalRows, result.MetaData.ExecutionTime)

	// TODO: Integrate with email service
	// emailService.SendEmail(report.Recipients, subject, body, files)

	for _, recipient := range report.Recipients {
		log.Printf("  → %s", recipient)
	}

	log.Printf("✅ Report sent successfully")
}

// updateNextExecutionTime calculates and updates the next execution time
func (rs *ReportScheduler) updateNextExecutionTime(report *models.ReportDefinition, scheduleConfig *models.ScheduleConfig) {
	var nextExecution time.Time

	// Load timezone
	loc, err := time.LoadLocation(scheduleConfig.Timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)

	// Parse scheduled time
	scheduledTime, err := time.Parse("15:04", scheduleConfig.Time)
	if err != nil {
		log.Printf("⚠️  Invalid time format for report %s: %v", report.Code, err)
		return
	}

	switch scheduleConfig.Frequency {
	case "daily":
		// Next execution is tomorrow at the scheduled time
		nextExecution = time.Date(now.Year(), now.Month(), now.Day(),
			scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, loc)

		if nextExecution.Before(now) {
			nextExecution = nextExecution.Add(24 * time.Hour)
		}

	case "weekly":
		// Next execution is next week on the specified day
		daysUntilTarget := (scheduleConfig.DayOfWeek - int(now.Weekday()) + 7) % 7
		if daysUntilTarget == 0 {
			daysUntilTarget = 7
		}

		nextExecution = time.Date(now.Year(), now.Month(), now.Day(),
			scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, loc)
		nextExecution = nextExecution.Add(time.Duration(daysUntilTarget) * 24 * time.Hour)

	case "monthly":
		// Next execution is next month on the specified day
		year := now.Year()
		month := now.Month() + 1
		if month > 12 {
			month = 1
			year++
		}

		day := scheduleConfig.DayOfMonth
		// Handle edge case where day doesn't exist in month (e.g., Feb 31)
		lastDayOfMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
		if day > lastDayOfMonth {
			day = lastDayOfMonth
		}

		nextExecution = time.Date(year, month, day,
			scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, loc)

	default:
		log.Printf("⚠️  Unknown frequency for report %s: %s", report.Code, scheduleConfig.Frequency)
		return
	}

	// Update the report's next execution time
	lastExec := time.Now()
	if err := rs.db.Model(report).Updates(map[string]interface{}{
		"last_executed_at":  &lastExec,
		"next_execution_at": &nextExecution,
	}).Error; err != nil {
		log.Printf("⚠️  Failed to update execution times for report %s: %v", report.Code, err)
	} else {
		log.Printf("📅 Next execution for %s scheduled at: %s", report.Code, nextExecution.Format("2006-01-02 15:04:05"))
	}
}

// ScheduleReport configures a report for scheduled execution
func (rs *ReportScheduler) ScheduleReport(
	reportID uuid.UUID,
	frequency string,
	scheduleTime string,
	dayOfWeek int,
	dayOfMonth int,
	timezone string,
	recipients []string,
	exportFormats []string,
) error {
	var report models.ReportDefinition
	if err := rs.db.Where("id = ?", reportID).First(&report).Error; err != nil {
		return fmt.Errorf("report not found")
	}

	scheduleConfig := models.ScheduleConfig{
		Frequency:  frequency,
		Time:       scheduleTime,
		DayOfWeek:  dayOfWeek,
		DayOfMonth: dayOfMonth,
		Timezone:   timezone,
		Enabled:    true,
	}

	scheduleJSON, _ := json.Marshal(scheduleConfig)

	// Calculate initial next execution time
	var nextExecution time.Time
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	scheduledTime, _ := time.Parse("15:04", scheduleTime)

	switch frequency {
	case "daily":
		nextExecution = time.Date(now.Year(), now.Month(), now.Day(),
			scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, loc)
		if nextExecution.Before(now) {
			nextExecution = nextExecution.Add(24 * time.Hour)
		}

	case "weekly":
		daysUntilTarget := (dayOfWeek - int(now.Weekday()) + 7) % 7
		if daysUntilTarget == 0 && now.Hour() >= scheduledTime.Hour() {
			daysUntilTarget = 7
		}
		nextExecution = time.Date(now.Year(), now.Month(), now.Day(),
			scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, loc)
		nextExecution = nextExecution.Add(time.Duration(daysUntilTarget) * 24 * time.Hour)

	case "monthly":
		year := now.Year()
		month := now.Month()
		if now.Day() >= dayOfMonth {
			month++
			if month > 12 {
				month = 1
				year++
			}
		}
		nextExecution = time.Date(year, month, dayOfMonth,
			scheduledTime.Hour(), scheduledTime.Minute(), 0, 0, loc)
	}

	// Update report
	updates := map[string]interface{}{
		"is_scheduled":      true,
		"schedule_config":   scheduleJSON,
		"recipients":        recipients,
		"export_formats":    exportFormats,
		"next_execution_at": &nextExecution,
	}

	if err := rs.db.Model(&report).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to schedule report: %v", err)
	}

	log.Printf("✅ Report %s scheduled successfully. Next execution: %s", report.Code, nextExecution.Format("2006-01-02 15:04:05"))
	return nil
}

// UnscheduleReport disables scheduling for a report
func (rs *ReportScheduler) UnscheduleReport(reportID uuid.UUID) error {
	updates := map[string]interface{}{
		"is_scheduled":      false,
		"next_execution_at": nil,
	}

	if err := rs.db.Model(&models.ReportDefinition{}).Where("id = ?", reportID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to unschedule report: %v", err)
	}

	log.Printf("✅ Report unscheduled successfully")
	return nil
}

// GetScheduledReports retrieves all scheduled reports
func (rs *ReportScheduler) GetScheduledReports() ([]models.ReportDefinition, error) {
	var reports []models.ReportDefinition

	if err := rs.db.Where("is_scheduled = ? AND is_active = ? AND deleted_at IS NULL", true, true).
		Order("next_execution_at ASC").
		Find(&reports).Error; err != nil {
		return nil, err
	}

	return reports, nil
}

// ExecuteReportNow executes a report immediately (on-demand)
func (rs *ReportScheduler) ExecuteReportNow(reportID uuid.UUID, userID string) error {
	var report models.ReportDefinition
	if err := rs.db.Where("id = ?", reportID).First(&report).Error; err != nil {
		return fmt.Errorf("report not found")
	}

	// Execute the report
	result, err := rs.reportEngine.ExecuteReport(&report, nil, userID)
	if err != nil {
		return fmt.Errorf("failed to execute report: %v", err)
	}

	log.Printf("✅ Report %s executed on-demand with %d rows", report.Code, result.MetaData.TotalRows)

	// Generate export files
	files := rs.generateExportFiles(&report, result)

	// Send to recipients if configured
	if len(report.Recipients) > 0 {
		rs.sendReportToRecipients(&report, result, files)
	}

	return nil
}

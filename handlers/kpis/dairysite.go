package kpi_handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/models/kpis"
)

func GetDairyKPIs(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var sites []models.DairySite
	if err := db.Find(&sites).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Reports submitted per site/day/engineer
	reportsPerSite := make(map[string]int)
	reportsPerDay := make(map[string]int)
	reportsPerEngineer := make(map[string]int)
	siteSet := make(map[string]bool)
	engineerSet := make(map[string]bool)
	workLogs := make([]kpis.WorkLogEntry, 0, len(sites))
	geoPoints := make([]kpis.GeoPoint, 0, len(sites))

	for _, s := range sites {
		reportsPerSite[s.NameOfSite]++
		day := time.Time(s.SubmittedAt).Format("2006-01-02")
		reportsPerDay[day]++
		reportsPerEngineer[s.SiteEngineerName]++
		siteSet[s.NameOfSite] = true
		engineerSet[s.SiteEngineerName] = true
		geoPoints = append(geoPoints, kpis.GeoPoint{
			Latitude:  s.Latitude,
			Longitude: s.Longitude,
		})
	}

	totalReports := len(sites)
	uniqueSites := len(siteSet)
	uniqueEngineers := len(engineerSet)

	// Calculate Reporting Compliance %
	// (Suppose daysExpected is a query param or calculated as unique days between min/max submittedAt)
	daysReported := len(reportsPerDay)
	var daysExpected int
	if len(sites) > 0 {
		minDate := time.Time(sites[0].SubmittedAt)
		maxDate := time.Time(sites[0].SubmittedAt)
		for _, s := range sites {
			t := time.Time(s.SubmittedAt)
			if t.Before(minDate) {
				minDate = t
			}
			if t.After(maxDate) {
				maxDate = t
			}
		}
		daysExpected = int(maxDate.Sub(minDate).Hours()/24) + 1
	} else {
		daysExpected = 1
	}
	compliancePct := 0.0
	if daysExpected > 0 {
		compliancePct = float64(daysReported) / float64(daysExpected) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(kpis.DairyKPI{
		TotalReports:           totalReports,
		ReportsPerSite:         reportsPerSite,
		ReportsPerDay:          reportsPerDay,
		ReportsPerEngineer:     reportsPerEngineer,
		UniqueSites:            uniqueSites,
		UniqueEngineers:        uniqueEngineers,
		WorkLogs:               workLogs,
		GeoPoints:              geoPoints,
		ReportingCompliancePct: compliancePct,
	})
}

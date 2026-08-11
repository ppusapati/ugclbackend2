package kpi_handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/helper"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/models/kpis"
)

func GetContractorKPIs(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var contractors []models.Contractor

	// Optional: add date filtering via query params if needed

	if err := db.Find(&contractors).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var (
		totalMeters, totalDiesel, totalWorkingHours float64
		totalReports, reportsWithPhotos             int
		dateSet                                     = map[string]struct{}{}
		firstDate, lastDate                         time.Time
		vehicleMap                                  = map[string]float64{}
		cardMap                                     = map[string]float64{}
		geoLocations                                [][2]float64
		reportsByDateSite                           = map[string]float64{}
	)

	for i, con := range contractors {
		meters := helper.ToFloat(con.ActualMeters)
		diesel := helper.ToFloat(con.DieselTaken)
		workingHours := helper.ToFloat(con.WoringHours)
		date := time.Time(con.SubmittedAt)

		// Total meters, diesel, working hours
		totalMeters += meters
		totalDiesel += diesel
		totalWorkingHours += workingHours

		// Reports with required photos
		if len(con.MeterPhotos) > 0 || len(con.AreaPhotos) > 0 {
			reportsWithPhotos++
		}

		totalReports++

		// Vehicle utilization (count by type)
		vehicleMap[con.VehicleType]++

		// CardNumber diesel
		cardMap[con.CardNumber] += diesel

		// Geo heatmap
		geoLocations = append(geoLocations, [2]float64{con.Latitude, con.Longitude})

		// Reports by date/site
		dateKey := fmt.Sprintf("%s|%s", date.Format("2006-01-02"), con.SiteName)
		reportsByDateSite[dateKey]++

		// For average meters/day
		dateSet[date.Format("2006-01-02")] = struct{}{}
		if i == 0 || date.Before(firstDate) {
			firstDate = date
		}
		if i == 0 || date.After(lastDate) {
			lastDate = date
		}
	}

	// Average meters per day
	days := float64(len(dateSet))
	averageMetersPerDay := 0.0
	if days > 0 {
		averageMetersPerDay = totalMeters / days
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(kpis.ContractorKPIs{
		TotalMetersCompleted:  totalMeters,
		TotalDieselTaken:      totalDiesel,
		DieselPerMeter:        helper.SafeDiv(totalDiesel, totalMeters),
		AverageMetersPerDay:   averageMetersPerDay,
		ReportsWithPhotosPct:  helper.SafeDiv(float64(reportsWithPhotos*100), float64(totalReports)),
		VehicleUtilization:    helper.MapToKeyValue(vehicleMap),
		AverageWorkingHours:   helper.SafeDiv(totalWorkingHours, float64(totalReports)),
		CardNumberDieselDrawn: helper.MapToKeyValue(cardMap),
		GeoLocations:          geoLocations,
		ReportsByDateSite:     helper.MapToKeyValue(reportsByDateSite),
	})

}

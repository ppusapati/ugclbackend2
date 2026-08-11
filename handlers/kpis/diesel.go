package kpi_handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/helper"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/models/kpis"
)

func GetDieselKPIs(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var diesels []models.Diesel
	if err := db.Find(&diesels).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var (
		totalLiters, totalAmount float64
		withPhotos, withRemarks  int
		byContractor, byVehicle  = map[string]float64{}, map[string]float64{}
		cardDiesel, cardAmount   = map[string]float64{}, map[string]float64{}
		geoPoints                [][]float64
		perSite, perDate         = map[string]int{}, map[string]int{}
	)

	for _, d := range diesels {
		liters := helper.ToFloat(d.QuantityInLiters)
		amount := helper.ToFloat(d.AmountPaid)

		totalLiters += liters
		totalAmount += amount

		// Groupings
		byContractor[d.ContractorName] += liters
		byVehicle[d.VehicleNumber] += liters
		cardDiesel[d.CardNumber] += liters
		cardAmount[d.CardNumber] += amount
		perSite[d.NameOfSite]++
		dateStr := time.Time(d.SubmittedAt).Format("2006-01-02")
		perDate[dateStr]++

		// Compliance & Exceptions
		if len(d.MeterReadingPhotos) > 0 || len(d.BillPhotos) > 0 {
			withPhotos++
		}
		if d.Remarks != nil && *d.Remarks != "" {
			withRemarks++
		}
		geoPoints = append(geoPoints, []float64{d.Latitude, d.Longitude})
	}

	// Convert map to sorted slice
	kvpSlice := func(m map[string]float64) []kpis.KVP {
		res := make([]kpis.KVP, 0, len(m))
		for k, v := range m {
			res = append(res, kpis.KVP{Key: k, Value: v})
		}
		sort.Slice(res, func(i, j int) bool { return res[i].Value > res[j].Value })
		return res
	}
	cardUsage := make([]kpis.KVP, 0, len(cardDiesel))
	for card, liters := range cardDiesel {
		cardUsage = append(cardUsage, kpis.KVP{
			Key:   card,
			Value: liters,
			Extra: cardAmount[card],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(kpis.DieselKPIs{
		TotalDieselConsumed:     totalLiters,
		TotalAmountPaid:         totalAmount,
		AvgDieselPerLiter:       helper.IfZero(totalAmount/totalLiters, totalLiters),
		DieselByContractor:      kvpSlice(byContractor),
		DieselByVehicle:         kvpSlice(byVehicle),
		CardNumberUsage:         cardUsage,
		EntriesWithPhotosPct:    helper.Percent(withPhotos, len(diesels)),
		GeoPoints:               geoPoints,
		EntriesWithRemarks:      withRemarks,
		EntriesSubmittedPerSite: helper.KvpCount(perSite),
		EntriesSubmittedPerDate: helper.KvpCount(perDate),
		TotalEntries:            len(diesels),
	})
}

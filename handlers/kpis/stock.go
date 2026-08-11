// handlers/stock.go
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

func GetStockKPIs(w http.ResponseWriter, r *http.Request) {
	db, cleanup, err := config.DBFromContext(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	defer cleanup()

	var stocks []models.Stock
	if err := db.Find(&stocks).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var (
		totalIn, totalOut, specials, withChallan, defective int
		delaySum, agingSum                                  float64
		contractorMap                                       = map[string]int{}
		itemPipeMap                                         = map[string]int{}
	)

	for _, s := range stocks {
		quantity := helper.ToInt(s.ItemQuantity)
		length := helper.ToInt(s.TotalLength)
		isIn := s.InOut == "IN"
		isOut := s.InOut == "OUT"
		isSpecial := s.SpecialItemDescription != ""
		hasDefective := s.DefectiveMaterial != nil && *s.DefectiveMaterial != ""

		// Totals
		if isIn {
			totalIn += quantity + length
		}
		if isOut {
			totalOut += quantity + length
		}
		// Specials
		if isSpecial {
			specials++
		}
		// Documentation compliance
		if len(helper.AsStringArray(s.ChallanFiles)) > 0 {
			withChallan++
		}
		// Defective
		if hasDefective {
			defective++
		}
		// Delay/aging
		invoiceDate := time.Time(s.InvoiceDate)
		submittedAt := time.Time(s.SubmittedAt)
		delay := submittedAt.Sub(invoiceDate).Hours() / 24
		delaySum += delay
		agingSum += delay
		// Top Contractors
		contractorMap[s.ContractorName] += quantity + length
		// Top Items/PipeDia
		itemKey := s.ItemDescription + " | " + s.PipeDia
		itemPipeMap[itemKey] += quantity + length
	}

	total := len(stocks)
	regulars := total - specials

	// Build top N
	topN := func(m map[string]int, n int) []kpis.KVP {
		type kv struct {
			K string
			V int
		}
		list := make([]kv, 0, len(m))
		for k, v := range m {
			list = append(list, kv{k, v})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].V > list[j].V })
		res := make([]kpis.KVP, 0, n)
		for i := 0; i < len(list) && i < n; i++ {
			res = append(res, kpis.KVP{Key: list[i].K, Value: float64(list[i].V)})
		}
		return res
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(kpis.StockKPIs{
		TotalStockIn:               totalIn,
		TotalStockOut:              totalOut,
		CurrentStockLevel:          totalIn - totalOut,
		StockAgingDays:             helper.Round(agingSum/float64(total), 2),
		DefectiveMaterialPct:       helper.Round((float64(defective)/float64(total))*100, 2),
		TopContractors:             topN(contractorMap, 3),
		TopItemsPipeDiaUsed:        topN(itemPipeMap, 3),
		DocumentationCompliancePct: helper.Round((float64(withChallan)/float64(total))*100, 2),
		SpecialsVsRegularRatio:     helper.IfZeroFloat(float64(specials)/float64(regulars), regulars),
		AvgDataEntryDelayDays:      helper.Round(delaySum/float64(total), 2),
	})
}

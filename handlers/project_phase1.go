package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// ProjectPhase1Handler exposes execution-planning and billing controls.
type ProjectPhase1Handler struct {
	db *gorm.DB
}

func NewProjectPhase1Handler() *ProjectPhase1Handler {
	return &ProjectPhase1Handler{db: config.DB}
}

func (h *ProjectPhase1Handler) CreateWBSNode(w http.ResponseWriter, r *http.Request) {
	project, claims, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	var req struct {
		ParentID         *uuid.UUID `json:"parent_id"`
		Code             string     `json:"code"`
		Name             string     `json:"name"`
		Description      string     `json:"description"`
		NodeType         string     `json:"node_type"`
		SortOrder        int        `json:"sort_order"`
		PlannedStartDate *time.Time `json:"planned_start_date"`
		PlannedEndDate   *time.Time `json:"planned_end_date"`
		Weightage        float64    `json:"weightage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		http.Error(w, "code and name are required", http.StatusBadRequest)
		return
	}

	nodeType := strings.ToLower(strings.TrimSpace(req.NodeType))
	if nodeType == "" {
		nodeType = "activity"
	}
	if nodeType != "package" && nodeType != "activity" && nodeType != "milestone" {
		http.Error(w, "node_type must be package, activity, or milestone", http.StatusBadRequest)
		return
	}

	node := models.WBSNode{
		ProjectID:        project.ID,
		ParentID:         req.ParentID,
		Code:             req.Code,
		Name:             req.Name,
		Description:      req.Description,
		NodeType:         nodeType,
		SortOrder:        req.SortOrder,
		PlannedStartDate: req.PlannedStartDate,
		PlannedEndDate:   req.PlannedEndDate,
		Weightage:        req.Weightage,
		CreatedBy:        claims.UserID,
	}

	if err := h.db.Create(&node).Error; err != nil {
		http.Error(w, "failed to create WBS node", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"wbs_node": node})
}

func (h *ProjectPhase1Handler) ListWBSNodes(w http.ResponseWriter, r *http.Request) {
	project, _, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	query := h.db.Where("project_id = ?", project.ID).Order("sort_order ASC, code ASC")
	if parentID := r.URL.Query().Get("parent_id"); parentID != "" {
		query = query.Where("parent_id = ?", parentID)
	}
	if nodeType := r.URL.Query().Get("node_type"); nodeType != "" {
		query = query.Where("node_type = ?", strings.ToLower(nodeType))
	}

	var nodes []models.WBSNode
	if err := query.Find(&nodes).Error; err != nil {
		http.Error(w, "failed to list WBS nodes", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"wbs_nodes": nodes, "count": len(nodes)})
}

func (h *ProjectPhase1Handler) CreateTaskDependency(w http.ResponseWriter, r *http.Request) {
	project, claims, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	var req struct {
		PredecessorTaskID uuid.UUID `json:"predecessor_task_id"`
		SuccessorTaskID   uuid.UUID `json:"successor_task_id"`
		DependencyType    string    `json:"dependency_type"`
		LagDays           int       `json:"lag_days"`
		Notes             string    `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.PredecessorTaskID == uuid.Nil || req.SuccessorTaskID == uuid.Nil || req.PredecessorTaskID == req.SuccessorTaskID {
		http.Error(w, "valid predecessor_task_id and successor_task_id are required", http.StatusBadRequest)
		return
	}

	depType := strings.ToUpper(strings.TrimSpace(req.DependencyType))
	if depType == "" {
		depType = "FS"
	}
	if depType != "FS" && depType != "SS" && depType != "FF" && depType != "SF" {
		http.Error(w, "dependency_type must be FS, SS, FF, or SF", http.StatusBadRequest)
		return
	}

	var count int64
	if err := h.db.Model(&models.Tasks{}).Where("project_id = ? AND id IN ?", project.ID, []uuid.UUID{req.PredecessorTaskID, req.SuccessorTaskID}).Count(&count).Error; err != nil || count != 2 {
		http.Error(w, "tasks must belong to the same project", http.StatusBadRequest)
		return
	}

	dep := models.TaskDependency{
		ProjectID:         project.ID,
		PredecessorTaskID: req.PredecessorTaskID,
		SuccessorTaskID:   req.SuccessorTaskID,
		DependencyType:    depType,
		LagDays:           req.LagDays,
		Notes:             req.Notes,
		IsActive:          true,
		CreatedBy:         claims.UserID,
	}

	if err := h.db.Create(&dep).Error; err != nil {
		http.Error(w, "failed to create task dependency", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"task_dependency": dep})
}

func (h *ProjectPhase1Handler) ListTaskDependencies(w http.ResponseWriter, r *http.Request) {
	project, _, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	query := h.db.Where("project_id = ?", project.ID).Order("created_at DESC")
	if taskID := r.URL.Query().Get("task_id"); taskID != "" {
		query = query.Where("predecessor_task_id = ? OR successor_task_id = ?", taskID, taskID)
	}

	var deps []models.TaskDependency
	if err := query.Find(&deps).Error; err != nil {
		http.Error(w, "failed to list dependencies", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"task_dependencies": deps, "count": len(deps)})
}

func (h *ProjectPhase1Handler) CreateBOQItem(w http.ResponseWriter, r *http.Request) {
	project, claims, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	var req struct {
		WBSNodeID       *uuid.UUID `json:"wbs_node_id"`
		Code            string     `json:"code"`
		Description     string     `json:"description"`
		UOM             string     `json:"uom"`
		PlannedQuantity float64    `json:"planned_quantity"`
		UnitRate        float64    `json:"unit_rate"`
		PlannedAmount   float64    `json:"planned_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	req.Description = strings.TrimSpace(req.Description)
	req.UOM = strings.TrimSpace(req.UOM)
	if req.Code == "" || req.Description == "" || req.UOM == "" {
		http.Error(w, "code, description, and uom are required", http.StatusBadRequest)
		return
	}

	plannedAmount := req.PlannedAmount
	if plannedAmount == 0 {
		plannedAmount = req.PlannedQuantity * req.UnitRate
	}

	item := models.BOQItem{
		ProjectID:       project.ID,
		WBSNodeID:       req.WBSNodeID,
		Code:            req.Code,
		Description:     req.Description,
		UOM:             req.UOM,
		PlannedQuantity: req.PlannedQuantity,
		UnitRate:        req.UnitRate,
		PlannedAmount:   plannedAmount,
		Status:          "planned",
		CreatedBy:       claims.UserID,
	}

	if err := h.db.Create(&item).Error; err != nil {
		http.Error(w, "failed to create BOQ item", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"boq_item": item})
}

func (h *ProjectPhase1Handler) ListBOQItems(w http.ResponseWriter, r *http.Request) {
	project, _, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	query := h.db.Where("project_id = ?", project.ID).Order("code ASC")
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", strings.ToLower(status))
	}

	var items []models.BOQItem
	if err := query.Find(&items).Error; err != nil {
		http.Error(w, "failed to list BOQ items", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"boq_items": items, "count": len(items)})
}

func (h *ProjectPhase1Handler) CreateMBEntry(w http.ResponseWriter, r *http.Request) {
	project, claims, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	var req struct {
		BOQItemID       uuid.UUID  `json:"boq_item_id"`
		EntryNumber     string     `json:"entry_number"`
		MeasurementDate *time.Time `json:"measurement_date"`
		MeasuredQty     float64    `json:"measured_qty"`
		Rate            float64    `json:"rate"`
		Amount          float64    `json:"amount"`
		LocationRef     string     `json:"location_ref"`
		Remarks         string     `json:"remarks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.BOQItemID == uuid.Nil || strings.TrimSpace(req.EntryNumber) == "" || req.MeasuredQty <= 0 {
		http.Error(w, "boq_item_id, entry_number and positive measured_qty are required", http.StatusBadRequest)
		return
	}

	var boq models.BOQItem
	if err := h.db.First(&boq, "id = ? AND project_id = ?", req.BOQItemID, project.ID).Error; err != nil {
		http.Error(w, "BOQ item not found", http.StatusBadRequest)
		return
	}

	measureDate := time.Now().UTC()
	if req.MeasurementDate != nil {
		measureDate = req.MeasurementDate.UTC()
	}

	rate := req.Rate
	if rate == 0 {
		rate = boq.UnitRate
	}
	amount := req.Amount
	if amount == 0 {
		amount = req.MeasuredQty * rate
	}

	entry := models.MBEntry{
		ProjectID:       project.ID,
		BOQItemID:       boq.ID,
		EntryNumber:     strings.TrimSpace(req.EntryNumber),
		MeasurementDate: measureDate,
		MeasuredQty:     req.MeasuredQty,
		Rate:            rate,
		Amount:          amount,
		LocationRef:     strings.TrimSpace(req.LocationRef),
		Remarks:         req.Remarks,
		RecordedBy:      claims.UserID,
	}

	tx := h.db.Begin()
	if err := tx.Create(&entry).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create MB entry", http.StatusInternalServerError)
		return
	}

	if err := tx.Model(&models.BOQItem{}).
		Where("id = ?", boq.ID).
		Updates(map[string]interface{}{
			"executed_quantity": gorm.Expr("executed_quantity + ?", req.MeasuredQty),
			"updated_by":        claims.UserID,
		}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update BOQ quantity", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit MB entry", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"mb_entry": entry})
}

func (h *ProjectPhase1Handler) ListMBEntries(w http.ResponseWriter, r *http.Request) {
	project, _, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	query := h.db.Where("project_id = ?", project.ID).Order("measurement_date DESC")
	if boqItemID := r.URL.Query().Get("boq_item_id"); boqItemID != "" {
		query = query.Where("boq_item_id = ?", boqItemID)
	}

	var entries []models.MBEntry
	if err := query.Find(&entries).Error; err != nil {
		http.Error(w, "failed to list MB entries", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"mb_entries": entries, "count": len(entries)})
}

func (h *ProjectPhase1Handler) CreateRABill(w http.ResponseWriter, r *http.Request) {
	project, claims, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	var req struct {
		BillNumber       string     `json:"bill_number"`
		PeriodStart      *time.Time `json:"period_start"`
		PeriodEnd        *time.Time `json:"period_end"`
		GrossAmount      float64    `json:"gross_amount"`
		DeductionsAmount float64    `json:"deductions_amount"`
		RetentionAmount  float64    `json:"retention_amount"`
		TaxAmount        float64    `json:"tax_amount"`
		NetAmount        float64    `json:"net_amount"`
		Notes            string     `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	req.BillNumber = strings.TrimSpace(req.BillNumber)
	if req.BillNumber == "" {
		http.Error(w, "bill_number is required", http.StatusBadRequest)
		return
	}

	netAmount := req.NetAmount
	if netAmount == 0 {
		netAmount = req.GrossAmount - req.DeductionsAmount - req.RetentionAmount + req.TaxAmount
	}

	bill := models.RABill{
		ProjectID:        project.ID,
		BillNumber:       req.BillNumber,
		PeriodStart:      req.PeriodStart,
		PeriodEnd:        req.PeriodEnd,
		GrossAmount:      req.GrossAmount,
		DeductionsAmount: req.DeductionsAmount,
		RetentionAmount:  req.RetentionAmount,
		TaxAmount:        req.TaxAmount,
		NetAmount:        netAmount,
		Status:           "draft",
		Notes:            req.Notes,
		CreatedBy:        claims.UserID,
	}

	if err := h.db.Create(&bill).Error; err != nil {
		http.Error(w, "failed to create RA bill", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"ra_bill": bill})
}

func (h *ProjectPhase1Handler) ListRABills(w http.ResponseWriter, r *http.Request) {
	project, _, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	query := h.db.Where("project_id = ?", project.ID).Order("created_at DESC")
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", strings.ToLower(status))
	}

	var bills []models.RABill
	if err := query.Find(&bills).Error; err != nil {
		http.Error(w, "failed to list RA bills", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"ra_bills": bills, "count": len(bills)})
}

func (h *ProjectPhase1Handler) GetRABill(w http.ResponseWriter, r *http.Request) {
	project, _, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	billID, err := uuid.Parse(mux.Vars(r)["billId"])
	if err != nil {
		http.Error(w, "invalid billId", http.StatusBadRequest)
		return
	}

	var bill models.RABill
	if err := h.db.Preload("Lines").First(&bill, "id = ? AND project_id = ?", billID, project.ID).Error; err != nil {
		http.Error(w, "RA bill not found", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"ra_bill": bill})
}

func (h *ProjectPhase1Handler) AddRABillLine(w http.ResponseWriter, r *http.Request) {
	project, claims, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	vars := mux.Vars(r)
	billID, err := uuid.Parse(vars["billId"])
	if err != nil {
		http.Error(w, "invalid billId", http.StatusBadRequest)
		return
	}

	var req struct {
		BOQItemID uuid.UUID  `json:"boq_item_id"`
		MBEntryID *uuid.UUID `json:"mb_entry_id"`
		Quantity  float64    `json:"quantity"`
		Rate      float64    `json:"rate"`
		Amount    float64    `json:"amount"`
		Remark    string     `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.BOQItemID == uuid.Nil || req.Quantity <= 0 {
		http.Error(w, "boq_item_id and positive quantity are required", http.StatusBadRequest)
		return
	}

	var bill models.RABill
	if err := h.db.First(&bill, "id = ? AND project_id = ?", billID, project.ID).Error; err != nil {
		http.Error(w, "RA bill not found", http.StatusNotFound)
		return
	}
	if bill.Status == "approved" || bill.Status == "paid" {
		http.Error(w, "cannot modify an approved or paid bill", http.StatusConflict)
		return
	}

	var boq models.BOQItem
	if err := h.db.First(&boq, "id = ? AND project_id = ?", req.BOQItemID, project.ID).Error; err != nil {
		http.Error(w, "BOQ item not found", http.StatusBadRequest)
		return
	}

	rate := req.Rate
	if rate == 0 {
		rate = boq.UnitRate
	}
	amount := req.Amount
	if amount == 0 {
		amount = req.Quantity * rate
	}

	line := models.RABillLine{
		RABillID:   bill.ID,
		BOQItemID:  boq.ID,
		MBEntryID:  req.MBEntryID,
		Quantity:   req.Quantity,
		Rate:       rate,
		Amount:     amount,
		LineRemark: req.Remark,
	}

	tx := h.db.Begin()
	if err := tx.Create(&line).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to add RA bill line", http.StatusInternalServerError)
		return
	}

	if err := tx.Model(&models.BOQItem{}).
		Where("id = ?", boq.ID).
		Updates(map[string]interface{}{
			"billed_quantity": gorm.Expr("billed_quantity + ?", req.Quantity),
			"updated_by":      claims.UserID,
		}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update BOQ billed quantity", http.StatusInternalServerError)
		return
	}

	var gross float64
	if err := tx.Model(&models.RABillLine{}).Where("ra_bill_id = ?", bill.ID).Select("COALESCE(SUM(amount),0)").Scan(&gross).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to aggregate RA bill lines", http.StatusInternalServerError)
		return
	}

	net := gross - bill.DeductionsAmount - bill.RetentionAmount + bill.TaxAmount
	if err := tx.Model(&models.RABill{}).Where("id = ?", bill.ID).Updates(map[string]interface{}{
		"gross_amount": gross,
		"net_amount":   net,
		"updated_by":   claims.UserID,
	}).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update RA bill totals", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to commit RA bill line", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"ra_bill_line": line})
}

func (h *ProjectPhase1Handler) SubmitRABill(w http.ResponseWriter, r *http.Request) {
	h.transitionRABillStatus(w, r, "submitted")
}

func (h *ProjectPhase1Handler) ApproveRABill(w http.ResponseWriter, r *http.Request) {
	h.transitionRABillStatus(w, r, "approved")
}

func (h *ProjectPhase1Handler) RejectRABill(w http.ResponseWriter, r *http.Request) {
	h.transitionRABillStatus(w, r, "rejected")
}

func (h *ProjectPhase1Handler) MarkRABillPaid(w http.ResponseWriter, r *http.Request) {
	h.transitionRABillStatus(w, r, "paid")
}

func (h *ProjectPhase1Handler) transitionRABillStatus(w http.ResponseWriter, r *http.Request, nextStatus string) {
	project, claims, err := h.requireProjectScope(r)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	billID, err := uuid.Parse(mux.Vars(r)["billId"])
	if err != nil {
		http.Error(w, "invalid billId", http.StatusBadRequest)
		return
	}

	var bill models.RABill
	if err := h.db.First(&bill, "id = ? AND project_id = ?", billID, project.ID).Error; err != nil {
		http.Error(w, "RA bill not found", http.StatusNotFound)
		return
	}

	if !isValidBillTransition(bill.Status, nextStatus) {
		http.Error(w, fmt.Sprintf("invalid status transition %s -> %s", bill.Status, nextStatus), http.StatusConflict)
		return
	}

	updates := map[string]interface{}{
		"status":     nextStatus,
		"updated_by": claims.UserID,
	}
	now := time.Now().UTC()
	if nextStatus == "submitted" {
		updates["submitted_by"] = claims.UserID
		updates["submitted_at"] = now
	}
	if nextStatus == "approved" {
		updates["approved_by"] = claims.UserID
		updates["approved_at"] = now
	}
	if nextStatus == "paid" {
		updates["payment_reference"] = strings.TrimSpace(r.URL.Query().Get("payment_reference"))
	}

	if err := h.db.Model(&bill).Updates(updates).Error; err != nil {
		http.Error(w, "failed to update RA bill status", http.StatusInternalServerError)
		return
	}

	if err := h.db.First(&bill, "id = ?", bill.ID).Error; err != nil {
		http.Error(w, "failed to load RA bill", http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]interface{}{"ra_bill": bill})
}

func isValidBillTransition(current, next string) bool {
	allowed := map[string]map[string]bool{
		"draft":     {"submitted": true},
		"submitted": {"approved": true, "rejected": true},
		"rejected":  {"submitted": true},
		"approved":  {"paid": true},
	}

	nextMap, ok := allowed[current]
	if !ok {
		return false
	}
	return nextMap[next]
}

func (h *ProjectPhase1Handler) requireProjectScope(r *http.Request) (*models.Project, *middleware.Claims, error) {
	projectID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		return nil, nil, apiError{status: http.StatusBadRequest, message: "invalid project id"}
	}

	claims := middleware.GetClaims(r)
	if claims == nil {
		return nil, nil, apiError{status: http.StatusUnauthorized, message: "unauthorized"}
	}

	query := h.db.Model(&models.Project{}).Where("id = ?", projectID)
	if businessContext := middleware.GetUserBusinessContext(r); businessContext != nil {
		if businessID, ok := businessContext["business_id"].(uuid.UUID); ok && businessID != uuid.Nil {
			query = query.Where("business_vertical_id = ?", businessID)
		}
	}

	var project models.Project
	if err := query.First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apiError{status: http.StatusNotFound, message: "project not found"}
		}
		return nil, nil, apiError{status: http.StatusInternalServerError, message: "failed to load project"}
	}

	return &project, claims, nil
}

type apiError struct {
	status  int
	message string
}

func (e apiError) Error() string { return e.message }

func (h *ProjectPhase1Handler) writeErr(w http.ResponseWriter, err error) {
	if ae, ok := err.(apiError); ok {
		http.Error(w, ae.message, ae.status)
		return
	}
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (h *ProjectPhase1Handler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

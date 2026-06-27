package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

func parseFinanceUUIDParam(r *http.Request, key string) (uuid.UUID, error) {
	vars := mux.Vars(r)
	return uuid.Parse(vars[key])
}

func createFinanceApprovalRequest(tx *gorm.DB, businessID uuid.UUID, entityType string, entityID uuid.UUID, requestType string, requestedBy string, notes string) (uuid.UUID, error) {
	var existing models.FinanceApprovalRequest
	if err := tx.Where("business_vertical_id = ? AND entity_type = ? AND entity_id = ? AND request_type = ? AND status = ?",
		businessID,
		entityType,
		entityID,
		requestType,
		models.FinanceApprovalPending,
	).First(&existing).Error; err == nil {
		return existing.ID, nil
	}

	req := models.FinanceApprovalRequest{
		BusinessVerticalID: businessID,
		EntityType:         entityType,
		EntityID:           entityID,
		RequestType:        requestType,
		Status:             models.FinanceApprovalPending,
		RequestedBy:        requestedBy,
		Notes:              notes,
		RequiredApprovals:  1,
		ReceivedApprovals:  0,
		Metadata:           datatypes.JSON([]byte(`{}`)),
	}

	if err := tx.Create(&req).Error; err != nil {
		return uuid.Nil, err
	}

	return req.ID, nil
}

func approveFinanceApprovalRequest(tx *gorm.DB, requestID *uuid.UUID, approverID string, comments string) error {
	if requestID == nil || *requestID == uuid.Nil {
		return nil
	}

	var req models.FinanceApprovalRequest
	if err := tx.First(&req, "id = ?", *requestID).Error; err != nil {
		return err
	}

	if req.Status == models.FinanceApprovalApproved {
		return nil
	}

	approval := models.FinanceApproval{
		RequestID:  req.ID,
		ApproverID: approverID,
		Status:     models.FinanceApprovalApproved,
		Comments:   comments,
	}
	if err := tx.Create(&approval).Error; err != nil {
		return err
	}

	now := time.Now()
	return tx.Model(&req).Updates(map[string]interface{}{
		"status":             models.FinanceApprovalApproved,
		"received_approvals": req.RequiredApprovals,
		"resolved_at":        &now,
		"resolved_by":        approverID,
	}).Error
}

// ==========================
// Bank Guarantee handlers
// ==========================

func ListBankGuarantees(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var items []models.BankGuarantee
	query := config.DB.Where("business_vertical_id = ?", businessID)
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		http.Error(w, "failed to fetch bank guarantees", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "count": len(items)})
}

func CreateBankGuarantee(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var item models.BankGuarantee
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if item.GuaranteeNumber == "" || item.Amount <= 0 || item.IssuingBank == "" {
		http.Error(w, "guarantee_number, issuing_bank and amount are required", http.StatusBadRequest)
		return
	}

	item.BusinessVerticalID = businessID
	item.CreatedBy = middleware.GetClaims(r).UserID
	if item.Status == "" {
		item.Status = "draft"
	}
	if item.Currency == "" {
		item.Currency = "INR"
	}

	tx := config.DB.Begin()
	if err := tx.Create(&item).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create bank guarantee", http.StatusInternalServerError)
		return
	}

	approvalID, err := createFinanceApprovalRequest(tx, businessID, "bank_guarantee", item.ID, "bg:create", item.CreatedBy, "Bank guarantee created and awaiting approval")
	if err != nil {
		tx.Rollback()
		http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
		return
	}
	item.ApprovalRequestID = &approvalID
	if err := tx.Model(&item).Update("approval_request_id", approvalID).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to link finance approval request", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize bank guarantee creation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "bank guarantee created", "item": item})
}

func GetBankGuarantee(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.BankGuarantee
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "bank guarantee not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func UpdateBankGuarantee(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.BankGuarantee
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "bank guarantee not found", http.StatusNotFound)
		return
	}

	var req models.BankGuarantee
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = item.ID
	req.BusinessVerticalID = item.BusinessVerticalID
	req.CreatedBy = item.CreatedBy
	req.UpdatedBy = middleware.GetClaims(r).UserID

	if err := config.DB.Model(&item).Updates(req).Error; err != nil {
		http.Error(w, "failed to update bank guarantee", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "bank guarantee updated"})
}

// ==========================
// Letter of Credit handlers
// ==========================

func ListLettersOfCredit(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var items []models.LetterOfCredit
	query := config.DB.Where("business_vertical_id = ?", businessID)
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		http.Error(w, "failed to fetch letters of credit", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "count": len(items)})
}

func CreateLetterOfCredit(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var item models.LetterOfCredit
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if item.LCNumber == "" || item.Amount <= 0 || item.ApplicantBank == "" {
		http.Error(w, "lc_number, applicant_bank and amount are required", http.StatusBadRequest)
		return
	}

	item.BusinessVerticalID = businessID
	item.CreatedBy = middleware.GetClaims(r).UserID
	if item.Status == "" {
		item.Status = "draft"
	}
	if item.Currency == "" {
		item.Currency = "INR"
	}

	tx := config.DB.Begin()
	if err := tx.Create(&item).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create letter of credit", http.StatusInternalServerError)
		return
	}

	approvalID, err := createFinanceApprovalRequest(tx, businessID, "letter_of_credit", item.ID, "lc:create", item.CreatedBy, "Letter of credit created and awaiting approval")
	if err != nil {
		tx.Rollback()
		http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
		return
	}
	item.ApprovalRequestID = &approvalID
	if err := tx.Model(&item).Update("approval_request_id", approvalID).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to link finance approval request", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize letter of credit creation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "letter of credit created", "item": item})
}

func GetLetterOfCredit(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.LetterOfCredit
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "letter of credit not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func UpdateLetterOfCredit(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.LetterOfCredit
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "letter of credit not found", http.StatusNotFound)
		return
	}

	var req models.LetterOfCredit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = item.ID
	req.BusinessVerticalID = item.BusinessVerticalID
	req.CreatedBy = item.CreatedBy
	req.UpdatedBy = middleware.GetClaims(r).UserID

	if err := config.DB.Model(&item).Updates(req).Error; err != nil {
		http.Error(w, "failed to update letter of credit", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "letter of credit updated"})
}

// ==========================
// Insurance policy handlers
// ==========================

func ListInsurancePolicies(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var items []models.InsurancePolicy
	query := config.DB.Where("business_vertical_id = ?", businessID)
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		http.Error(w, "failed to fetch insurance policies", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "count": len(items)})
}

func CreateInsurancePolicy(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var item models.InsurancePolicy
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if item.PolicyNumber == "" || item.ProviderName == "" || item.SumInsured <= 0 {
		http.Error(w, "policy_number, provider_name and sum_insured are required", http.StatusBadRequest)
		return
	}

	item.BusinessVerticalID = businessID
	item.CreatedBy = middleware.GetClaims(r).UserID
	if item.Status == "" {
		item.Status = "draft"
	}
	if item.Currency == "" {
		item.Currency = "INR"
	}

	tx := config.DB.Begin()
	if err := tx.Create(&item).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create insurance policy", http.StatusInternalServerError)
		return
	}

	approvalID, err := createFinanceApprovalRequest(tx, businessID, "insurance_policy", item.ID, "insurance:create", item.CreatedBy, "Insurance policy created and awaiting approval")
	if err != nil {
		tx.Rollback()
		http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
		return
	}
	item.ApprovalRequestID = &approvalID
	if err := tx.Model(&item).Update("approval_request_id", approvalID).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to link finance approval request", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize insurance policy creation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "insurance policy created", "item": item})
}

func GetInsurancePolicy(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.InsurancePolicy
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "insurance policy not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func UpdateInsurancePolicy(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.InsurancePolicy
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "insurance policy not found", http.StatusNotFound)
		return
	}

	var req models.InsurancePolicy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = item.ID
	req.BusinessVerticalID = item.BusinessVerticalID
	req.CreatedBy = item.CreatedBy
	req.UpdatedBy = middleware.GetClaims(r).UserID

	if err := config.DB.Model(&item).Updates(req).Error; err != nil {
		http.Error(w, "failed to update insurance policy", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "insurance policy updated"})
}

// ==========================
// Insurance claim handlers
// ==========================

func ListInsuranceClaims(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var items []models.InsuranceClaim
	query := config.DB.Where("business_vertical_id = ?", businessID)
	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if policyID := r.URL.Query().Get("policy_id"); policyID != "" {
		query = query.Where("policy_id = ?", policyID)
	}

	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		http.Error(w, "failed to fetch insurance claims", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "count": len(items)})
}

func CreateInsuranceClaim(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	var item models.InsuranceClaim
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if item.PolicyID == uuid.Nil || item.ClaimNumber == "" || item.ClaimAmount <= 0 {
		http.Error(w, "policy_id, claim_number and claim_amount are required", http.StatusBadRequest)
		return
	}

	var policy models.InsurancePolicy
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", item.PolicyID, businessID).First(&policy).Error; err != nil {
		http.Error(w, "policy not found in business context", http.StatusBadRequest)
		return
	}

	item.BusinessVerticalID = businessID
	item.CreatedBy = middleware.GetClaims(r).UserID
	if item.Status == "" {
		item.Status = "filed"
	}

	tx := config.DB.Begin()
	if err := tx.Create(&item).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to create insurance claim", http.StatusInternalServerError)
		return
	}

	approvalID, err := createFinanceApprovalRequest(tx, businessID, "insurance_claim", item.ID, "insurance:file_claim", item.CreatedBy, "Insurance claim filed and awaiting approval")
	if err != nil {
		tx.Rollback()
		http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
		return
	}
	item.ApprovalRequestID = &approvalID
	if err := tx.Model(&item).Update("approval_request_id", approvalID).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to link finance approval request", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize insurance claim creation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "insurance claim created", "item": item})
}

func GetInsuranceClaim(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.InsuranceClaim
	if err := config.DB.Preload("Policy").Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "insurance claim not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func UpdateInsuranceClaim(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.InsuranceClaim
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "insurance claim not found", http.StatusNotFound)
		return
	}

	var req models.InsuranceClaim
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = item.ID
	req.BusinessVerticalID = item.BusinessVerticalID
	req.PolicyID = item.PolicyID
	req.CreatedBy = item.CreatedBy
	req.UpdatedBy = middleware.GetClaims(r).UserID

	if err := config.DB.Model(&item).Updates(req).Error; err != nil {
		http.Error(w, "failed to update insurance claim", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "insurance claim updated"})
}

type financeActionRequest struct {
	Remarks        string     `json:"remarks"`
	ExpiryDate     *time.Time `json:"expiry_date,omitempty"`
	RenewalDate    *time.Time `json:"renewal_date,omitempty"`
	ApprovedAmount *float64   `json:"approved_amount,omitempty"`
}

func transitionBankGuaranteeStatus(w http.ResponseWriter, r *http.Request, status string, setClaimDate bool) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.BankGuarantee
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "bank guarantee not found", http.StatusNotFound)
		return
	}

	var req financeActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"updated_by": middleware.GetClaims(r).UserID,
	}
	if req.Remarks != "" {
		updates["remarks"] = req.Remarks
	}
	if req.ExpiryDate != nil {
		updates["expiry_date"] = req.ExpiryDate
	}
	if setClaimDate {
		updates["claim_date"] = &now
	}

	tx := config.DB.Begin()
	approvalID := item.ApprovalRequestID
	if status == "approved" {
		if approvalID == nil || *approvalID == uuid.Nil {
			newID, err := createFinanceApprovalRequest(tx, businessID, "bank_guarantee", item.ID, "bg:approve", middleware.GetClaims(r).UserID, "Bank guarantee approval action")
			if err != nil {
				tx.Rollback()
				http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
				return
			}
			approvalID = &newID
			updates["approval_request_id"] = newID
		}
		if err := approveFinanceApprovalRequest(tx, approvalID, middleware.GetClaims(r).UserID, req.Remarks); err != nil {
			tx.Rollback()
			http.Error(w, "failed to approve finance approval request", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Model(&item).Updates(updates).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update bank guarantee status", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize bank guarantee status update", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "bank guarantee status updated", "status": status})
}

func ApproveBankGuarantee(w http.ResponseWriter, r *http.Request) {
	transitionBankGuaranteeStatus(w, r, "approved", false)
}

func ClaimBankGuarantee(w http.ResponseWriter, r *http.Request) {
	transitionBankGuaranteeStatus(w, r, "claimed", true)
}

func ReleaseBankGuarantee(w http.ResponseWriter, r *http.Request) {
	transitionBankGuaranteeStatus(w, r, "released", false)
}

func RenewBankGuarantee(w http.ResponseWriter, r *http.Request) {
	transitionBankGuaranteeStatus(w, r, "renewed", false)
}

func transitionLetterOfCreditStatus(w http.ResponseWriter, r *http.Request, status string) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.LetterOfCredit
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "letter of credit not found", http.StatusNotFound)
		return
	}

	var req financeActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	updates := map[string]interface{}{
		"status":     status,
		"updated_by": middleware.GetClaims(r).UserID,
	}
	if req.Remarks != "" {
		updates["remarks"] = req.Remarks
	}

	tx := config.DB.Begin()
	approvalID := item.ApprovalRequestID
	if status == "issued" {
		if approvalID == nil || *approvalID == uuid.Nil {
			newID, err := createFinanceApprovalRequest(tx, businessID, "letter_of_credit", item.ID, "lc:issue", middleware.GetClaims(r).UserID, "Letter of credit issue action")
			if err != nil {
				tx.Rollback()
				http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
				return
			}
			approvalID = &newID
			updates["approval_request_id"] = newID
		}
		if err := approveFinanceApprovalRequest(tx, approvalID, middleware.GetClaims(r).UserID, req.Remarks); err != nil {
			tx.Rollback()
			http.Error(w, "failed to approve finance approval request", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Model(&item).Updates(updates).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update letter of credit status", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize letter of credit status update", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "letter of credit status updated", "status": status})
}

func IssueLetterOfCredit(w http.ResponseWriter, r *http.Request) {
	transitionLetterOfCreditStatus(w, r, "issued")
}

func AmendLetterOfCredit(w http.ResponseWriter, r *http.Request) {
	transitionLetterOfCreditStatus(w, r, "amended")
}

func NegotiateLetterOfCredit(w http.ResponseWriter, r *http.Request) {
	transitionLetterOfCreditStatus(w, r, "negotiation")
}

func ClaimLetterOfCredit(w http.ResponseWriter, r *http.Request) {
	transitionLetterOfCreditStatus(w, r, "claimed")
}

func RenewInsurancePolicy(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.InsurancePolicy
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "insurance policy not found", http.StatusNotFound)
		return
	}

	var req financeActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	updates := map[string]interface{}{
		"status":     "renewed",
		"updated_by": middleware.GetClaims(r).UserID,
	}
	if req.Remarks != "" {
		updates["remarks"] = req.Remarks
	}
	if req.RenewalDate != nil {
		updates["renewal_date"] = req.RenewalDate
	}

	tx := config.DB.Begin()
	approvalID := item.ApprovalRequestID
	if approvalID == nil || *approvalID == uuid.Nil {
		newID, err := createFinanceApprovalRequest(tx, businessID, "insurance_policy", item.ID, "insurance:renew", middleware.GetClaims(r).UserID, "Insurance policy renewal action")
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
			return
		}
		approvalID = &newID
		updates["approval_request_id"] = newID
	}
	if err := approveFinanceApprovalRequest(tx, approvalID, middleware.GetClaims(r).UserID, req.Remarks); err != nil {
		tx.Rollback()
		http.Error(w, "failed to approve finance approval request", http.StatusInternalServerError)
		return
	}

	if err := tx.Model(&item).Updates(updates).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to renew insurance policy", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize insurance policy renewal", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "insurance policy renewed"})
}

func transitionInsuranceClaimStatus(w http.ResponseWriter, r *http.Request, status string, settled bool) {
	businessID := middleware.GetCurrentBusinessID(r)
	if businessID == uuid.Nil {
		http.Error(w, "business ID required", http.StatusBadRequest)
		return
	}

	id, err := parseFinanceUUIDParam(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.InsuranceClaim
	if err := config.DB.Where("id = ? AND business_vertical_id = ?", id, businessID).First(&item).Error; err != nil {
		http.Error(w, "insurance claim not found", http.StatusNotFound)
		return
	}

	var req financeActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"updated_by": middleware.GetClaims(r).UserID,
	}
	if req.Remarks != "" {
		updates["remarks"] = req.Remarks
	}
	if req.ApprovedAmount != nil {
		updates["approved_amount"] = *req.ApprovedAmount
	}
	if settled {
		updates["settled_date"] = &now
	}

	tx := config.DB.Begin()
	approvalID := item.ApprovalRequestID
	if status == "approved" || status == "settled" {
		if approvalID == nil || *approvalID == uuid.Nil {
			newID, err := createFinanceApprovalRequest(tx, businessID, "insurance_claim", item.ID, "insurance:approve_claim", middleware.GetClaims(r).UserID, "Insurance claim approval/settlement action")
			if err != nil {
				tx.Rollback()
				http.Error(w, "failed to create finance approval request", http.StatusInternalServerError)
				return
			}
			approvalID = &newID
			updates["approval_request_id"] = newID
		}
		if err := approveFinanceApprovalRequest(tx, approvalID, middleware.GetClaims(r).UserID, req.Remarks); err != nil {
			tx.Rollback()
			http.Error(w, "failed to approve finance approval request", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Model(&item).Updates(updates).Error; err != nil {
		tx.Rollback()
		http.Error(w, "failed to update insurance claim status", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "failed to finalize insurance claim status update", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "insurance claim status updated", "status": status})
}

func ApproveInsuranceClaim(w http.ResponseWriter, r *http.Request) {
	transitionInsuranceClaimStatus(w, r, "approved", false)
}

func SettleInsuranceClaim(w http.ResponseWriter, r *http.Request) {
	transitionInsuranceClaimStatus(w, r, "settled", true)
}

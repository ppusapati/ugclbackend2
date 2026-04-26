package models

import (
	"time"

	"github.com/google/uuid"
)

// WBSNode represents hierarchical project breakdown nodes.
type WBSNode struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"project_id"`
	Project     *Project   `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	ParentID    *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Parent      *WBSNode   `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Code        string     `gorm:"size:64;not null" json:"code"`
	Name        string     `gorm:"size:255;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description,omitempty"`
	NodeType    string     `gorm:"size:32;not null;default:'activity';index" json:"node_type"`
	SortOrder   int        `gorm:"default:0" json:"sort_order"`

	PlannedStartDate *time.Time `json:"planned_start_date,omitempty"`
	PlannedEndDate   *time.Time `json:"planned_end_date,omitempty"`
	ActualStartDate  *time.Time `json:"actual_start_date,omitempty"`
	ActualEndDate    *time.Time `json:"actual_end_date,omitempty"`

	Progress  float64 `gorm:"type:decimal(5,2);default:0" json:"progress"`
	Weightage float64 `gorm:"type:decimal(5,2);default:0" json:"weightage"`

	CreatedBy string     `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy string     `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (WBSNode) TableName() string {
	return "wbs_nodes"
}

// TaskDependency captures precedence relationships between tasks.
type TaskDependency struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID         uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Project           *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	PredecessorTaskID uuid.UUID `gorm:"type:uuid;not null;index" json:"predecessor_task_id"`
	PredecessorTask   *Tasks    `gorm:"foreignKey:PredecessorTaskID" json:"predecessor_task,omitempty"`
	SuccessorTaskID   uuid.UUID `gorm:"type:uuid;not null;index" json:"successor_task_id"`
	SuccessorTask     *Tasks    `gorm:"foreignKey:SuccessorTaskID" json:"successor_task,omitempty"`
	DependencyType    string    `gorm:"size:8;not null;default:'FS'" json:"dependency_type"`
	LagDays           int       `gorm:"default:0" json:"lag_days"`
	Notes             string    `gorm:"type:text" json:"notes,omitempty"`
	IsActive          bool      `gorm:"default:true" json:"is_active"`
	CreatedBy         string    `gorm:"size:255;not null" json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (TaskDependency) TableName() string {
	return "task_dependencies"
}

// BOQItem stores bill-of-quantity planning lines.
type BOQItem struct {
	ID               uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"project_id"`
	Project          *Project   `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	WBSNodeID        *uuid.UUID `gorm:"type:uuid;index" json:"wbs_node_id,omitempty"`
	WBSNode          *WBSNode   `gorm:"foreignKey:WBSNodeID" json:"wbs_node,omitempty"`
	Code             string     `gorm:"size:64;not null;index" json:"code"`
	Description      string     `gorm:"type:text;not null" json:"description"`
	UOM              string     `gorm:"size:32;not null" json:"uom"`
	PlannedQuantity  float64    `gorm:"type:decimal(15,4);default:0" json:"planned_quantity"`
	ExecutedQuantity float64    `gorm:"type:decimal(15,4);default:0" json:"executed_quantity"`
	BilledQuantity   float64    `gorm:"type:decimal(15,4);default:0" json:"billed_quantity"`
	UnitRate         float64    `gorm:"type:decimal(15,2);default:0" json:"unit_rate"`
	PlannedAmount    float64    `gorm:"type:decimal(15,2);default:0" json:"planned_amount"`
	Status           string     `gorm:"size:32;not null;default:'planned';index" json:"status"`
	CreatedBy        string     `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy        string     `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (BOQItem) TableName() string {
	return "boq_items"
}

// MBEntry stores measured quantities used for billing and progress.
type MBEntry struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID       uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Project         *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	BOQItemID       uuid.UUID `gorm:"type:uuid;not null;index" json:"boq_item_id"`
	BOQItem         *BOQItem  `gorm:"foreignKey:BOQItemID" json:"boq_item,omitempty"`
	EntryNumber     string    `gorm:"size:64;not null;index" json:"entry_number"`
	MeasurementDate time.Time `gorm:"not null;index" json:"measurement_date"`
	MeasuredQty     float64   `gorm:"type:decimal(15,4);not null" json:"measured_qty"`
	Rate            float64   `gorm:"type:decimal(15,2);default:0" json:"rate"`
	Amount          float64   `gorm:"type:decimal(15,2);default:0" json:"amount"`
	LocationRef     string    `gorm:"size:255" json:"location_ref,omitempty"`
	Remarks         string    `gorm:"type:text" json:"remarks,omitempty"`
	RecordedBy      string    `gorm:"size:255;not null" json:"recorded_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (MBEntry) TableName() string {
	return "mb_entries"
}

// RABill represents running account bills for project billing cycles.
type RABill struct {
	ID               uuid.UUID    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID        uuid.UUID    `gorm:"type:uuid;not null;index" json:"project_id"`
	Project          *Project     `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	BillNumber       string       `gorm:"size:64;not null;index" json:"bill_number"`
	PeriodStart      *time.Time   `json:"period_start,omitempty"`
	PeriodEnd        *time.Time   `json:"period_end,omitempty"`
	GrossAmount      float64      `gorm:"type:decimal(15,2);default:0" json:"gross_amount"`
	DeductionsAmount float64      `gorm:"type:decimal(15,2);default:0" json:"deductions_amount"`
	RetentionAmount  float64      `gorm:"type:decimal(15,2);default:0" json:"retention_amount"`
	TaxAmount        float64      `gorm:"type:decimal(15,2);default:0" json:"tax_amount"`
	NetAmount        float64      `gorm:"type:decimal(15,2);default:0" json:"net_amount"`
	Status           string       `gorm:"size:32;not null;default:'draft';index" json:"status"`
	SubmittedBy      string       `gorm:"size:255" json:"submitted_by,omitempty"`
	SubmittedAt      *time.Time   `json:"submitted_at,omitempty"`
	ApprovedBy       string       `gorm:"size:255" json:"approved_by,omitempty"`
	ApprovedAt       *time.Time   `json:"approved_at,omitempty"`
	PaymentReference string       `gorm:"size:255" json:"payment_reference,omitempty"`
	Notes            string       `gorm:"type:text" json:"notes,omitempty"`
	CreatedBy        string       `gorm:"size:255;not null" json:"created_by"`
	UpdatedBy        string       `gorm:"size:255" json:"updated_by,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	DeletedAt        *time.Time   `gorm:"index" json:"deleted_at,omitempty"`
	Lines            []RABillLine `gorm:"foreignKey:RABillID" json:"lines,omitempty"`
}

func (RABill) TableName() string {
	return "ra_bills"
}

// RABillLine holds itemized bill quantities and amounts.
type RABillLine struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	RABillID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"ra_bill_id"`
	RABill     *RABill    `gorm:"foreignKey:RABillID" json:"ra_bill,omitempty"`
	BOQItemID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"boq_item_id"`
	BOQItem    *BOQItem   `gorm:"foreignKey:BOQItemID" json:"boq_item,omitempty"`
	MBEntryID  *uuid.UUID `gorm:"type:uuid;index" json:"mb_entry_id,omitempty"`
	MBEntry    *MBEntry   `gorm:"foreignKey:MBEntryID" json:"mb_entry,omitempty"`
	Quantity   float64    `gorm:"type:decimal(15,4);not null" json:"quantity"`
	Rate       float64    `gorm:"type:decimal(15,2);not null" json:"rate"`
	Amount     float64    `gorm:"type:decimal(15,2);not null" json:"amount"`
	LineRemark string     `gorm:"type:text" json:"line_remark,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (RABillLine) TableName() string {
	return "ra_bill_lines"
}

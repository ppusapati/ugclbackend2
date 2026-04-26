# Project Management Phase 1 API Contracts

These APIs are registered under the existing `/api/v1` base prefix.

## WBS Nodes

### Create WBS Node
- Method: `POST`
- Path: `/api/v1/projects/{id}/wbs-nodes`
- Permission: `project:wbs_manage`

Request:
```json
{
  "parent_id": "optional-uuid",
  "code": "PKG-CIV-001",
  "name": "Civil Foundations",
  "description": "Foundation and pedestals",
  "node_type": "package",
  "sort_order": 10,
  "planned_start_date": "2026-05-01T00:00:00Z",
  "planned_end_date": "2026-05-25T00:00:00Z",
  "weightage": 20
}
```

### List WBS Nodes
- Method: `GET`
- Path: `/api/v1/projects/{id}/wbs-nodes`
- Permission: `project:wbs_read`
- Query: `parent_id`, `node_type`

## Task Dependencies

### Create Task Dependency
- Method: `POST`
- Path: `/api/v1/projects/{id}/task-dependencies`
- Permission: `task:dependency_manage`

Request:
```json
{
  "predecessor_task_id": "uuid",
  "successor_task_id": "uuid",
  "dependency_type": "FS",
  "lag_days": 2,
  "notes": "Curing gap"
}
```

### List Task Dependencies
- Method: `GET`
- Path: `/api/v1/projects/{id}/task-dependencies`
- Permission: `task:dependency_read`
- Query: `task_id`

## BOQ

### Create BOQ Item
- Method: `POST`
- Path: `/api/v1/projects/{id}/boq-items`
- Permission: `project:boq_manage`

Request:
```json
{
  "wbs_node_id": "optional-uuid",
  "code": "BOQ-STEEL-01",
  "description": "Rebar placement",
  "uom": "kg",
  "planned_quantity": 15000,
  "unit_rate": 72,
  "planned_amount": 1080000
}
```

### List BOQ Items
- Method: `GET`
- Path: `/api/v1/projects/{id}/boq-items`
- Permission: `project:boq_read`
- Query: `status`

## Measurement Book (MB)

### Create MB Entry
- Method: `POST`
- Path: `/api/v1/projects/{id}/mb-entries`
- Permission: `project:mb_manage`

Request:
```json
{
  "boq_item_id": "uuid",
  "entry_number": "MB-2026-0012",
  "measurement_date": "2026-06-12T00:00:00Z",
  "measured_qty": 245.5,
  "rate": 72,
  "amount": 17676,
  "location_ref": "Zone-A / Row-8",
  "remarks": "Verified by site engineer"
}
```

### List MB Entries
- Method: `GET`
- Path: `/api/v1/projects/{id}/mb-entries`
- Permission: `project:mb_read`
- Query: `boq_item_id`

## RA Bills

### Create RA Bill
- Method: `POST`
- Path: `/api/v1/projects/{id}/ra-bills`
- Permission: `project:billing_manage`

Request:
```json
{
  "bill_number": "RA-03",
  "period_start": "2026-06-01T00:00:00Z",
  "period_end": "2026-06-30T23:59:59Z",
  "gross_amount": 0,
  "deductions_amount": 12000,
  "retention_amount": 25000,
  "tax_amount": 18000,
  "net_amount": 0,
  "notes": "June cycle"
}
```

### Add RA Bill Line
- Method: `POST`
- Path: `/api/v1/projects/{id}/ra-bills/{billId}/lines`
- Permission: `project:billing_manage`

Request:
```json
{
  "boq_item_id": "uuid",
  "mb_entry_id": "optional-uuid",
  "quantity": 245.5,
  "rate": 72,
  "amount": 17676,
  "remark": "Derived from MB-2026-0012"
}
```

### List RA Bills
- Method: `GET`
- Path: `/api/v1/projects/{id}/ra-bills`
- Permission: `project:billing_read`
- Query: `status`

### Get RA Bill (with lines)
- Method: `GET`
- Path: `/api/v1/projects/{id}/ra-bills/{billId}`
- Permission: `project:billing_read`

### Status Transitions
- Submit: `POST /api/v1/projects/{id}/ra-bills/{billId}/submit` permission `project:billing_submit`
- Approve: `POST /api/v1/projects/{id}/ra-bills/{billId}/approve` permission `project:billing_approve`
- Reject: `POST /api/v1/projects/{id}/ra-bills/{billId}/reject` permission `project:billing_approve`
- Pay: `POST /api/v1/projects/{id}/ra-bills/{billId}/pay` permission `project:billing_pay`

Allowed transitions:
- `draft -> submitted`
- `submitted -> approved`
- `submitted -> rejected`
- `rejected -> submitted`
- `approved -> paid`

## Scope Rules
- All endpoints are project-scoped by path parameter `{id}`.
- If business context exists, project must belong to current business vertical.
- Create/transition operations require authenticated user claims and persist actor IDs.

# Project Management Phase 1 Permission Matrix

## Permissions Seeded in Migration

| Permission | Scope | Typical Assignees |
|---|---|---|
| project:wbs_read | Project | PM, Planning Engineer, Commercial Lead |
| project:wbs_manage | Project | PM, Planning Engineer |
| task:dependency_read | Project | PM, Planning Engineer |
| task:dependency_manage | Project | PM, Planning Engineer |
| project:boq_read | Project | PM, QS, Finance Reviewer |
| project:boq_manage | Project | QS, Commercial Manager |
| project:mb_read | Project | PM, QS, Audit |
| project:mb_manage | Project | Site Engineer, QS |
| project:billing_read | Project | PM, Finance Reviewer, Accounts |
| project:billing_manage | Project | QS, Commercial Manager |
| project:billing_submit | Project | Commercial Manager |
| project:billing_approve | Project | Project Director, Finance Approver |
| project:billing_pay | Project | Accounts Payable |

## Suggested Role Bundles

### Construction
- Site Engineer: `project:wbs_read`, `project:boq_read`, `project:mb_manage`, `project:mb_read`
- Planning Engineer: `project:wbs_read`, `project:wbs_manage`, `task:dependency_read`, `task:dependency_manage`
- Quantity Surveyor: `project:boq_read`, `project:boq_manage`, `project:mb_read`, `project:billing_manage`
- Commercial Manager: `project:billing_read`, `project:billing_manage`, `project:billing_submit`
- Project Director: `project:billing_read`, `project:billing_approve`

### Solar
- EPC Planning Lead: `project:wbs_read`, `project:wbs_manage`, `task:dependency_manage`
- Commissioning QS: `project:boq_manage`, `project:mb_manage`, `project:billing_manage`
- O&M Finance: `project:billing_read`, `project:billing_approve`, `project:billing_pay`

### Water
- Pipeline Planning Lead: `project:wbs_manage`, `task:dependency_manage`
- Measurement Engineer: `project:mb_manage`, `project:mb_read`, `project:boq_read`
- Commercial Approver: `project:billing_submit`, `project:billing_approve`, `project:billing_read`

## Governance Notes
- Keep `project:billing_pay` separate from `project:billing_approve` for SoD.
- Restrict `project:wbs_manage` and `task:dependency_manage` to planning roles only.
- Apply site-level access checks in MB/BOQ extensions if line items become site-partitioned.

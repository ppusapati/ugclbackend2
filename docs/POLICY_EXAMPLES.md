# UGCL Policy Examples

This document contains comprehensive policy examples tailored for UGCL's Water Works and Solar businesses.

---

## Table of Contents

1. [Financial & Purchase Policies](#financial--purchase-policies)
2. [Site Access & Operations Policies](#site-access--operations-policies)
3. [Data Security & Privacy Policies](#data-security--privacy-policies)
4. [Approval & Workflow Policies](#approval--workflow-policies)
5. [Time & Location-Based Policies](#time--location-based-policies)
6. [Role & Hierarchy Policies](#role--hierarchy-policies)
7. [Water Works Specific Policies](#water-works-specific-policies)
8. [Solar Works Specific Policies](#solar-works-specific-policies)
9. [Contractor & Vendor Policies](#contractor--vendor-policies)
10. [Audit & Compliance Policies](#audit--compliance-policies)

---

## Financial & Purchase Policies

### 1. High-Value Purchase Approval

**Scenario**: Purchases over ₹500,000 require manager approval during business hours only.

```json
{
  "name": "high_value_purchase_approval",
  "display_name": "High-Value Purchase Approval Required",
  "description": "Purchases exceeding ₹5 lakh require manager approval during business hours",
  "effect": "DENY",
  "priority": 100,
  "status": "active",
  "actions": ["purchase:create", "purchase:approve"],
  "resources": ["purchase"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.amount",
        "operator": ">",
        "value": 500000
      },
      {
        "OR": [
          {
            "attribute": "environment.hour",
            "operator": "<",
            "value": 9
          },
          {
            "attribute": "environment.hour",
            "operator": ">=",
            "value": 18
          }
        ]
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["super_admin", "System_Admin"]
      }
    ]
  }
}
```

### 2. Emergency Purchase Exception

**Scenario**: Emergency purchases up to ₹1 crore allowed 24/7 for managers.

```json
{
  "name": "emergency_purchase_exception",
  "display_name": "Emergency Purchase Exception",
  "description": "Managers can make emergency purchases up to ₹1 crore anytime",
  "effect": "ALLOW",
  "priority": 110,
  "status": "active",
  "actions": ["purchase:create"],
  "resources": ["purchase"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.amount",
        "operator": "<=",
        "value": 10000000
      },
      {
        "attribute": "resource.is_emergency",
        "operator": "=",
        "value": "true"
      },
      {
        "attribute": "user.role",
        "operator": "IN",
        "value": ["Manager", "Admin", "super_admin"]
      }
    ]
  }
}
```

### 3. Payment Approval Hierarchy

**Scenario**: Payments require approval based on amount tiers.

```json
{
  "name": "payment_approval_hierarchy",
  "display_name": "Payment Approval Hierarchy",
  "description": "Payments over ₹10 lakh require finance head approval",
  "effect": "DENY",
  "priority": 90,
  "status": "active",
  "actions": ["payment:approve"],
  "resources": ["payment"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.amount",
        "operator": ">",
        "value": 1000000
      },
      {
        "attribute": "user.department",
        "operator": "!=",
        "value": "finance"
      },
      {
        "attribute": "user.clearance_level",
        "operator": "<",
        "value": 4
      }
    ]
  }
}
```

### 4. Weekend Financial Operations Restriction

**Scenario**: No financial operations on weekends except by finance team.

```json
{
  "name": "weekend_financial_restriction",
  "display_name": "Weekend Financial Operations Restriction",
  "description": "Financial operations restricted on weekends except for finance team",
  "effect": "DENY",
  "priority": 80,
  "status": "active",
  "actions": ["payment:create", "payment:approve", "finance:create"],
  "resources": ["payment", "finance"],
  "conditions": {
    "AND": [
      {
        "attribute": "environment.day_of_week",
        "operator": "IN",
        "value": ["Saturday", "Sunday"]
      },
      {
        "attribute": "user.department",
        "operator": "!=",
        "value": "finance"
      },
      {
        "attribute": "user.role",
        "operator": "!=",
        "value": "super_admin"
      }
    ]
  }
}
```

---

## Site Access & Operations Policies

### 5. Site Geographic Access Control

**Scenario**: Users can only access sites within their assigned region.

```json
{
  "name": "site_geographic_access",
  "display_name": "Site Geographic Access Control",
  "description": "Users can only access sites in their assigned region",
  "effect": "DENY",
  "priority": 85,
  "status": "active",
  "actions": ["site:access", "dprsite:create", "dprsite:update"],
  "resources": ["site", "dprsite"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.assigned_region",
        "operator": "!=",
        "value": "{{resource.site_region}}"
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["super_admin", "Admin"]
      }
    ]
  }
}
```

### 6. On-Site Reporting Requirement

**Scenario**: DPR reports must be created from site location (within 5km radius).

```json
{
  "name": "onsite_reporting_requirement",
  "display_name": "On-Site Reporting Requirement",
  "description": "DPR reports must be created from site location",
  "effect": "DENY",
  "priority": 95,
  "status": "active",
  "actions": ["dprsite:create"],
  "resources": ["dprsite"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.current_location",
        "operator": "NOT_WITHIN_RADIUS",
        "value": {
          "site_id": "{{resource.site_id}}",
          "radius_km": 5
        }
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["super_admin", "Admin"]
      }
    ]
  }
}
```

### 7. Multi-Site Access Restriction

**Scenario**: Engineers can only report on one site per day.

```json
{
  "name": "single_site_per_day",
  "display_name": "Single Site Per Day Restriction",
  "description": "Engineers limited to one site per day",
  "effect": "DENY",
  "priority": 70,
  "status": "active",
  "actions": ["dprsite:create"],
  "resources": ["dprsite"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.today_site_count",
        "operator": ">=",
        "value": 1
      },
      {
        "attribute": "resource.site_id",
        "operator": "!=",
        "value": "{{user.today_active_site}}"
      },
      {
        "attribute": "user.role",
        "operator": "IN",
        "value": ["Engineer", "Supervisor"]
      }
    ]
  }
}
```

---

## Data Security & Privacy Policies

### 8. Confidential Data Access by Clearance

**Scenario**: Confidential financial data requires clearance level 4+.

```json
{
  "name": "confidential_data_clearance",
  "display_name": "Confidential Data Access Control",
  "description": "Confidential data requires clearance level 4 or higher",
  "effect": "DENY",
  "priority": 100,
  "status": "active",
  "actions": ["read", "update", "delete"],
  "resources": ["payment", "payroll", "finance"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.sensitivity",
        "operator": "IN",
        "value": ["confidential", "secret"]
      },
      {
        "attribute": "user.clearance_level",
        "operator": "<",
        "value": 4
      }
    ]
  }
}
```

### 9. Cross-Department Data Isolation

**Scenario**: HR data only accessible by HR department.

```json
{
  "name": "hr_data_isolation",
  "display_name": "HR Data Isolation",
  "description": "HR data restricted to HR department only",
  "effect": "DENY",
  "priority": 95,
  "status": "active",
  "actions": ["read", "update", "delete"],
  "resources": ["hr", "payroll"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.department",
        "operator": "!=",
        "value": "hr"
      },
      {
        "attribute": "user.role",
        "operator": "!=",
        "value": "super_admin"
      }
    ]
  }
}
```

### 10. Resource Owner Full Access

**Scenario**: Users have full access to resources they created.

```json
{
  "name": "resource_owner_access",
  "display_name": "Resource Owner Full Access",
  "description": "Resource owners always have full access",
  "effect": "ALLOW",
  "priority": 105,
  "status": "active",
  "actions": ["*"],
  "resources": ["*"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.id",
        "operator": "=",
        "value": "{{resource.owner_id}}"
      }
    ]
  }
}
```

---

## Approval & Workflow Policies

### 11. Manager Approval for Team Actions

**Scenario**: Managers must approve actions by their team members.

```json
{
  "name": "manager_approval_required",
  "display_name": "Manager Approval Required",
  "description": "Team actions require manager approval",
  "effect": "DENY",
  "priority": 75,
  "status": "active",
  "actions": ["expense:create", "leave:create", "purchase:create"],
  "resources": ["expense", "leave", "purchase"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.amount",
        "operator": ">",
        "value": 10000
      },
      {
        "attribute": "resource.status",
        "operator": "!=",
        "value": "approved"
      },
      {
        "attribute": "user.role",
        "operator": "IN",
        "value": ["Engineer", "Supervisor", "Consultant"]
      }
    ]
  }
}
```

### 12. Hierarchical Approval Chain

**Scenario**: Approvals must follow organizational hierarchy.

```json
{
  "name": "hierarchical_approval",
  "display_name": "Hierarchical Approval Chain",
  "description": "Approvals must follow org hierarchy",
  "effect": "DENY",
  "priority": 88,
  "status": "active",
  "actions": ["approve"],
  "resources": ["*"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.role_level",
        "operator": ">=",
        "value": "{{resource.creator_role_level}}"
      },
      {
        "attribute": "user.role",
        "operator": "!=",
        "value": "super_admin"
      }
    ]
  }
}
```

---

## Time & Location-Based Policies

### 13. After-Hours Access Control

**Scenario**: Non-critical operations restricted after hours.

```json
{
  "name": "after_hours_restriction",
  "display_name": "After Hours Access Control",
  "description": "Non-critical operations restricted after 10 PM",
  "effect": "DENY",
  "priority": 65,
  "status": "active",
  "actions": ["create", "update", "delete"],
  "resources": ["material", "inventory", "stock"],
  "conditions": {
    "AND": [
      {
        "OR": [
          {
            "attribute": "environment.hour",
            "operator": ">=",
            "value": 22
          },
          {
            "attribute": "environment.hour",
            "operator": "<",
            "value": 6
          }
        ]
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["super_admin", "Admin"]
      }
    ]
  }
}
```

### 14. Office IP Restriction

**Scenario**: Sensitive operations only from office network.

```json
{
  "name": "office_ip_restriction",
  "display_name": "Office IP Restriction",
  "description": "Sensitive operations require office network",
  "effect": "DENY",
  "priority": 92,
  "status": "active",
  "actions": ["payroll:generate", "finance:approve", "user:delete"],
  "resources": ["payroll", "finance", "user"],
  "conditions": {
    "AND": [
      {
        "attribute": "environment.ip_address",
        "operator": "NOT_STARTS_WITH",
        "value": "192.168.1"
      },
      {
        "attribute": "user.role",
        "operator": "!=",
        "value": "super_admin"
      }
    ]
  }
}
```

---

## Water Works Specific Policies

### 15. Water Quality Data Access

**Scenario**: Water quality data requires engineer certification.

```json
{
  "name": "water_quality_certification",
  "display_name": "Water Quality Data Certification",
  "description": "Water quality data requires certified engineers",
  "effect": "DENY",
  "priority": 90,
  "status": "active",
  "actions": ["water:update", "water:quality_control"],
  "resources": ["water"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.certification",
        "operator": "NOT_CONTAINS",
        "value": "water_quality"
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["super_admin", "Water_Admin"]
      }
    ]
  }
}
```

### 16. Water Supply Management

**Scenario**: Water supply changes require approval above threshold.

```json
{
  "name": "water_supply_approval",
  "display_name": "Water Supply Change Approval",
  "description": "Major water supply changes require approval",
  "effect": "DENY",
  "priority": 85,
  "status": "active",
  "actions": ["water:manage_supply"],
  "resources": ["water"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.volume_change",
        "operator": ">",
        "value": 10000
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["Water_Admin", "Sr_Deputy_PM", "super_admin"]
      }
    ]
  }
}
```

### 17. Water Consumption Monitoring

**Scenario**: Abnormal consumption requires supervisor review.

```json
{
  "name": "abnormal_consumption_alert",
  "display_name": "Abnormal Water Consumption Alert",
  "description": "Abnormal consumption requires supervisor review",
  "effect": "DENY",
  "priority": 80,
  "status": "active",
  "actions": ["water:read_consumption"],
  "resources": ["water"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.consumption_variance",
        "operator": ">",
        "value": 50
      },
      {
        "attribute": "resource.reviewed_by_supervisor",
        "operator": "!=",
        "value": "true"
      }
    ]
  }
}
```

---

## Solar Works Specific Policies

### 18. Solar Panel Configuration

**Scenario**: Solar panel configuration requires certified technicians.

```json
{
  "name": "solar_panel_certification",
  "display_name": "Solar Panel Configuration Certification",
  "description": "Panel configuration requires certified technicians",
  "effect": "DENY",
  "priority": 95,
  "status": "active",
  "actions": ["solar:manage_panels"],
  "resources": ["solar"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.certification",
        "operator": "NOT_CONTAINS",
        "value": "solar_technician"
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["Solar_Admin", "Area_Project_Manager", "super_admin"]
      }
    ]
  }
}
```

### 19. Solar Generation Data Accuracy

**Scenario**: Generation data variance requires verification.

```json
{
  "name": "solar_generation_verification",
  "display_name": "Solar Generation Data Verification",
  "description": "High variance generation data requires verification",
  "effect": "DENY",
  "priority": 85,
  "status": "active",
  "actions": ["solar:read_generation"],
  "resources": ["solar"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.generation_variance",
        "operator": ">",
        "value": 30
      },
      {
        "attribute": "resource.verified",
        "operator": "!=",
        "value": "true"
      }
    ]
  }
}
```

### 20. Solar Maintenance Windows

**Scenario**: Solar maintenance only during non-peak hours.

```json
{
  "name": "solar_maintenance_window",
  "display_name": "Solar Maintenance Time Window",
  "description": "Maintenance restricted to non-peak hours (10 PM - 6 AM)",
  "effect": "DENY",
  "priority": 75,
  "status": "active",
  "actions": ["solar:maintenance"],
  "resources": ["solar"],
  "conditions": {
    "AND": [
      {
        "attribute": "environment.hour",
        "operator": "BETWEEN",
        "value": [6, 22]
      },
      {
        "attribute": "resource.is_emergency",
        "operator": "!=",
        "value": "true"
      },
      {
        "attribute": "user.role",
        "operator": "!=",
        "value": "super_admin"
      }
    ]
  }
}
```

---

## Contractor & Vendor Policies

### 21. Contractor Read-Only Access

**Scenario**: Contractors have read-only access to assigned projects.

```json
{
  "name": "contractor_readonly",
  "display_name": "Contractor Read-Only Access",
  "description": "Contractors limited to read-only on assigned projects",
  "effect": "DENY",
  "priority": 100,
  "status": "active",
  "actions": ["create", "update", "delete", "approve"],
  "resources": ["*"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.employment_type",
        "operator": "=",
        "value": "contractor"
      }
    ]
  }
}
```

### 22. Vendor Data Scope Limitation

**Scenario**: Vendors can only access data for their contracts.

```json
{
  "name": "vendor_scope_limitation",
  "display_name": "Vendor Data Scope Limitation",
  "description": "Vendors limited to their contract data",
  "effect": "DENY",
  "priority": 98,
  "status": "active",
  "actions": ["read", "update"],
  "resources": ["project", "material", "inventory"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.employment_type",
        "operator": "IN",
        "value": ["contractor", "vendor"]
      },
      {
        "attribute": "resource.contract_id",
        "operator": "NOT_IN",
        "value": "{{user.assigned_contracts}}"
      }
    ]
  }
}
```

---

## Audit & Compliance Policies

### 23. Audit Trail Requirement

**Scenario**: Critical operations require audit trail with reason.

```json
{
  "name": "audit_trail_requirement",
  "display_name": "Audit Trail Requirement",
  "description": "Critical operations require documented reason",
  "effect": "DENY",
  "priority": 93,
  "status": "active",
  "actions": ["delete", "approve"],
  "resources": ["payment", "finance", "payroll"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.reason",
        "operator": "=",
        "value": ""
      },
      {
        "attribute": "user.role",
        "operator": "!=",
        "value": "super_admin"
      }
    ]
  }
}
```

### 24. Data Retention Policy

**Scenario**: Financial records cannot be deleted within 7 years.

```json
{
  "name": "financial_data_retention",
  "display_name": "Financial Data Retention Policy",
  "description": "Financial records retained for 7 years minimum",
  "effect": "DENY",
  "priority": 100,
  "status": "active",
  "actions": ["delete"],
  "resources": ["payment", "finance", "invoice"],
  "conditions": {
    "AND": [
      {
        "attribute": "resource.age_years",
        "operator": "<",
        "value": 7
      },
      {
        "attribute": "user.role",
        "operator": "!=",
        "value": "super_admin"
      }
    ]
  }
}
```

### 25. Compliance Reporting Access

**Scenario**: Compliance reports require special authorization.

```json
{
  "name": "compliance_report_access",
  "display_name": "Compliance Reporting Access",
  "description": "Compliance reports require special clearance",
  "effect": "DENY",
  "priority": 97,
  "status": "active",
  "actions": ["read", "export"],
  "resources": ["compliance_report", "audit_report"],
  "conditions": {
    "AND": [
      {
        "attribute": "user.has_compliance_access",
        "operator": "!=",
        "value": "true"
      },
      {
        "attribute": "user.role",
        "operator": "NOT_IN",
        "value": ["super_admin", "System_Admin", "Admin"]
      }
    ]
  }
}
```

---

## How to Import These Policies

Use the Policy Management API to create these policies:

```bash
# Example: Create a policy
curl -X POST http://localhost:8080/api/v1/policies \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY" \
  -d @policy.json
```

Or use the frontend Policy Management UI to import and test these policies.

---

## Policy Testing Checklist

Before activating a policy, test it with:

1. ✅ **Positive test**: Verify allowed actions work
2. ✅ **Negative test**: Verify denied actions are blocked
3. ✅ **Edge cases**: Test boundary conditions
4. ✅ **Super admin bypass**: Verify super admin always has access
5. ✅ **Performance**: Check evaluation time
6. ✅ **Audit logs**: Verify logging works

---

## Best Practices

1. **Start with draft status** - Test thoroughly before activating
2. **Use priority wisely** - Higher priority for security policies
3. **Document reasons** - Add clear descriptions
4. **Regular reviews** - Review policies quarterly
5. **Monitor evaluations** - Check audit logs for unexpected denials
6. **Version control** - Create new versions for major changes
7. **Approval workflow** - Use approval system for production policies

---

## Next Steps

1. Import relevant policies for your use cases
2. Customize attribute values for your organization
3. Test policies in staging environment
4. Get approval from stakeholders
5. Activate in production with monitoring
6. Train users on new policies
7. Set up alerts for policy violations

For more examples and custom policy creation, see [ABAC_IMPLEMENTATION_GUIDE.md](./ABAC_IMPLEMENTATION_GUIDE.md)

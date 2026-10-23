---
status: accepted
---

# Tenant and Department replace Hospital and Ward

SMtrack now serves laboratories and factories, not only hospitals. We replace
`Hospital → Ward → User` with **Tenant** (the unit of tenancy, with a `kind`) and
**Department**. A user belongs to exactly one tenant directly (`users.tenant_id`) and to any
number of that tenant's departments through `user_departments`. Departments are flat.
Each tenant may rename the two words per language in a label profile; labels are display
text and never affect authorization. Soft delete applies to users, tenants and departments.

## Considered Options

- **Keep Hospital/Ward and add a `type`**. Rejected: a factory "hospital" leaks the wrong
  vocabulary into the API, and the tenant of a user would still be derived through a ward.
- **One department per user** (`users.department_id`). Rejected: staff who cover several
  lines or wards are common, and a nullable single column cannot express them.
- **User in several tenants**. Rejected: it would force the token's `tenantId` to become a
  list or a "current tenant" switch, and every scope rule would need an active-tenant
  concept. Revisit only if a real case appears.
- **Department hierarchy**. Rejected for now: no requirement, and it complicates scope checks.

## Consequences

- Tenant scope is a comparison of `tenantId`, no longer a join through a ward.
- Non-admin users see only their own tenant and their linked departments (narrower than
  before, which a frontend must expect).
- Existing IDs are kept as `tenantId` / `departmentId`, so other services can keep their
  references.
- Label text is stored as JSONB per tenant; adding a label key needs no migration.

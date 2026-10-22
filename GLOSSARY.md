# SMtrack Authentication

Who may use the SMtrack platform, which tenant and departments they belong to, and what each
of them is allowed to see and change.

## Organisation

**Tenant**:
A customer organisation using SMtrack (a hospital, laboratory, factory, clinic, …), and the
unit of tenancy. Every user belongs to exactly one tenant, and every department belongs to
exactly one tenant.
_Avoid_: Hospital (as the general term), organisation, site, customer

**Tenant kind**:
What sort of organisation a tenant is: hospital, laboratory, factory, clinic, pharmacy or
other. It chooses the default labels shown to that tenant's users and lets the frontend
adapt, but it never changes what anyone is allowed to do.
_Avoid_: Tenant type, category

**Department**:
A subdivision of a tenant where monitored devices live (a ward, a production line, a lab
bench). A user may be linked to any number of departments of their own tenant, or none.
Departments are flat; one does not contain another.
_Avoid_: Ward (as the general term), unit, room, team

**Label profile**:
A tenant's own names for "tenant" and "department" in each language, such as "Factory" and
"Line", shown by the frontend to that tenant's users. Labels are display text only and never
affect access. A tenant without a profile gets the defaults for its kind.
_Avoid_: Alias, translation, custom name, theme

**Hidden tenant**:
A tenant that only SUPER can see, used internally for testing. It cannot be chosen when
registering.
_Avoid_: Development hospital, test tenant, sandbox

**Legacy**:
A flag on a tenant or department that was migrated from the earlier hospital/ward model and
still has its devices managed by the legacy service.
_Avoid_: Old, deprecated

## People and access

**User**:
An account that can sign in. It has a username, a role, a status, one tenant and any number
of departments within it.
_Avoid_: Account (for the record), member, login

**Actor**:
The user performing the current request, as identified by their access token.
_Avoid_: Principal, JWT payload, current user

**Role**:
The single permission level a user holds: SUPER, SERVICE, ADMIN, LEGACY_ADMIN, USER,
LEGACY_USER or GUEST.
_Avoid_: Permission, group, privilege

**Guest**:
The role every self-registered user starts with; it can see little until an admin promotes it.
_Avoid_: Anonymous, visitor, pending user

**Admin tier**:
The roles allowed to act on users other than themselves: SUPER, SERVICE, ADMIN and LEGACY_ADMIN.
_Avoid_: Admins (when SERVICE or SUPER are meant too), staff

**Tenant-scoped admin**:
An ADMIN or LEGACY_ADMIN: admin tier, but confined to their own tenant.
_Avoid_: Local admin, hospital admin

**Role ceiling**:
The highest role an actor may give to a user, which is also the highest role of a user the
actor may change or delete. SUPER may reach any role; SERVICE any but SUPER; a
tenant-scoped admin only USER, LEGACY_USER and GUEST. Nobody may change their own role.
_Avoid_: Role hierarchy, rank

**Tenant scope**:
The tenants whose records an actor may see or change. System-wide for SUPER and SERVICE, the
actor's own tenant for everyone else. Records outside it are treated as if they do not exist.
_Avoid_: Visibility, permissions, data scope

**Department access**:
Which departments of their tenant an actor can see. A tenant-scoped admin sees all of them;
any other role sees only the departments they are linked to.
_Avoid_: Department permission, membership

**Status**:
Whether a user is active. An inactive user cannot sign in or renew their session.
_Avoid_: Enabled, locked, banned

**Lockout**:
A temporary refusal to sign in to a username after repeated wrong passwords. It looks the
same as a wrong password to the caller.
_Avoid_: Ban, suspension

## Record keeping

**Soft delete**:
Removing a user, tenant or department from every normal view and from sign-in while keeping
the record so it can be restored. A deleted user's username stays reserved.
_Avoid_: Archive, deactivate (that is Status)

**Restore**:
Bringing a soft-deleted record back with its former status and links.
_Avoid_: Undelete, recover

**Purge**:
Permanently erasing records that have been soft-deleted for longer than the retention period.
It is a deliberate operator action, never automatic.
_Avoid_: Hard delete, cleanup

**Audit log**:
An append-only record of security-relevant events: who created, updated, deleted, restored or
purged a user, department or tenant and what changed, and sign-ins, failed sign-ins,
password resets and refused accesses. It cannot be edited or deleted, and it is not divided
by tenant.
_Avoid_: History, activity log, event log

**Access review**:
A report of every user with their role, status, tenant, departments and last sign-in, used to
check that people still hold only the access they need.
_Avoid_: Audit (that is the Audit log), user export

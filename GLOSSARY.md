# SMtrack Authentication

Who may use the SMtrack platform, which hospital and ward they belong to, and what each of
them is allowed to see and change.

## Organisation

**Hospital**:
A customer organisation using SMtrack, and the unit of tenancy. Every ward, and through it
every user, belongs to exactly one hospital.
_Avoid_: Tenant (as a noun for the record), site, organisation

**Ward**:
A department inside a hospital where monitored devices live. A user belongs to a hospital
only by belonging to one of its wards.
_Avoid_: Department, unit, room

**Development hospital**:
The internal hospital (`HID-DEVELOPMENT`) used for testing, hidden from every role except
SUPER.
_Avoid_: Test tenant, sandbox

## People and access

**User**:
An account that can sign in. It has a username, a role, a status and a ward.
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

**Hospital-scoped admin**:
An ADMIN or LEGACY_ADMIN: admin tier, but confined to their own hospital.
_Avoid_: Local admin, tenant admin

**Role ceiling**:
The highest role an actor may give to a user, which is also the highest role of a user the
actor may change or delete. SUPER may reach any role; SERVICE any but SUPER; a
hospital-scoped admin only USER, LEGACY_USER and GUEST. Nobody may change their own role.
_Avoid_: Role hierarchy, rank

**Tenant scope**:
The hospitals whose records an actor may see or change. System-wide for SUPER and
SERVICE, the actor's own hospital for a hospital-scoped admin. Records outside it are
treated as if they do not exist.
_Avoid_: Visibility, permissions, data scope

**Status**:
Whether a user is active. An inactive user cannot sign in or renew their session.
_Avoid_: Enabled, locked, banned

## Record keeping

**Audit log**:
An append-only record of who created, updated or deleted a user, ward or hospital, and what
changed. It is not divided by hospital.
_Avoid_: History, activity log, event log

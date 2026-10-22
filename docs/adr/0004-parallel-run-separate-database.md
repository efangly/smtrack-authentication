---
status: accepted
---

# The new version runs beside the old one on its own database

The tenant/department version is deployed next to the existing version and the old one is
shut down only after the new one is proven. The new version uses a separate PostgreSQL
database created from migrations (`goose`), separate Redis keys and new JWT secrets. A
re-runnable `cmd/import-legacy` copies the old data in, keeping IDs and password hashes, and
`--verify` must pass before cutover. At cutover the old version's writes are briefly
refused, a final import runs, and the gateway is switched. Rollback means switching back;
changes made on the new version are not copied back.

## Considered Options

- **Expand/contract on the shared database**. Rejected: while both versions run, the old one
  keeps writing `Wards` and `Users.wardId` and the new one reads `Tenants`, so the two
  models drift apart from the first new registration unless every write goes to both.
- **Reverse sync after cutover**. Rejected: a large piece of work for a rollback window that
  is expected to be short.

## Consequences

- Users sign in again once at cutover, because the secrets differ.
- Frontend and device service switch at the same time as the gateway; no alias routes, old
  token claims or old event names are kept.
- The old database is only ever read by the import tool.

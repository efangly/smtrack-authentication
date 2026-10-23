# Cutover and rollback runbook — hospital/ward version → tenant/department version

Context: ADR 0003 (model) and ADR 0004 (parallel run on a separate database). The new version
(`authentication-v2`) runs beside the old one; nothing about the old deployment changes until
the gateway is switched. Frontend and the device service switch **at the same time** as the
gateway (no alias routes, no old claims, no old event names).

Roles: **operator** runs the steps; **gateway owner** changes KrakenD; **frontend/device owners**
deploy their side. Allow a short write freeze.

## 0. Preconditions (days before)

- [ ] New image built from `main` and pushed (`siamatic/smtrack-auth-service:3.0.0`); PR checks green.
- [ ] A new, empty PostgreSQL database and a runtime role for it. The role must not own `audit_logs`
      (see `docs/security/iso27001-controls.md`). Migrations create the schema on first start; run
      them once with an owner role if the runtime role is restricted.
- [ ] `authentication-v2-secrets` created (new JWT secrets ≥ 32 chars, different from each other;
      new `DATABASE_URL`). Redis: use a separate instance, or at least expect new keys (`auth:*`).
- [ ] `RABBITMQ` left **unset** on v2 until cutover so no `update-tenant` / `update-department`
      events reach consumers that still expect the old names.
- [ ] Frontend and device service builds ready that use `/auth/tenant`, `/auth/department`,
      `tenantId`, `departmentIds`, the new token claims, token refresh every <15 min (access token
      is now 15 minutes) and `POST /auth/logout`.
- [ ] Password rule is unchanged (4–20), so no user action is needed beyond signing in once.
- [ ] Decide each tenant's label overrides (optional; defaults come from the tenant kind:
      hospital/ward for HOSPITAL).

## 1. Rehearse on a snapshot (must pass before the real run)

1. Restore a production snapshot of the **old** database into a scratch server.
2. Create an empty scratch database for v2.
3. Run the import twice and verify (see §3 for the command). Both runs must finish `ok`; the second
   must change nothing; `--verify` must print no problems.
4. If the import stops with problems (duplicate ward names in one hospital, a ward pointing at a
   missing hospital, a user pointing at a missing ward, an unknown role or `hosType`), fix them in
   the **old** database (rename the ward, reassign the user) and repeat. The tool never renames or
   drops anything itself.
5. Start v2 against the scratch database; sign in as a real user from the snapshot; check one
   hospital admin sees only their tenant; check `/auth/tenant` and `/auth/department`.

## 2. Deploy v2 beside v1 (no traffic)

```bash
kubectl apply -f k8s/deploy-v2.yaml
kubectl -n smtrack rollout status deploy/authentication-v2
kubectl -n smtrack apply -f k8s/purge-cronjob.yaml   # optional now; fine to defer
```

Check `GET /auth/health` through the v2 Service. Migrations ran at startup (look for
"Database connected successfully"). Smoke-test with a throw-away tenant and user on the **v2**
database only, then delete or leave them as a soft-deleted record.

## 3. Import (first pass can be done days ahead; the final pass is in the freeze)

Create the Secret `authentication-legacy-db` (key `LEGACY_DATABASE_URL`, preferably a read-only
role) and run the Job in `k8s/import-legacy-job.yaml`; set `args: ["--verify"]` for the check (see
the header of that file for the commands). Passing the URLs on a command line would leave them in
shell history, so use the Secrets.

The tool only reads the old database and refuses to run
if both URLs are identical. It is safe to re-run: it upserts by the old IDs and keeps `seq`
numbers, password hashes, ward → department membership and audit history (old entity names `WARD`
and `HOSPITAL` stay as history).

## 4. Freeze and switch (target: minutes)

1. **Announce** the window. Gateway owner makes the **old** version reject writes
   (`POST/PUT/PATCH/DELETE` on `/auth/*`); logins keep working if possible (they only read, but
   note that a write-freeze also stops registrations).
2. **Final import**, then **`--verify`**. Do not continue unless verify prints no problems.
3. Label overrides, if any: `PUT /auth/tenant/:id/labels` (as SUPER/SERVICE).
4. Set `RABBITMQ` on v2 (from the `rabbitmq-config` ConfigMap) and roll it:
   `kubectl -n smtrack rollout restart deploy/authentication-v2`.
5. Gateway owner points the `/auth` backend at the `authentication-v2` Service; frontend and
   device service deploy their new versions at the same moment.
6. Everyone signs in again once (new secrets, new claims). That is expected.

## 5. Verify after the switch

- [ ] Sign in as SUPER, a hospital ADMIN, a plain USER and (if any) a LEGACY_ADMIN. Each sees only
      what the scope rules allow (ADMIN: own tenant; USER: own tenant and linked departments).
- [ ] `GET /auth/audit?action=LOGIN_SUCCESS` shows the sign-ins with IP, user agent and request id.
- [ ] Device service resolves users/tenants/departments with the new routes.
- [ ] No `ACCESS_DENIED` or `LOGIN_FAILURE` burst in the audit log or Loki.
- [ ] `GET /auth/report/access` works for SUPER.

## 6. Rollback

**Switch the gateway (and frontend, device service) back to the old version.** The old database was
never written by the importer or v2, so it is intact and its data is what it was at the freeze.

- Anything created or changed **on v2 after the switch is not carried back**. There is no reverse
  sync (ADR 0004). If the window was long, list what changed from v2's audit log
  (`GET /auth/audit`, `action` CREATE/UPDATE/DELETE since the switch) and re-apply it by hand.
- Lift the write freeze on the old version.
- Users sign in again once more (v1 secrets).

## 7. After the new version is proven

- Keep the old version and its database for the agreed period, then retire them. Only then drop
  the `HID-DEVELOPMENT`-style special cases from any consumers.
- Enable the purge CronJob (`k8s/purge-cronjob.yaml`), start with `--dry-run` and review.
- Point `.github/workflows/main.yaml` at v2 only (it already rewrites `k8s/deploy-v2.yaml` and
  `k8s/purge-cronjob.yaml`, not `k8s/deploy.yaml`).
- Schedule the first access review with `GET /auth/report/access`.

## Known risks

- Non-admin users see less than before (own tenant and linked departments only).
- The 15-minute access token needs refresh logic in every client.
- `departmentIds` in the token grows with a user's department count.
- Rollback after cutover loses writes made on v2.

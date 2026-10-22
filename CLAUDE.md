# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this is

`smtrack-authentication` — the authentication/authorization microservice for the SMtrack
platform. Go 1.25, Fiber v3, GORM/PostgreSQL, Redis, RabbitMQ. It issues JWTs and owns the
`users`, `tenants`, `departments`, `user_departments`, `tenant_profiles` and `audit_logs`
tables. A tenant (hospital, laboratory, factory, …) has departments; a user belongs to one
tenant and any number of its departments (`GLOSSARY.md`, ADR 0003).

It sits behind a **KrakenD API gateway** in production. All routes are served under the
`/auth` prefix.

## Commands

```bash
go build ./...          # compile
go vet ./...            # static checks
go test ./...           # full test suite (no external services needed — everything is mocked)
go test ./internal/core/services/ -run TestAuthService   # single package / test
gofmt -l .              # must print nothing before committing
```

There is no Makefile and no lint config; `go vet` + `gofmt` are the bar.

Running the service locally needs PostgreSQL and Redis reachable, plus a `.env`
(copy `.env.example`). The schema is created by the `goose` migrations in `migrations/`,
applied at startup (`database.Migrate`, guarded by an advisory lock). This version runs
beside the old hospital/ward one on its own database (ADR 0004). RabbitMQ and the upload service are optional — the adapters
degrade to no-ops with a warning when unconfigured.

Two operator tools live in `cmd/`: `import-legacy` (copies the old hospital/ward database
into this one, re-runnable, `--verify` compares them; ADR 0004) and `purge` (really deletes
records soft-deleted longer than `SOFT_DELETE_RETENTION`, default 365d, with their pictures;
`--dry-run`). Their database tests are behind `-tags integration` and need
`TEST_DATABASE_URL` (plus `LEGACY_TEST_DATABASE_URL` for the import) pointing at empty
databases, because `audit_logs` rows cannot be cleaned up.

CI: `.github/workflows/pr.yaml` runs on every pull request — `go vet`, a `gofmt` check,
`go test ./...`, the integration tests against real Postgres and Redis, `govulncheck`, and a Trivy
scan of the built (not pushed) image. `.github/workflows/main.yaml` runs on pushes to `main`: the
same vet/gofmt/test, then it builds and pushes the Docker image and rewrites the image tag in
`k8s/deploy-v2.yaml`, `k8s/purge-cronjob.yaml` and `k8s/import-legacy-job.yaml`. Bump `IMAGE_NAME` in `main.yaml` for a release
(pushing an existing tag overwrites it). The image also contains `import-legacy` and `purge`.
Deployment, cutover and rollback: `docs/ops/cutover-runbook.md`; security mapping:
`docs/security/iso27001-controls.md`.

## Architecture

Hexagonal (ports & adapters). The layer map is in `README.md`; what matters when editing:

- **`internal/core/`** must not import adapters, `config`, Fiber, GORM, JWT, bcrypt,
  Redis, RabbitMQ, `mime/multipart` or `net/http` — `internal/arch_test.go` enforces it.
  Business rules live in `core/services/`, contracts in `core/ports/`.
- **Domain structs carry no `gorm`/`json` tags** (ADR 0001). Table shape lives in the
  persistence models in `adapters/driven/database/models.go`; JSON shape in the response DTOs
  in `adapters/driving/http/dto/mapping.go`. Never return a domain value from a handler. The
  arch test rejects tags in the domain (`tagAllowlist` is empty).
- **`core/ports/`** defines both driving ports (`services.go` — what the HTTP layer calls)
  and driven ports (`repositories.go`, `cache.go`, `upload.go`, `messaging.go`,
  `security.go` — what the services call). Adapters implement these; nothing else may be depended on directly.
- **`internal/adapters/driving/http/`** is the only place that knows about Fiber.
  Handlers translate HTTP ↔ DTO ↔ domain and enforce request-level authorization.
- Wiring happens exclusively in `main.go`. Constructors return port interfaces, not structs.

Adding a capability normally means: port method → service implementation → repository
method → handler → route in `router.go`.

## Conventions

- **Database names are snake_case and unquoted** (`tenant_id`, `created_at`); the JSON API is
  camelCase. Column names appear only in `adapters/driven/database` — the core never sees
  them. Change the schema by adding a numbered `migrations/*.sql`, never by editing an
  applied one.
- **Soft delete.** Users, tenants and departments carry `deleted_at` / `deleted_by`; the
  models use `gorm.DeletedAt`, so every ordinary read hides them. Only `FindAnyByID` (to
  authorize a restore) and `UsernameTaken` (a username stays reserved) look at deleted rows.
  Removing a record never deletes its picture or its `user_departments` links; a purge does.
  Department names are unique among live departments only, so a restore re-checks the name.
- **Partial updates** flow as a `domain.*Patch` (pointer fields; nil = unchanged) built by
  a DTO `ToPatch()` method. The repository turns it into a column map in
  `database/columns.go` and stamps `updated_at` itself. Audit entries record the patch via
  `services/audit_changes.go`, which never includes a password.
- **Listings are narrowed by `ports.*Filter`** structs built in `build*Filter`, which also
  return a cache key and `ok=false` when the caller's scope is empty (a tenant-scoped token
  without a tenant must list nothing, not everything). Hidden tenants are excluded by a
  subquery; `database/query_sql_test.go` checks the generated SQL with GORM's DryRun (a
  subquery is recorded before the main statement, so read the last one).
- **Cache keys** are prefixes: `user`, `user:<id>`, `user:tenant:<id>…`, `tenant…`,
  `department…`. `CachePort.Del` deletes by *prefix* (SCAN-based), so `Del(ctx, "user")`
  clears every user key. Anything that changes a tenant or department must also invalidate
  `tenant`, `department` and `user`, because those listings embed each other (ADR 0002). Services pass values to `CachePort`; the
  Redis adapter owns the JSON encoding. TTLs are `listTTL` / `itemTTL` in
  `services/cache_ttl.go`.
- **Errors:** domain errors are created with `errors.New` / `fmt.Errorf` **without** `%w`;
  infrastructure errors are always wrapped with `%w` at the adapter boundary. The HTTP
  layer relies on this — `handlers.serviceError` returns the message verbatim for
  unwrapped (domain) errors and replaces wrapped ones with a generic 500 so driver and SQL
  detail never reaches a client. **Keep that discipline when adding code.**
- **Validation** is declarative: `validate:` tags on DTOs, enforced automatically because
  `pkg/validator.Validator` is registered as Fiber's `StructValidator` in `main.go`.
  Adding a tag is enough; do not call the validator by hand in a handler.
- **Logging** goes through `pkg/logger` (slog JSON, shipped to Loki). Never `fmt.Println`.

## Security invariants

These were bugs once. Do not regress them.

- `POST /auth/register` is **public**. It must go through `CreateUserRequest.ToPublicUser()`,
  which forces `role=GUEST` and ignores `id`/`status`/`createBy` from the body. Never use
  `ToUser()` there — that path is only for the authenticated, role-guarded `POST /auth/user`.
- **Passwords are hashed in exactly two places:** `userService.Create` and
  `userService.Update`, through `ports.PasswordHasher` (bcrypt adapter in
  `adapters/driven/security`). `authService.Register` passes the plain password straight through to
  `Create` — it does not hash its own copy. Any map reaching `userService.Update` with a
  `password` key holds the *plain* value and gets hashed there. `UpdateUserRequest`
  deliberately has no password field — password changes go through `PATCH /auth/reset/:username`.
- **`/auth/user/:id` routes are ownership-checked in the handler**, not just by role:
  a non-admin-tier caller may only touch its own record, and `role`/`status`/`tenantId`/
  `departmentIds` are stripped from its update. Use `domain.Actor.IsAdminTier()` for that test —
  admin tier is SUPER, SERVICE, ADMIN, LEGACY_ADMIN.
- **Tenant scope is enforced in the services.** ADMIN/LEGACY_ADMIN may only read, change
  or place users in their own tenant (`canAdminTenant` in `core/services/scope.go`);
  `FindOne` for tenants and departments mirrors the matching `build*Filter` listing
  (`canSeeTenant`, `canSeeDepartment`). Non-admin roles see only their own tenant and only
  the departments they are linked to; a hidden tenant is left out of every admin listing except SUPER's. Out-of-scope
  records read as *not found*. Every caller-facing `FindOne` takes the caller; the unscoped
  `UserService.FindByID` / `FindByUsername` exist only for the auth flows. A user's
  departments must belong to the user's tenant (`checkDepartments`), and moving a user to
  another tenant requires saying which departments it has there. A tenant can be deleted
  only when empty. `/auth/audit` is SUPER/SERVICE only because logs are not tenant-scoped.
- **Role ceiling is enforced in `userService`, not the handler.** `domain.Role.CanAssign`
  (via `domain.Actor.CanAssign`)
  decides which roles a caller may assign *and* which existing users it may update or
  delete: SUPER → any; SERVICE → any but SUPER; ADMIN/LEGACY_ADMIN → USER, LEGACY_USER,
  GUEST. Nobody may change their own role. A nil caller may only create a GUEST (public
  register) and may not update or remove. Authorization reads the target from the
  repository, never the cache.
- **Every mutating route carries a `RolesGuard`.** `PUT`/`DELETE /auth/ward/:id` once had
  none, so any token — including a self-registered GUEST — could rename or delete any ward.
  `TestRouter_MutatingRoutes_RequireTheirRoles` lists every one (restore routes included).
  `internal/adapters/driving/http/router_test.go` drives the real route table; extend it
  when adding a guarded route.
- **A deactivated account (`status=false`) cannot log in.** `authService.ValidateUser`
  rejects it *after* the bcrypt comparison and returns the same `nil, nil` as a wrong
  password, so neither the message nor the timing reveals that the account exists.
- **Access and refresh tokens are not interchangeable.** They use different secrets and a
  `typ` claim checked by `adapters/driven/security/jwt.go`; a token without `typ` is
  rejected. Claims are `tenantId` and `departmentIds`.
- **No password-strength policy yet.** DTOs keep the old rule (4–20 characters) so users carried
  over from the old system are not affected. A stronger rule (ISO/IEC 27001 gap) belongs in
  `userService.Create` / `Update`, the two places that hash, and not at login.
- **Lockout:** 5 failed logins per username (case-insensitive) in a 15-minute window, counted in
  Redis through `ports.SessionStore`. A locked username answers exactly like a wrong password and
  still costs a hash. A Redis outage lets logins through (logged); it must not lock everyone out.
- **Sessions:** access tokens last 15 minutes, refresh tokens 7 days and carry a `jti`.
  `POST /auth/logout` denylists the refresh token's `jti`; a password reset sets the user's
  "tokens valid after" mark. Both are checked in `authService.RefreshTokens`, which **fails
  closed** when Redis cannot be read. Access tokens are not checked against Redis (they simply
  expire), so a logout takes full effect within 15 minutes. `SessionStore` keys start with
  `auth:` so `CachePort.Del` can never clear them.
- **Audit trail:** `middleware.RequestMeta` puts IP (trusted-proxy rules), user agent and request
  id into the context; `auditService.Record` copies them and the caller's tenant onto the row.
  Events: CREATE/UPDATE/DELETE/RESTORE/PURGE plus LOGIN_SUCCESS/LOGIN_FAILURE (entity id = the
  username tried), LOGOUT, TOKEN_REFRESH, PASSWORD_RESET, ACCESS_DENIED (`middleware.AuditDenied`
  records every 403) and ACCESS_REPORT. `GET /auth/report/access` reads the database directly, is
  scoped like the user listing and is itself audited.
- **Startup refuses weak secrets:** `config.Config.Validate` requires both JWT secrets to be at
  least 32 characters and different, and `BCRYPT_COST` >= 12.
- **Refresh rebuilds claims from the database** (`authService.RefreshTokens`), never by
  copying the old token's claims, so a demoted or deactivated user loses access at the next
  refresh instead of keeping it for the token's remaining lifetime.
- **No password hash leaves the service.** Every user written to the cache goes through
  `domain.User.WithoutSecrets()` (or `UsersWithoutSecrets`) first, audit entries use the
  `*Snapshot` maps in `services/audit_changes.go`, and `dto.FromUser` has no password field.
  That is also why `FindByID` / `FindByUsername` are uncached: the auth flows need the hash.
- **`pic` cannot be set from a request body**, only by uploading an image.
  Accepting a URL let a user point their record at someone else's file, which the next
  upload then deleted from the image service.
- **Uploads go through `handlers.formFile`.** It caps the image at `maxImageBytes`, sniffs
  the type from the bytes (JPEG/PNG/WebP only — the client's `Content-Type` is ignored) and
  hands the core a `domain.File`. The upload adapter escapes the client filename with
  `mime.FormatMediaType` because it is copied into a header sent upstream. Never call
  `c.FormFile` directly in a handler.
- CORS lives in `main.go` on purpose even though KrakenD also sets it; leave it unless
  asked. Rate limiting on `/auth/login` and `/auth/register` is per-client-IP and
  in-process — it resets on restart and is not shared across replicas. The key comes from
  `middleware.TrustedProxies.ClientIP`: when the peer is in `TRUSTED_PROXIES` it walks
  `X-Forwarded-For` **from the right** and takes the first untrusted hop. Never switch it
  to `fiber.Config.ProxyHeader` / `c.IP()` — Fiber returns the *leftmost* entry, which
  the client controls, so one client could rotate through unlimited buckets.

## Testing

Tests are table-driven-ish, `testify` (`assert`/`require`), and fully mocked:

- `internal/core/services/mocks_test.go` — hand-written function-field mocks for every
  repository and driven port. Set only the `…Fn` fields a test needs.
- `internal/adapters/driving/http/handlers/handlers_test.go` — mocks the *services* and
  drives real Fiber apps via `app.Test(httptest.NewRequest(...))`. Use the `injectCaller`
  helper to put a `*domain.Actor` in `c.Locals("user")`; handlers that read the caller
  return 401 without it.

When changing a port signature, update the corresponding mock in both files.

Two guard-rail suites run with `go test ./...`:

- `handlers/golden_test.go` pins every endpoint's exact JSON response in
  `handlers/testdata/golden/`. If a change to a response is *intended*, regenerate with
  `go test ./internal/adapters/driving/http/handlers/ -run TestGolden -update` and review
  the diff. Otherwise a failure there means a client-visible regression.
- `internal/arch_test.go` enforces the hexagonal dependency rules. Its allowlists are a
  ratchet for `docs/refactor/hexagonal-plan.md`: entries may only be removed, and a stale
  entry fails the test. The domain vocabulary is in `GLOSSARY.md` and the decisions are in
  `docs/adr/`.

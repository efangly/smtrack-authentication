# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this is

`smtrack-authentication` — the authentication/authorization microservice for the SMtrack
platform. Go 1.25, Fiber v3, GORM/PostgreSQL, Redis, RabbitMQ. It issues JWTs and owns the
`Users`, `Wards` and `Hospitals` tables.

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
(copy `.env.example`). RabbitMQ and the upload service are optional — the adapters
degrade to no-ops with a warning when unconfigured.

CI (`.github/workflows/main.yaml`) only builds and pushes the Docker image on `main`,
then rewrites the image tag in `k8s/deploy.yaml`. **CI does not run the tests** — run
them locally.

## Architecture

Hexagonal (ports & adapters). The layer map is in `README.md`; what matters when editing:

- **`internal/core/`** must not import anything from `internal/adapters/` or from Fiber.
  Business rules live in `core/services/`, contracts in `core/ports/`.
- **`core/ports/`** defines both driving ports (`services.go` — what the HTTP layer calls)
  and driven ports (`repositories.go`, `cache.go`, `upload.go`, `messaging.go` — what the
  services call). Adapters implement these; nothing else may be depended on directly.
- **`internal/adapters/driving/http/`** is the only place that knows about Fiber.
  Handlers translate HTTP ↔ DTO ↔ domain and enforce request-level authorization.
- Wiring happens exclusively in `main.go`. Constructors return port interfaces, not structs.

Adding a capability normally means: port method → service implementation → repository
method → handler → route in `router.go`.

## Conventions

- **Database columns are camelCase and quoted.** The schema comes from a Prisma-era
  service, so raw GORM conditions must quote identifiers: `Where("\"hosId\" = ?", id)`.
  Timestamp columns are `createAt` / `updateAt` (no "d") — partial-update maps must use
  those exact keys.
- **Partial updates** flow as `map[string]any` built by a DTO `ToMap()` method, never as
  a struct, so that unset fields are not zeroed.
- **Cache keys** are prefixes: `user`, `user:<id>`, `user:<hosId>`, `ward…`, `hospital…`.
  `CachePort.Del` deletes by *prefix* (SCAN-based), so `Del(ctx, "user")` clears every
  user key. Anything that changes a ward or hospital must also invalidate `user`, because
  user listings preload their ward.
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
  `userService.Update`. `authService.Register` passes the plain password straight through to
  `Create` — it does not hash its own copy. Any map reaching `userService.Update` with a
  `password` key holds the *plain* value and gets hashed there. `UpdateUserRequest`
  deliberately has no password field — password changes go through `PATCH /auth/reset/:username`.
- **`/auth/user/:id` routes are ownership-checked in the handler**, not just by role:
  a non-admin-tier caller may only touch its own record, and `role`/`status`/`wardId` are
  stripped from its update. Use `ports.JwtPayload.IsAdminTier()` for that test —
  admin tier is SUPER, SERVICE, ADMIN, LEGACY_ADMIN.
- **Tenant scope is enforced in the services.** ADMIN/LEGACY_ADMIN may only read, change
  or place users in their own hospital (`canManageUsersIn` in `core/services/scope.go`);
  `FindOne` for hospitals and wards mirrors the matching `build*Conditions` listing
  (`canSeeHospital`, `canSeeWard`). Out-of-scope records read as *not found*. Every
  caller-facing `FindOne` takes the caller; the unscoped `UserService.FindByID` /
  `FindByUsername` exist only for the auth flows. `/auth/audit` is SUPER/SERVICE only
  because logs are not tenant-scoped.
- **Role ceiling is enforced in `userService`, not the handler.** `domain.Role.CanAssign`
  decides which roles a caller may assign *and* which existing users it may update or
  delete: SUPER → any; SERVICE → any but SUPER; ADMIN/LEGACY_ADMIN → USER, LEGACY_USER,
  GUEST. Nobody may change their own role. A nil caller may only create a GUEST (public
  register) and may not update or remove. Authorization reads the target from the
  repository, never the cache.
- **Every mutating route carries a `RolesGuard`.** `PUT`/`DELETE /auth/ward/:id` once had
  none, so any token — including a self-registered GUEST — could rename or delete any ward.
  `internal/adapters/driving/http/router_test.go` drives the real route table; extend it
  when adding a guarded route.
- **Refresh rebuilds claims from the database** (`authService.RefreshTokens`), never by
  copying the old token's claims, so a demoted or deactivated user loses access at the next
  refresh instead of keeping it for the token's remaining lifetime.
- CORS lives in `main.go` on purpose even though KrakenD also sets it; leave it unless
  asked. Rate limiting on `/auth/login` and `/auth/register` is per-IP and in-process —
  it resets on restart and is not shared across replicas.

## Testing

Tests are table-driven-ish, `testify` (`assert`/`require`), and fully mocked:

- `internal/core/services/mocks_test.go` — hand-written function-field mocks for every
  repository and driven port. Set only the `…Fn` fields a test needs.
- `internal/adapters/driving/http/handlers/handlers_test.go` — mocks the *services* and
  drives real Fiber apps via `app.Test(httptest.NewRequest(...))`. Use the `injectCaller`
  helper to put a `*ports.JwtPayload` in `c.Locals("user")`; handlers that read the caller
  return 401 without it.

When changing a port signature, update the corresponding mock in both files.

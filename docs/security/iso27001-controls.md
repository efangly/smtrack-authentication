# ISO/IEC 27001:2022 Annex A — technical controls of the auth service

This maps the **technical** controls this service implements to the Annex A (2022) controls
they support, with where to find the code and the evidence. It is an engineering aid for the
people who own the organisation's ISMS. **It is not a statement of compliance or
certification**: ISO/IEC 27001 certifies a management system (policies, risk assessment,
roles, audits), which no code can provide. Items that belong to infrastructure or to the
organisation are listed explicitly at the end.

## Implemented in the service

| Annex A | Control | What the service does | Code | Evidence |
|---|---|---|---|---|
| 5.15, 8.3 | Access control, information access restriction | Role ceiling (who may assign or touch which role); tenant scope (admins confined to their tenant, out-of-scope records read as *not found*); non-admin roles see only their own tenant and linked departments; hidden tenants visible to SUPER only | `core/services/scope.go`, `user_service.go`, `domain/enums.go` | `scope_test.go`, `user_service_test.go`, `router_test.go` (every mutating route × every role) |
| 5.16 | Identity management | Unique usernames across the system; a username stays reserved after soft delete so it cannot be reassigned to someone else | `users.username UNIQUE`, `UserRepository.UsernameTaken` | `pg_integration_test.go` |
| 5.17, 8.5 | Authentication information, secure authentication | bcrypt hashes (cost from `BCRYPT_COST`, minimum 12); constant-work login (unknown user, wrong password and disabled account cost the same and read the same); lockout after 5 failures per 15 minutes per username; separate access and refresh secrets with a `typ` claim; refresh claims rebuilt from the database | `adapters/driven/security`, `auth_service.go` | `auth_security_test.go`, `jwt_test.go`; smoke test recorded in the session notes |
| 5.18 | Access rights | `GET /auth/report/access` lists role, status, tenant, departments and last login for the caller's scope, read from the database (not cache), and the call is itself audited. Use it for the periodic access review | `user_service.go: AccessReport` | `access_report_test.go` |
| 8.2 | Privileged access | SUPER-only actions (delete tenant, hidden tenants); nobody can change their own role; SERVICE cannot create or touch SUPER | `domain.Role.CanAssign`, router guards | `router_test.go` |
| 8.10 | Information deletion | Soft delete with restore, then permanent removal (including pictures) by `cmd/purge` after `SOFT_DELETE_RETENTION` (default 365 days); each purge is audited. Audit rows themselves are never purged | `internal/purge`, `cmd/purge`, `k8s/purge-cronjob.yaml` | `internal/purge/integration_test.go` |
| 8.15 | Logging | Audit trail of create, update, delete, restore, purge, login success/failure (with reason, including "locked"), logout, token refresh, password reset, access denied and access report, each with actor, tenant, IP (trusted-proxy rules, not the spoofable leftmost `X-Forwarded-For`), user agent and request id. `audit_logs` is append-only: a database trigger rejects UPDATE, DELETE and TRUNCATE. No password or hash ever reaches an audit row | `audit_service.go`, `middleware/request_meta.go`, `migrations/00002_audit_logs.sql` | `pg_integration_test.go` (trigger), `audit_changes` tests |
| 8.16 | Monitoring activities | Failed logins, lockouts and denials are recorded as queryable events (`GET /auth/audit`, SUPER/SERVICE). Request logs go to Loki as structured JSON | `request_logger.go` | smoke test |
| 8.24 | Use of cryptography | Startup refuses JWT secrets shorter than 32 characters or identical access/refresh secrets, and bcrypt cost below 12; secrets come from a Kubernetes `Secret`, not a ConfigMap | `config/config.go: Validate`, `k8s/deploy-v2.yaml` | `config_test.go` |
| 8.25, 8.28 | Secure development, secure coding | Architecture tests keep infrastructure out of the core; domain errors never leak SQL or driver detail; uploads are size-capped and type-sniffed; request bodies validated declaratively | `internal/arch_test.go`, `handlers/errors`, `handlers/formFile` | `go test ./...` |
| 8.8 | Technical vulnerability management | Every pull request runs `govulncheck` (fails on reachable known vulnerabilities) and a Trivy scan of the image (fails on CRITICAL/HIGH with a fix available) | `.github/workflows/pr.yaml` | CI run history |
| 8.9, 8.32 | Configuration and change management | Schema changes only by numbered migrations applied under an advisory lock; PR checks (vet, gofmt, tests incl. integration against real Postgres and Redis) must pass before merge | `migrations/`, `database/migrate.go`, `pr.yaml` | CI run history |
| 8.12 (partial) | Data leakage prevention | Response DTOs are separate from domain types so a password hash cannot be serialised by accident; the cache never holds a hash | `dto/mapping.go`, `domain.User.WithoutSecrets` | `cache_test.go`, golden tests |

## Operating constraints the code cannot enforce

These must be true in the deployment for the table above to hold.

- **The service's database user must not own `audit_logs`** (or be a superuser). A table owner
  can drop the append-only trigger. Create the tables with a migration/owner role and give the
  runtime user only what it needs. Today the service runs migrations on startup, so either run
  the migration step with an owner role, or accept that the runtime role can alter the trigger
  and compensate with database-level audit (pgaudit) — decide and record it.
- **Audit retention:** keep `audit_logs` for at least 12 months (policy; nothing deletes
  them). Back them up with the rest of the database.
- **Transport security:** TLS terminates at the gateway/ingress; use `sslmode=require` (or
  stronger) in `DATABASE_URL`, and protect Redis and RabbitMQ the same way.
- **Redis availability:** lockout and revocation live in Redis. If it is down, logins are still
  allowed (logged) but refresh is refused. Run Redis with persistence and monitoring if
  revocations must survive a restart; a restart loses the revoked-token list for tokens still
  within their 7-day life.
- **Secrets:** create `authentication-v2-secrets` out of band (`k8s/secrets.example.yaml` is a
  template), restrict who can read it, and rotate on a schedule. Changing `JWT_*` secrets signs
  everybody out.
- **Rate limiting** on `/auth/login` and `/auth/register` is per client IP and per replica
  (in memory). The per-username lockout is shared across replicas.

## Known gaps

| Annex A | Gap | Note |
|---|---|---|
| 5.17, 8.5 | **No MFA** | The design allows adding it later (a second step after `ValidateUser`); not implemented |
| 5.17 | **No password-strength policy** | Passwords stay 4–20 characters, as before the migration, so imported users are not affected. Add the rule in `userService.Create` / `Update` (not at login) when the organisation decides to |
| 8.5 | Access token is not revocable | Access tokens last 15 minutes and are not checked against Redis, so logout, deactivation or a password change takes full effect within 15 minutes (refresh is refused immediately) |
| 8.15 | Self-registration audit row has no tenant | The `CREATE` row written for a public registration has an empty `tenant_id` (no authenticated actor) |
| 8.16 | No alerting | Events are recorded and queryable; alert rules (many `LOGIN_FAILURE`, `ACCESS_DENIED` bursts) belong in Loki/Grafana and are not defined here |
| 8.13 | Backup | Not handled by the service |
| 5.18 | Access review is a report, not a workflow | Someone must run it on a schedule and keep the sign-off |

## Outside the service (organisation / infrastructure)

Information security policy and roles (5.1–5.4), risk assessment and treatment, supplier and
cloud management (5.19–5.23), incident management (5.24–5.28), business continuity (5.29–5.30),
HR and awareness (6.x), physical security (7.x), backup and restore testing (8.13), network
segmentation (8.20–8.22), time synchronisation (8.17), and the internal audit and management
review that make up the management system itself.

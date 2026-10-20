# แผนปรับโครงสร้าง smtrack-authentication ให้เป็น Hexagonal อย่างสมบูรณ์ + ปิดช่องโหว่

## Context

โปรเจคนี้ **เป็น hexagonal อยู่แล้วในเชิงโครงสร้าง** (`internal/core/{domain,ports,services}`, `internal/adapters/{driven,driving}`, wiring ใน `main.go`)
แต่ยังมี infrastructure รั่วเข้า core 7 จุด (L1–L7) และจาก security review พบช่องโหว่ที่ใช้งานได้จริงใน production (C1, H1–H3, M1–M4, L1–L4)

เป้าหมาย:
1. ปิดช่องโหว่ร้ายแรงก่อน (Phase S) — deploy ได้ทันที
2. อุดรอยรั่ว hexagonal ทีละ phase โดย **ไม่เปลี่ยนพฤติกรรม API** (ยกเว้นที่ระบุว่าเป็น security fix) และ **คง layout แบบ per-layer เดิม**
3. ปิดท้ายด้วย architecture test ที่บังคับกฎ dependency อัตโนมัติ

ข้อตกลงที่ใช้ทุก phase:
- 1 phase = 1 PR (Phase S แยกย่อยได้หลาย PR) ตั้งชื่อ branch `refactor/phase-N-<slug>` / `fix/sec-<id>`
- ทุก PR ต้องผ่าน: `go build ./...` · `go vet ./...` · `gofmt -l .` (ว่าง) · `go test ./...` · e2e ใน `test/e2e`
- เปลี่ยน port signature → อัปเดต mock ทั้ง `internal/core/services/mocks_test.go` และ `internal/adapters/driving/http/handlers/handlers_test.go`
- คง error discipline: domain error ไม่ใช้ `%w`, infra error wrap `%w` ที่ adapter
- ของที่ **ตั้งใจคงไว้ใน core**: `pkg/logger`, `uuid.New()`, `time.Now()`
- คัดลอกแผนนี้ไปไว้ที่ `docs/refactor/hexagonal-plan.md` ใน PR แรก แล้วติ๊ก checkbox ตามความคืบหน้า

---

## Phase S — Security hotfix (ทำก่อนทุกอย่าง, deploy ทันที)

### S1 — ward update/delete ไม่มีการตรวจสิทธิ์ (C1, Critical)
- [x] `router.go`: ใส่ `RolesGuard(RoleSuper, RoleService)` ให้ `PUT /ward/:id` และ `DELETE /ward/:id`
- [x] test: `internal/adapters/driving/http/router_test.go` (7 role × PUT/DELETE) + e2e `test/e2e/ward_guard_test.go`

### S2 — Role ceiling: กันการยกระดับสิทธิ์ (H1, High)
- [x] เพิ่ม `domain.Role` rank/กฎ `CanAssign` (ชั่วคราวเป็น method บน `ports.JwtPayload`, ย้ายไป `domain.Actor` ใน Phase 1)
  - SUPER → ได้ทุก role · SERVICE → ทุก role ยกเว้น SUPER · ADMIN/LEGACY_ADMIN → USER, LEGACY_USER, GUEST
  - ห้ามเปลี่ยน role ของตัวเอง
- [x] enforce ใน `userService.Create` และ `userService.Update` (เมื่อมี key `role`) — คืน domain error (ไม่ wrap)
- [x] admin แก้/ลบ user ได้เฉพาะเมื่อ role **ปัจจุบัน** ของเป้าหมายอยู่ในชุดที่ตัวเอง assign ได้
- [x] unit test ทุกคู่ (caller role × target role) ใน `user_service_test.go`

### S3 — Tenant scope ของ admin (H2 + M2, High)
- [x] `userService.FindOne/Update/Remove`: ADMIN/LEGACY_ADMIN ได้เฉพาะ user ที่ `Ward.HosID == caller.HosID`; ย้าย `wardId` ได้เฉพาะ ward ใน hospital เดียวกัน (ต้องฉีด `WardRepository` เข้า `userService`)
- [x] `hospitalService.FindOne`, `wardService.FindOne`: ใช้ scope เดียวกับ `buildHospitalConditions` / `buildWardConditions`
- [x] `router.go`: `/auth/audit` → `RolesGuard(RoleSuper, RoleService)`
- [x] test ข้ามโรงพยาบาล → 403/404 ทุก endpoint ที่แตะ

### S4 — บัญชีที่ถูกปิดยัง login ได้ (H3, High)
- [ ] `authService.ValidateUser`: ถ้า `!user.Status` → `nil, nil` (handler ตอบ "Invalid username or password" เหมือนรหัสผิด) โดยยังทำ bcrypt compare ให้เวลาเท่าเดิม
- [ ] test ใน `auth_service_test.go` + e2e `auth_login_test.go`

### S5 — Rate limit หลัง KrakenD (M1, Medium)
- [ ] `config.Config` เพิ่ม `TrustedProxies []string` จาก env `TRUSTED_PROXIES` (comma-separated CIDR)
- [ ] `main.go` `fiber.Config`: ถ้ามีค่า → `TrustProxy: true`, `TrustProxyConfig{Proxies: ...}`, `ProxyHeader: fiber.HeaderXForwardedFor`; ถ้าไม่มี → พฤติกรรมเดิม
- [ ] เพิ่มใน `.env.example`, `k8s/deploy.yaml`
- [ ] test: request จาก trusted proxy ที่ X-Forwarded-For ต่างกันได้ bucket แยกกัน

---

## Phase 0 — Safety net + เอกสารการตัดสินใจ

- [ ] Golden JSON snapshot test ของ response ทุก endpoint (success + error) — `test/e2e/golden/` + flag `-update`
- [ ] `internal/arch_test.go`: ใช้ `go list -deps -json ./internal/core/...` แล้ว fail ถ้า core import `internal/adapters`, `gofiber`, `gorm.io`, `golang-jwt`, `x/crypto/bcrypt`, `smtrack-authentication/config`, `mime/multipart`
  - [ ] มี **allowlist** การละเมิดปัจจุบัน (ratchet) — แต่ละ phase ต้องลบรายการของตัวเองออก
- [ ] `GLOSSARY.md` (ตาม format ของ skill domain-modeling): **Actor**, **Admin tier**, **Role ceiling**, **Tenant scope** (hospital), **Hospital**, **Ward**, **Guest**
- [ ] `docs/adr/0001-domain-free-of-infrastructure-tags.md` — domain ไม่มีแท็ก gorm/json; persistence model + mapper อยู่ใน adapter; response อยู่ใน DTO
- [ ] `docs/adr/0002-cache-aside-in-services.md` — cache-aside และกฎ cascade invalidation อยู่ใน service (ไม่ใช่ decorator) เพราะกฎ cascade ข้าม entity เป็น business knowledge; codec อยู่ใน adapter

## Phase 1 — `JwtPayload` → `domain.Actor` (L6)

- [ ] สร้าง `internal/core/domain/actor.go`: `type Actor struct{ ID, Name string; Role Role; HosID, WardID string }` + `IsAdminTier()`, `CanAssign(Role)` (ย้ายจาก S2)
- [ ] ลบ `ports.JwtPayload`; แทนที่ทุก `caller *ports.JwtPayload` ด้วย `*domain.Actor` ใน `ports/services.go`, services, handlers (`getCaller`), middleware (`auth.go`, `roles.go`), mocks, `injectCaller`
- [ ] switch แบบ string literal ใน `build*Conditions` (`case "ADMIN"`) → ใช้ค่าคงที่ `domain.Role…`
- [ ] อัปเดต CLAUDE.md ส่วนที่อ้าง `ports.JwtPayload.IsAdminTier()`

## Phase 2 — Security ports: token + password hasher (L2, L3, L2-sec, L4-sec)

- [ ] `ports/security.go`:
  - `TokenIssuer`: `IssuePair(ctx, domain.Actor) (domain.TokenPair, error)`, `VerifyAccess(token string) (*domain.Actor, error)`, `VerifyRefresh(token string) (userID string, error)`
  - `PasswordHasher`: `Hash(plain string) (string, error)`, `Compare(hash, plain string) bool`
- [ ] `internal/adapters/driven/security/jwt.go` — สร้างจาก secret + TTL (ย้าย `parseDuration` มาด้วย); เพิ่ม claim `typ: access|refresh` และตรวจตอน verify (ปิด L2)
- [ ] `internal/adapters/driven/security/bcrypt.go` — cost 10, dummy hash สำหรับ timing (ย้าย `dummyHash` มา, เพิ่ม `CompareDummy(plain)` หรือให้ `Compare("", plain)` ทำ dummy)
- [ ] `authService` รับ `TokenIssuer`, `PasswordHasher` แทน `*config.Config`; ลบการเช็ค secret ว่างที่ตายแล้ว (`config.Load` fatal ให้แล้ว)
- [ ] `userService` รับ `PasswordHasher`; hash ยังเกิดที่ `Create`/`Update` เท่านั้น (invariant เดิม)
- [ ] `LoginResult`/`RefreshResult` → type ใน core ไม่มีแท็ก; รูปร่าง JSON ย้ายไป `dto/auth.go` (`LoginResponse`, `RefreshResponse`)
- [ ] `middleware.JWTAuth` / `RefreshJWTAuth` รับ `ports.TokenIssuer` แทน secret string; `router.go` ไม่ต้องรู้ secret
- [ ] invalid refresh token → domain error (ไม่ wrap) → 401 (ปิด L4)
- [ ] ลบ allowlist `golang-jwt`, `bcrypt`, `config` ออกจาก arch test

## Phase 3 — `domain.File` แทน multipart (L4, M4)

- [ ] `domain/file.go`: `type File struct{ Name, ContentType string; Size int64; Content io.Reader }`
- [ ] `FileUploadPort.Upload(ctx, domain.File, path string)`; service ports ที่รับไฟล์ใช้ `*domain.File`
- [ ] helper ใน handlers: `formFile(c, "image") (*domain.File, error)` — เปิดไฟล์, ตรวจ **ขนาด ≤ 5MB** และ **MIME เป็น image/jpeg|png|webp** (sniff จาก bytes) → 400 ถ้าไม่ผ่าน (ปิด M4)
- [ ] upload adapter: escape filename ด้วย `mime.FormatMediaType` / ใช้ชื่อที่ generate เอง (ปิด header injection)
- [ ] ลบ allowlist `mime/multipart`

## Phase 4 — Filter และ Patch แบบมี type (L5)

- [ ] `ports`: `UserFilter{HosID string; ExcludeHosIDs []string; ExcludeRoles []domain.Role}`, `WardFilter`, `HospitalFilter{ID, WardID string; ExcludeIDs []string}` (สไตล์เดียวกับ `AuditFilter`)
- [ ] `build*Conditions` คืน filter struct + cache key; repository แปลงเป็น `Where` แบบ quoted เอง
- [ ] `domain.UserPatch`, `WardPatch`, `HospitalPatch` — field เป็น pointer; `Password *string` มีเฉพาะใน `UserPatch` (ใช้ภายในจาก ResetPassword)
- [ ] DTO `ToMap()` → `ToPatch()`; handler strip `Role/Status/WardID` โดย set เป็น nil
- [ ] repository แปลง patch → `map[string]any` ด้วยคีย์คอลัมน์จริง รวม `"updateAt"`
- [ ] `sanitizeAuditChanges` → audit บันทึก patch ที่ `Password` ถูกตัดออกแบบ type-safe
- [ ] เอา `pic` ออกจาก `CreateUserRequest`/`UpdateUserRequest` และ `hosPic` ออกจาก DTO hospital — ตั้งได้ผ่าน upload เท่านั้น (ปิด M3)
- [ ] อัปเดต CLAUDE.md หัวข้อ "Partial updates"

## Phase 5 — Cache codec + ไม่เก็บ secret ลง cache (L7)

- [ ] `CachePort`: `Get(ctx, key string, dest any) (hit bool, err error)`, `Set(ctx, key string, v any, ttl time.Duration) error`, `Del(ctx, prefix string) error`
- [ ] Redis adapter เป็นฝ่าย JSON encode/decode
- [ ] `domain.User.WithoutSecrets() User` — เรียกก่อน `Set` ทุกครั้ง; unit test ยืนยันว่า payload ที่ส่งเข้า `Set` ไม่มี hash
- [ ] ลบ `encoding/json` ออกจาก services (ยกเว้น audit ถ้ายังต้อง marshal changes — พิจารณาให้ `AuditRepository` รับ `any` แล้ว adapter marshal)
- [ ] TTL เป็นค่าคงที่ `time.Duration` ที่มีชื่อ (`listTTL = 10*time.Hour`, `itemTTL = 24*time.Hour`)

## Phase 6 — Persistence model + response DTO; ถอดแท็กออกจาก domain (L1)

- [ ] `internal/adapters/driven/database/models.go`: `userModel`, `wardModel`, `hospitalModel`, `auditLogModel` พร้อมแท็ก `gorm` และ `TableName()` เดิม (`"Users"`, `"Wards"`, `"Hospitals"`, …)
- [ ] mapper `toDomain()` / `fromDomain()` ต่อ model; repository ใช้ model ภายในเท่านั้น
- [ ] `dto/`: `UserResponse`, `WardResponse`, `HospitalResponse`, `AuditLogResponse` + `From…()` — handler ไม่ serialize domain ตรงอีกต่อไป (`UserResponse` ไม่มี password)
- [ ] ถอดแท็ก `gorm:` และ `json:` ทั้งหมดออกจาก `internal/core/domain/*.go`
- [ ] golden snapshot จาก Phase 0 ต้องผ่าน **โดยไม่ต้อง `-update`**
- [ ] ตรวจ AutoMigrate (ถ้ามีใน `postgres.go`) ใช้ model ใหม่และไม่สร้างการเปลี่ยน schema

## Phase 7 — ปิดงาน

- [ ] allowlist ใน `internal/arch_test.go` ต้องว่าง และเพิ่มกฎ: `domain` ห้าม import `ports`/`services`; `ports` ห้าม import `services`
- [ ] เพิ่ม `go test ./...` เข้า `.github/workflows/main.yaml` ก่อนขั้น build image (ปัจจุบัน CI ไม่รันเทสต์)
- [ ] อัปเดต `CLAUDE.md` (Architecture, Conventions, Security invariants: role ceiling, tenant scope, ward guard, disabled-login), `README.md` (โครงสร้างโฟลเดอร์ + ตาราง role ของ ward/audit)
- [ ] ทบทวน `GLOSSARY.md` และ ADR ว่ายังตรงกับโค้ด
- [ ] L3 (ทางเลือก): ResetPassword ตอบ "user not found" เฉพาะ admin tier, คนอื่นได้ 403 ข้อความเดียวกัน

---

## นอกขอบเขต (ตัดสินใจเลื่อนไว้)

- L1 password policy (เดิม 4–20 ตัวอักษร) — **ยังไม่แก้ในรอบนี้** ตามที่ตกลง; คงไว้ในรายการ security backlog

## ไฟล์หลักที่ถูกแก้

- `internal/core/ports/{services,repositories,cache,upload}.go` + `ports/security.go` (ใหม่)
- `internal/core/domain/*.go` + `actor.go`, `file.go`, patch types (ใหม่)
- `internal/core/services/*_service.go`, `mocks_test.go`
- `internal/adapters/driven/{database,cache,upload}/*.go` + `security/` (ใหม่)
- `internal/adapters/driving/http/{router.go,handlers/*,middleware/{auth,roles}.go,dto/*}`
- `main.go`, `config/config.go`, `k8s/deploy.yaml`, `.env.example`, `.github/workflows/main.yaml`
- `CLAUDE.md`, `README.md`, `GLOSSARY.md`, `docs/adr/*` (ใหม่)

## Verification

แต่ละ phase:
```bash
go build ./... && go vet ./... && test -z "$(gofmt -l .)" && go test ./...
go test ./test/e2e/...            # e2e + golden snapshots
go test ./internal -run TestArch  # architecture rules (ตั้งแต่ Phase 0)
```
Phase S เพิ่มเติม — รันเครื่อง local (Postgres + Redis + `.env`) แล้วยิงด้วย curl:
- register GUEST → login → `PUT /auth/ward/<id>` ต้องได้ 403
- ADMIN → `PUT /auth/user/<self>` body `{"role":"SUPER"}` ต้องได้ 400/403
- ADMIN รพ. A → `GET /auth/user/<user รพ. B>` ต้องได้ 403/404
- ตั้ง `status=false` → login ต้องได้ 401 "Invalid username or password"
- ยิง login 11 ครั้งด้วย `X-Forwarded-For` ต่างกันจาก trusted proxy → ไม่โดน 429

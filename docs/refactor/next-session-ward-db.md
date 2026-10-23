# ส่งต่อ session ถัดไป: ปรับโครงสร้าง DB ของ ward + Phase 6

เอกสารนี้สรุปสิ่งที่ session หน้าต้องรู้ก่อนเริ่มวางแผน ไม่ต้องสืบบริบทใหม่
แผนหลักและสถานะของแต่ละ phase อยู่ที่ [`hexagonal-plan.md`](hexagonal-plan.md)

## จุดเริ่มต้น

- โค้ดล่าสุดอยู่ที่ branch `refactor/phase-7-wrap-up` (stacked ต่อจาก Phase S → 0 → … → 5 → 7) ทุก branch push แล้ว **แต่ยังไม่ได้เปิด PR**
- `go test ./...` ครอบ unit, golden response (`handlers/testdata/golden/`) และ architecture rules (`internal/arch_test.go`) ส่วน e2e ยังไม่เคยรันกับ DB จริงสำหรับการแก้ทั้งหมดในรอบนี้
- Phase 6 คือการถอดแท็ก `gorm`/`json` ออกจาก domain (ADR 0001) **ยังไม่ได้ทำ** โดยตั้งใจ เพราะ persistence model ผูกกับ schema ถ้าทำตอนนี้แล้วเปลี่ยน ward ทีหลังจะต้องเขียนซ้ำ

## ความสัมพันธ์ ward ↔ user ↔ hospital ในปัจจุบัน

- `Users.wardId` **ห้ามว่าง** (`domain/user.go:8`, `CreateUserRequest.WardID` เป็น `required`) user มีได้ ward เดียว
- user ไม่มี `hosId` ของตัวเอง **hospital ของ user มาจาก `Wards.hosId` ทางเดียว**
- ward อยู่ได้ใน hospital เดียว (`Wards.hosId`)

### โค้ดที่พึ่งความสัมพันธ์นี้ (ต้องแก้ตามถ้า schema เปลี่ยน)

| เรื่อง | ที่ | พึ่งอะไร |
|---|---|---|
| Tenant scope ของ user | `core/services/scope.go:15` `canManageUsersIn`, `:28` `userHosID` | hospital ของ user = `user.Ward.HosID` |
| ห้ามย้าย user ข้ามโรงพยาบาล | `core/services/user_service.go:287` `checkWardInScope` | lookup ward → `HosID` |
| Scope ของ hospital/ward | `scope.go:37` `canSeeHospital` (เช็คว่ามี `caller.WardID` อยู่ใน `hospital.Wards`), `:59` `canSeeWard` | ward ของผู้เรียก |
| Claim ใน token | `core/services/auth_service.go:167` `actorFor` → `security/jwt.go:66-67` | `hosId` คำนวณจาก ward ตอน login/refresh |
| รายการ user ตามโรงพยาบาล | `database/user_repository.go:39` `JOIN "Wards"` + `UserFilter` (`services/user_service.go:303`) | join ผ่าน ward |
| รายการ hospital ของ USER/GUEST | `database/hospital_repository.go:41` `EXISTS … "Wards".id = ?` + `buildHospitalFilter` (`hospital_service.go:182`) | ward ของผู้เรียก |
| Cache cascade | ADR 0002; `ward_service.go`/`hospital_service.go` ล้าง prefix `user` | user listing ฝัง ward/hospital ไว้ |
| Event ไป service อื่น | `ward_service.go:109,113` `update-ward`; `hospital_service.go:134,137` `update-hospital` (RabbitMQ → device/legacy) | payload `{id, name}` |
| Golden response | `handlers/testdata/golden/user_*.json` | user มี object `ward` ซ้อน `hospital` |

## ตัดสินใจแล้ว (Q24–Q27 ปิด)

ดู [ADR 0003](../adr/0003-tenant-department-model.md) (Tenant/Department แทน Hospital/Ward),
[ADR 0004](../adr/0004-parallel-run-separate-database.md) (รันคู่กับเวอร์ชันเดิมบน DB แยก, migration
ด้วย `goose`, รวม Phase 6) และ [`GLOSSARY.md`](../../GLOSSARY.md) แผนลงมือทำอยู่ที่
`~/.claude/plans/db-user-purring-hickey.md` ส่วนหัวข้อด้านล่างเก็บไว้เป็นบริบทเดิม

## คำถามเดิม (ปิดแล้ว)

1. **(Q24) จะเปลี่ยนอะไรกับ ward?** (a) user อยู่ได้หลาย ward (b) user ผูกกับ hospital ตรงๆ และ ward เป็นตัวเลือก (c) ward มีลำดับชั้น (d) เปลี่ยนแค่ field/ชนิด (e) อื่นๆ คำตอบนี้กำหนดว่าตารางข้างบนต้องแก้กี่แถว
2. **(Q25) migration ใครเป็นเจ้าของ?** คำแนะนำ: ย้ายมาไว้ใน repo นี้เป็นไฟล์ SQL ที่มี version (`golang-migrate` หรือ `goose`) แล้วรันตอน deploy ตอนนี้ AutoMigrate มีแค่ `AuditLogs` (`database/postgres.go`)
3. **(Q26) มี service อื่นอ่านตาราง `Wards`/`Users` ตรงจาก DB ไหม?** ถ้ามี ต้องทำแบบ expand/contract (เพิ่มของใหม่ → ย้ายข้อมูล → ลบของเก่า) และอาจต้องเปลี่ยน payload ของ event
4. **(Q27) จะรวม Phase 6 เข้ากับการปรับ DB หรือไม่?** คำแนะนำ: รวม เพื่อให้เขียน persistence model ตาม schema ใหม่แค่ครั้งเดียว
5. ถ้า user อยู่ได้หลาย ward/hospital: claim `hosId` ใน token จะเป็นค่าเดียวต่อไปไม่ได้ ต้องตัดสินใจว่าจะใส่ทุกค่า เลือก "hospital ที่กำลังใช้งาน" หรือเลิกใส่ใน token

## Checklist ของ Phase 6 (ยกมาจากแผนหลัก)

- [ ] persistence model (`userModel`, `wardModel`, `hospitalModel`, `auditLogModel`) + mapper ใน `adapters/driven/database` **ตาม schema ใหม่**
- [ ] response DTO (`UserResponse`, `WardResponse`, `HospitalResponse`, `AuditLogResponse`) ให้ golden ผ่านโดยไม่ต้อง `-update` ถ้าตั้งใจเปลี่ยนรูปร่าง JSON ให้ update golden แล้ว review diff ร่วมกับทีม frontend
- [ ] ถอดแท็กออกจาก `internal/core/domain/*.go` แล้วลบรายการออกจาก `tagAllowlist` ใน `internal/arch_test.go` (ลบได้อย่างเดียว เพิ่มไม่ได้)
- [ ] **cache**: JSON ใน Redis จะเปลี่ยนรูป → เปลี่ยน prefix ของ cache key (เช่น `v2:user…`) หรือ flush ตอน deploy มิฉะนั้นค่าเก่าจะ decode ได้ไม่ครบ
- [ ] **audit**: snapshot ตอน CREATE/DELETE ต้องใช้ map/DTO ที่ใช้ชื่อ field แบบ API (เหมือน `services/audit_changes.go`) ไม่ marshal domain struct ตรงๆ
- [ ] ตรวจว่า AutoMigrate ของ `AuditLogs` ใช้ model ใหม่แล้วไม่เปลี่ยน schema

## Backlog (ไม่ขึ้นกับงาน ward)

| งาน | ทำไม | เมื่อไร |
|---|---|---|
| เลิกรับ token ที่ไม่มี `typ` | ตอนนี้ยอมรับไว้ชั่วคราว (`security/jwt.go` TODO) | หลัง deploy Phase 2 ไปแล้ว ≥ `REFRESH_EXPIRE_TIME` (7 วัน) |
| ตั้ง `TRUSTED_PROXIES` ใน configmap `authentication-config` | ถ้าไม่ตั้ง ทุก client หลัง KrakenD จะใช้โควต้า rate limit 10/นาทีร่วมกัน | พร้อม deploy S5 (ops) |
| ~~ย้าย `JWT_SECRET`, `JWT_REFRESH_SECRET`, `REDIS_PASSWORD` จาก ConfigMap ไปเป็น Secret~~ **ทำแล้วใน `k8s/deploy-v2.yaml`** (เวอร์ชันเก่า `k8s/deploy.yaml` ยังใช้ ConfigMap) | ต้องสร้าง Secret `authentication-v2-secrets` ใน cluster ก่อน (ดู `k8s/secrets.example.yaml`) | ops |
| L1 password policy (ตอนนี้ 4–20 ตัวอักษร) | อ่อนเกินไป | ผู้ใช้สั่งให้เลื่อนไว้ |
| L3 `PATCH /auth/reset/:username` ตอบ "user not found" ให้ทุกคน | ใช้ไล่หา username ได้ | งานเล็ก ทำได้ทุกเมื่อ |
| `buildWardFilter` ไม่จำกัด scope ของ USER/GUEST และรายการ hospital ของ guest ที่ไม่มี ward คืนทุกโรงพยาบาล | เห็นข้อมูลข้าม tenant (UI อาจพึ่งอยู่) | ตัดสินใจพร้อมงาน ward |
| ยืนยันบั๊ก SERVICE `GET /auth/user` → 500 กับ Postgres จริง | แก้แล้วใน Phase 4 แต่ยืนยันจาก DryRun SQL เท่านั้น | ตอนรัน e2e |
| รัน e2e (`go test -tags=e2e ./test/e2e/... -v`) | ยังไม่เคยรันกับการแก้รอบนี้ | ก่อน merge Phase S |
| CI รันบน PR | ตอนนี้รันเฉพาะ push เข้า `main` | แยก job ที่ไม่ push image |
| `FileUploadPort.Delete` รับ `ctx` | ให้สอดคล้องกับ `Upload` | งานเล็ก |
| เทสต์ของ Redis adapter | ไม่มี miniredis ใน repo | ถ้าเพิ่ม dependency ได้ |
| แจ้ง frontend | GIF/SVG ถูกปฏิเสธ, ส่ง `pic`/`hosPic` ใน body แล้วถูกเพิกเฉย, ADMIN ได้ 403/404 ใหม่ (ward, audit, ข้ามโรงพยาบาล, user ที่ role สูงกว่า) | ก่อน deploy |

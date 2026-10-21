---
status: accepted
---

# Domain types carry no persistence or transport shape

The domain structs in `internal/core/domain` used to carry `gorm:` and `json:` tags, so the
Prisma-era database schema and the public JSON contract were both encoded in the core and
changed whenever either one did. We decided the domain holds plain structs. Each adapter
owns its own shape and maps it at the boundary: persistence models with `gorm` tags live in
`adapters/driven/database`, and response DTOs live in `adapters/driving/http/dto`.

## Consequences

- Every entity needs a mapper in both directions. That is the deliberate cost. Do not
  "simplify" it by putting tags back on the domain.
- The JSON contract is pinned by the golden tests in
  `internal/adapters/driving/http/handlers/testdata/golden`, so moving serialization out of
  the domain cannot silently change what clients receive.
- `internal/arch_test.go` fails if a domain file gains a `gorm` or `json` tag.

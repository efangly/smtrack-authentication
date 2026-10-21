---
status: accepted
---

# Cache-aside and invalidation stay in the services

Reads are cached in Redis under prefix keys (`user…`, `ward…`, `hospital…`). Invalidation
cascades across entities: changing a ward or hospital must also clear every `user` key,
because user listings embed their ward and hospital. We decided that both the cache-aside
reads and these invalidation rules live in `core/services`. The services talk to a
`CachePort`, and only the encoding of values is left to the Redis adapter.

## Considered Options

- **Caching decorator around each repository** (in the adapter). Rejected because a
  decorator only knows its own entity. The cross-entity cascade would then have to live in
  infrastructure, where nobody reviewing a business change would think to look.

## Consequences

- Values must be stripped of secrets before they are cached. A user's password hash must
  never reach Redis.
- Authorization decisions read from the repository, never from the cache, so they cannot be
  made on a stale role or status.

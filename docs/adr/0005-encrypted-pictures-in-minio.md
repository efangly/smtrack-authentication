---
status: accepted
---

# Pictures are stored encrypted in MinIO and served by this service

Pictures moved from an external image API to MinIO. The service encrypts each image with
AES-256-GCM before upload, using one key from `IMAGE_ENCRYPTION_KEY`, and serves it decrypted
through `GET /auth/image/:path/:name`. `pic` keeps holding an absolute URL (now pointing at
that endpoint), so the services, the database and the response JSON did not change. The
endpoint needs no token, as the old image URLs did; the generated object name is the secret.

## Considered Options

- **MinIO server-side encryption (SSE-S3/KMS)**. Rejected: anyone with access to the bucket
  still reads plain images, and it needs a KMS set up on the MinIO side.
- **Presigned MinIO URLs in `pic`**. Rejected: they expire, and the stored bytes would be
  plain.
- **Require a token on the image endpoint**. Rejected for now: `<img>` tags cannot send one,
  and the old URLs were public.

## Consequences

- Losing `IMAGE_ENCRYPTION_KEY` loses every picture. The first byte of each object names the
  key version so a second key can be added later, but rotation is not built.
- Pictures from the old image service are not migrated. Their URLs stay in `pic`, still point
  at the old service, and are skipped when a picture is replaced or purged.
- Every picture view goes through this service instead of MinIO.

# Phase 1 Design: Auth + Paywall + Media Pipeline + Admin Users + Feed Filters

## Context

RedChef is a Go + SQLite content platform with an admin panel. The current codebase has:
- A flat `users` table (id, username, password_hash) without email, roles, or paid status
- Session-based auth via `admin_token` cookie, admin-only
- Cookie-based mock subscription (`royal_member`, `unlocked_N` cookies) — insecure, not user-scoped
- Raw uploads with no processing (no resize, no compression, no transcoding)
- No feed filtering capabilities
- Admin panel lacks user management

Phase 1 fundamentally reworks the auth model, adds proper paywall gating, optimizes media uploads, adds admin user management, and feed filters.

## Goals / Non-Goals

### Goals
- User registration with email + password; first user = admin
- Session-based auth for all users (admin + normal)
- Paywall gating: locked posts invisible to unpaid normal users
- Mock payment flow that sets paid=true
- Image resize + compression (Go imaging library, server-side)
- Video resize + transcoding (ffmpeg, server-side)
- Admin users view and paid-status management
- Feed sort, type, and date-range filters
- Front-end UI for login, register, filter controls

### Non-Goals
- Real payment integration (Stripe) — Phase 2
- Comments, favourites, tips, WhatsApp sharing, linked posts — Phase 2
- Password reset flow
- User profile editing
- Social features of any kind
- Role creation/management beyond admin/normal

## Decisions

### Decision: Session-based auth replaces cookie-based subscription
**Why**: The current `royal_member` cookie is client-side, easily forged, and not tied to a user identity. Switching to server-side session tokens (HttpOnly, SameSite=Lax) is the minimum viable security model.

**Approach**: Reuse the existing `sessions` table but extend to all users. Sessions last 30 days. Admin middleware checks both session validity and `role == "admin"`.

**Alternatives considered**: JWT — no advantage here; session tokens are simpler with SQLite and let us invalidate server-side.

### Decision: Image processing with `github.com/disintegration/imaging`
**Why**: Pure Go image processing library with no CGO dependency. Supports resize, JPEG/PNG/WebP output, quality control. Avoids needing ImageMagick or libvips.

**Approach**: After upload, process synchronously in the upload goroutine (async via goroutine). Resize to max 1920px width, save at 85% JPEG quality. For PNGs with transparency, preserve PNG format.

**Trade-off**: Processing synchronous in a goroutine means upload returns immediately but processed file replaces original after a brief delay. The `processing` field on the post signals this state.

### Decision: Video transcoding with ffmpeg command-line
**Why**: ffmpeg is the standard for server-side video processing. Go bindings exist but add complexity and API instability. Shelling out to ffmpeg is reliable and well-understood.

**Approach**: Alpine Docker image gets ffmpeg package. After upload, spawn `ffmpeg` as subprocess. Transcode to H.264/AAC/MP4. Max 1080p. Extract thumbnail at 2s mark. Async processing.

**Risk**: High CPU/memory on large videos. Mitigation: file size limit (50MB current), process one at a time, add timeout.

### Decision: Media processing runs async in a goroutine
**Why**: Large uploads (especially video) could take seconds to minutes. Blocking the upload endpoint would create a terrible UX. The post is created immediately, media processed in background.

**Approach**: `go processMedia(postID, tempPath)` after `db.CreatePost`. The `Processing` bool on the post model tracks state. Frontend can poll or the page can just wait for processed file.

**Risk**: If server crashes mid-processing, the post stays in `processing=true` state permanently. Mitigation: add a startup recovery check that re-processes or marks abandoned processing as failed.

### Decision: SQLite migration via ALTER TABLE + new tables
**Why**: Modern SQLite supports limited ALTER TABLE. We add columns to `users` and create a new model. No need for a full ORM.

**Schema changes**:
```sql
ALTER TABLE users ADD COLUMN email TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'normal';
ALTER TABLE users ADD COLUMN paid INTEGER DEFAULT 0;

CREATE TABLE IF NOT EXISTS user_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT UNIQUE NOT NULL,
    user_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
```

Existing sessions (admin_token) are handled separately — the admin user gets role=admin, existing sessions are reused via migration.

### Decision: Existing admin migration strategy
**Approach**: On startup, if any users exist in the old schema (username column exists), the first user gets role=admin, paid=true, and username is copied to email as `username@local`. The old sessions table continues working (admin_token cookies remain valid). New sessions go through `user_sessions`.

## Risks / Trade-offs

- **[Concurrent video processing]** If multiple videos upload simultaneously, ffmpeg processes could exhaust CPU. → Mitigation: simple semaphore limiting concurrent processing to 1-2 goroutines.
- **[Ffmpeg missing in Docker]** If the Alpine image lacks ffmpeg, video uploads fail silently. → Mitigation: fail fast with a clear error message if `ffmpeg` binary not found on startup.
- **[Abandoned processing]** If server restarts mid-video-transcode, the post remains "processing" forever. → Mitigation: mark processing=false with error on startup sweep.
- **[Image quality trade-off]** 85% JPEG is a good default but some images may look compressed. → Mitigation: make quality configurable via environment variable (IMAGE_QUALITY, default 85).
- **[Backwards compatibility]** Existing users logging in via username instead of email. → Mitigation: `/api/auth/login` accepts both email and migrated username (for existing admin). New registrations require email.

## Migration Plan

1. Add new columns to `users` table in migration
2. Create `user_sessions` table
3. Seed existing admin user with role="admin", paid=true
4. Deploy Docker image with ffmpeg included
5. Existing admin login still works (username/password, now via new login endpoint)
6. New visitor flow: Register → (optional) Pay → Browse unlocked content
7. Old cookie-based subscription (`royal_member`, `unlocked_N`) is no longer checked — existing subscribers will see locked content. Admin can manually grant them paid status.

## Open Questions

- Should the feed show locked post titles/descriptions to unpaid users (with blurred media), or hide locked posts entirely from the feed? → **Chosen**: Show titles + blurred preview (same as current behavior), media URL returned as null.
- Should we preserve the old `Subscribe` endpoint for backwards compatibility? → **Chosen**: Replace it. Old cookies ignored. No backward compat needed for mock payment.

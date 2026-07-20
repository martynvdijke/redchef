# Phase 1: Auth + Paywall + Media Pipeline + Admin Users + Feed Filters

## Why
RedChef needs proper user authentication, a working paywall, media optimization, admin user management, and feed filtering to be a functional content platform. Currently it uses cookie-based mock paywall with no user model, raw upload handling, and no filtering.

## What Changes

- **Add user registration with email + password** — first user is admin, all subsequent users are normal
- **Add role-based access** — admin users see/d o everything; normal users see public content unless they've paid (mock payment flow)
- **Add email field to users** — migrate existing username-based users to have email
- **Add paid status tracking** — users marked as `paid` after mock payment flow
- **Add mock payment flow** — simplified "unlock" flow that marks user as paid and clears locked post restrictions
- **Add image resize + compression** — server-side processing after upload (max width, JPEG/WebP compression)
- **Add video resize + transcoding** — ffmpeg-based resize to 1080p max, H.264 + AAC, with thumbnail generation
- **Add ffmpeg dependency to Dockerfile**
- **Add admin Users page** — list all users, see paid status, email, role; toggle paid status manually
- **Add feed filters** — query params on `GET /api/posts` for `sort`, `type`, `date_from`, `date_to`
- **Replace existing cookie-based subscription with user-scoped session auth** — normal users get session cookies; public feed uses session to show/hide locked content
- **Streamline UI** — add login/register page, update public feed to use session-aware locking, update admin UI with Users tab

## Capabilities

### New Capabilities
- `user-auth` — Registration, login, session management for normal users
- `paywall` — Payment flow (mock), paid status tracking, access gating
- `media-pipeline` — Image resize/compress, video resize/transcode, thumbnail generation
- `admin-users` — Admin users list, paid status management, role visibility
- `feed-filters` — Sort by date, filter by media type, date range on public feed

### Modified Capabilities
- *(none — no existing specs)*

## Impact

- **Database**: New tables/columns needed — `users` gets `email`, `role`, `paid` columns; new `user_sessions` table
- **Dependencies**: ffmpeg added to Docker (multistage build or Alpine package)
- **API changes**:
  - `POST /api/auth/register` (new)
  - `POST /api/auth/login` (replaces existing admin-only login; now returns user role)
  - `POST /api/auth/logout` (replaces admin logout)
  - `POST /api/pay/unlock` (replaces `/api/subscribe` for mock payment)
  - `GET /api/admin/users` (new)
  - `PATCH /api/admin/users/{id}` (new — toggle paid status)
  - `GET /api/posts?sort=latest&type=photo|video&date_from=...&date_to=...` (modified)
  - `GET /api/posts/{id}` now includes `unlocked` status per session
- **Frontend**: New pages (login, register), updated feed (session-aware locked posts), admin gets Users tab
- **Existing admin login migrates** to new auth system (existing user becomes admin with role=admin)

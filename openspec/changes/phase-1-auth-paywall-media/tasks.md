# Phase 1: Auth + Paywall + Media Pipeline — Tasks

## 1. Database Schema Migration

- [x] 1.1 Add migration to add `email`, `role`, `paid` columns to `users` table
- [x] 1.2 Create `user_sessions` table (id, token, user_id, created_at)
- [x] 1.3 Add `processing` column to `posts` table
- [x] 1.4 Create migration function that seeds existing admin user with role=admin, paid=true
- [x] 1.5 Update `Post` struct with `Processing` field
- [x] 1.6 Add DB functions: CreateUserSession, GetUserSession, DeleteUserSession, GetUserByEmail, GetUserByID, ListUsers, UpdateUserPaid
- [x] 1.7 Update db_test.go with new table/field tests

## 2. Auth API (Backend)

- [x] 2.1 Implement POST /api/auth/register (email, password, confirm_password)
- [x] 2.2 Implement POST /api/auth/login (email, password — also accept username for existing admin)
- [x] 2.3 Implement POST /api/auth/logout
- [x] 2.4 Implement GET /api/auth/me (returns current user info)
- [x] 2.5 Create `AuthMiddleware` that reads session cookie, sets user info in context
- [x] 2.6 Create `AdminMiddleware` that checks role=admin
- [x] 2.7 Replace existing `AdminAuth` handler to use new middleware chain
- [x] 2.8 Update routes in main.go (remove old login/logout, add new auth routes)

## 3. Paywall (Backend)

- [x] 3.1 Implement POST /api/pay/unlock (mock payment — sets paid=true on user)
- [x] 3.2 Update GET /api/posts to return `unlocked` field per post based on user session
- [x] 3.3 Update GET /api/posts/{id} to return `media_url: null` and `unlocked: false` for locked posts when user is unpaid
- [x] 3.4 Remove old cookie-based subscription logic from handlers/public.go
- [x] 3.5 Remove old `/api/subscribe` route

## 4. Media Pipeline (Backend)

- [x] 4.1 Add `github.com/disintegration/imaging` dependency
- [x] 4.2 Implement image processing function: resize to max 1920px, save at 85% JPEG quality
- [x] 4.3 Implement video processing function: ffmpeg subprocess for H.264/AAC/MP4 transcode at max 1080p, 8 Mbps
- [x] 4.4 Implement video thumbnail extraction (ffmpeg frame at 2s, 640x360)
- [x] 4.5 Implement async media processing goroutine after upload
- [x] 4.6 Add `processing` field to post response; update after processing completes
- [x] 4.7 Add startup recovery for abandoned processing jobs

## 5. Docker & Deployment

- [x] 5.1 Add ffmpeg + ffprobe to Dockerfile (apk add ffmpeg)
- [x] 5.2 Ensure ffmpeg binary check on startup (fail fast if missing)
- [x] 5.3 Add IMAGE_QUALITY env var (default 85)
- [x] 5.4 Update Makefile if needed

## 6. Admin API + UI

- [x] 6.1 Implement GET /api/admin/users (list all users, no password hashes)
- [x] 6.2 Implement PATCH /api/admin/users/{id} (toggle paid status)
- [x] 6.3 Implement DELETE /api/admin/users/{id} (with safeguard against deleting last admin)
- [x] 6.4 Update admin.html with Users tab/panel
- [x] 6.5 Update admin.js with users CRUD and UI rendering

## 7. Feed Filters (Backend + Frontend)

- [x] 7.1 Implement DB query with dynamic WHERE and ORDER BY for filters
- [x] 7.2 Update GET /api/posts handler to parse query params (sort, type, date_from, date_to)
- [x] 7.3 Add filter bar UI to index.html (sort toggle, type toggle, date inputs)
- [x] 7.4 Update app.js to apply filters and re-fetch posts

## 8. Frontend Pages (Login + Register)

- [x] 8.1 Create /static/login.html — login form (email, password)
- [x] 8.2 Create /static/register.html — registration form (email, password, confirm)
- [x] 8.3 Create /static/login.js — login client logic (stores session)
- [x] 8.4 Create /static/register.js — registration client logic
- [x] 8.5 Update index.html nav to show login/register links (hide for authenticated)
- [x] 8.6 Update index.html to use session-aware locking (check /api/auth/me on load)
- [x] 8.7 Add paywall/pay button in UI for authenticated unpaid users
- [x] 8.8 Update CSS for login/register and pay button styles

## 9. Verification

- [x] 9.1 Run existing tests (`go test ./...`)
- [x] 9.2 Manual test: register new user, verify normal role and unpaid
- [x] 9.3 Manual test: login as admin, verify admin users list, toggle paid status
- [x] 9.4 Manual test: upload image, verify resize/compression
- [x] 9.5 Manual test: upload video, verify transcode + thumbnail
- [x] 9.6 Manual test: verify locked post gating for unpaid user
- [x] 9.7 Manual test: verify filters (sort, type, date range)
- [x] 9.8 Manual test: existing admin login still works (username/password migration)

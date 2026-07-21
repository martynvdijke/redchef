# Testing & CI Improvements Design

## Overview
Improve RedChef's test coverage (Go unit tests), add E2E testing (Playwright), and strengthen CI pipeline with coverage reporting.

## Go Test Coverage

### New Test Files
- `handlers/public_test.go` — ListPosts (filters, auth states), GetPost (found/not found/locked), PublicGetAnalyticsSettings
- `handlers/auth_test.go` — Register (validation, duplicate email, auto-login), Login (email/username fallback, wrong password), Logout (cookie clearing), Me (authenticated/anonymous), middleware tests (AuthMiddleware, RequireAuth, AdminAuth)
- `handlers/comments_test.go` — ListComments, CreateComment (auth check, post exists check, empty body), AdminDeleteComment
- `handlers/favourites_test.go` — ToggleFavourite (add, remove, post not found), ListFavourites (empty, populated)
- `handlers/tips_test.go` — CreateTip (valid, invalid amount, auth required, post not found)
- `handlers/paywall_test.go` — PayUnlock (first time, already paid, unauth), PayItem (valid, invalid post_id, already purchased, admin bypass)

### DB Test Extensions
- Comments CRUD, favourites toggle, tips CRUD and totals, purchases, post links, user queries (by email, username, list, update), session management

### Coverage in CI
- Generate `coverage.out` via `go test -coverprofile=coverage.out ./...`
- Upload as CI artifact
- Display coverage summary in CI output

## Playwright E2E Tests

### Setup
- Playwright starts the Go server inline (builds binary, runs on random port, polls `/api/setup/status` until ready)
- Test data is seeded per-suite via API calls
- Tests run against the static SPA (real browser interactions)

### Test Suites
- `setup.spec.ts` — first-run setup page loads, admin account creation, redirect to admin.html
- `auth.spec.ts` — register modal, login modal, logout, session persistence
- `public.spec.ts` — posts load in feed, filters work, single post view
- `admin.spec.ts` — login as admin, upload post, edit/delete post, manage settings
- `comments.spec.ts` — add comment to post, admin deletes comment
- `paywall.spec.ts` — subscription unlock flow, per-item purchase flow

### CI Integration
- Separate CI job for Playwright
- Installs deps `npx playwright install --with-deps`
- Builds Go binary in CI before tests

## Release Process
- No changes to semantic-release config (already solid)
- Coverage artifacts available on CI runs for visibility

## Context

RedChef already has session authentication, `AuthMiddleware`/`RequireAuth`, admin authorization, and CSRF-aware browser flows. Public post and TRMNL reads must remain usable while API writes gain a machine-client credential without bypassing user ownership or admin checks.

## Goals / Non-Goals

### Goals

- Add hashed, revocable bearer tokens scoped to users.
- Require session identity and token identity to agree before mutations.
- Preserve public GET and existing browser behavior.

### Non-Goals

- Changing public read payloads or TRMNL configuration.
- Generating a real production token in source control.
- Replacing browser sessions, CSRF, or admin roles.

## Decisions

- Store token metadata and a cryptographic hash with the owning user and timestamps; compare hashes using constant-time verification.
- Accept `Authorization: Bearer <token>` through a dedicated middleware composed with existing session/auth middleware.
- Apply protection to post comment/favourite/tip writes and every create/update/delete route not already protected; keep `/api/admin` admin checks.
- Expose create/list/revoke/rotate operations only to the authenticated owner and show secrets only on creation.

## Risks / Trade-offs

- Existing API clients must obtain a token and send a session plus bearer token; migration errors should be explicit and logged without secrets.
- Requiring two credentials is safer but less convenient than token-only access.

## Migration Plan

1. Add the token table and indexes with a reversible migration.
2. Implement token lifecycle endpoints/UI and middleware in audit/logging-safe mode.
3. Protect mutations, add authorization tests, and deploy.
4. Issue tokens to existing users and document client migration; revoke any leaked token.

## Open Questions

- Should tokens have configurable expiry or only explicit revocation?
- Which existing API clients need an automated session bootstrap?

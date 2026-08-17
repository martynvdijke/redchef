## 1. Inventory and persistence

- [ ] 1.1 Confirm every RedChef create/update/delete route and its existing admin/ownership check.
- [ ] 1.2 Add a reversible token table with owner, hash, name, created, last-used, expiry, and revoked timestamps.

## 2. Authentication

- [ ] 2.1 Implement bearer-token parsing, hashing, constant-time verification, expiry, and revocation checks.
- [ ] 2.2 Require session user and token owner agreement for all mutations while preserving admin and CSRF middleware.
- [ ] 2.3 Add authenticated create/list/revoke/rotate token endpoints with one-time secret responses.

## 3. Verification and rollout

- [ ] 3.1 Add tests for public reads, anonymous/token-only mutations, ownership, malformed/expired/revoked tokens, and admin routes.
- [ ] 3.2 Document token creation and migrate existing clients without committing real secrets.
- [ ] 3.3 Run the full RedChef test suite and review logs for secret leakage.

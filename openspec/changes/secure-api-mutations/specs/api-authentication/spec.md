## ADDED Requirements

### Requirement: Public read endpoints remain accessible

The service SHALL allow unauthenticated GET requests to public read endpoints, including `/api/trmnl/latest`, posts, comments, favourites, and analytics reads that are currently public.

#### Scenario: TRMNL reads without credentials

- WHEN a client requests `GET /api/trmnl/latest` without a session or bearer token
- THEN the service returns the current public TRMNL payload

### Requirement: Mutations require user and API token authentication

The service MUST require both an authenticated application user and a valid bearer API token for post comments, favourites, tips, and all create/update/delete routes. Existing admin routes SHALL retain their admin authorization.

#### Scenario: Anonymous mutation is rejected

- WHEN an unauthenticated client submits a mutation
- THEN the service returns 401 or 403 and performs no write

#### Scenario: Token without user session is rejected

- WHEN a request has a valid bearer token but no authenticated user session
- THEN the service rejects the mutation and performs no write

### Requirement: Tokens are securely managed

An authenticated user SHALL be able to create a token, see its secret exactly once, list token metadata, revoke a token, and rotate a token. The service MUST store only a one-way hash, never log the secret, and never return it after creation.

#### Scenario: Token creation is one-time visible

- WHEN an authenticated user creates a token
- THEN the response returns the secret once and later metadata responses omit it

### Requirement: Token security failures are safe

The service MUST reject malformed, expired, revoked, and tokens owned by another user without disclosing whether a token exists, and SHALL record no mutation.

#### Scenario: Revoked token cannot mutate

- WHEN a user submits a revoked token to a protected route
- THEN the service returns an authorization error and leaves data unchanged

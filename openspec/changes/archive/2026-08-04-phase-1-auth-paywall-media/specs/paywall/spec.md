# Paywall

## ADDED Requirements

### Requirement: Paid status tracking
The system SHALL track whether a user has paid (paid=true) on the user record.

#### Scenario: New users start unpaid
- **WHEN** a new user registers
- **THEN** the system SHALL set paid=false for the new user (except first user who gets paid=true as admin)

#### Scenario: Admin always has access
- **WHEN** a user with role="admin" requests locked content
- **THEN** the system SHALL grant access regardless of paid status

#### Scenario: Paid user can access all locked content
- **WHEN** a user has paid=true
- **THEN** the system SHALL treat all locked posts as accessible

### Requirement: Mock payment flow
The system SHALL provide a mock payment endpoint that simulates a payment unlocking the user.

#### Scenario: User pays to unlock all content
- **WHEN** an authenticated normal user with paid=false calls POST /api/pay/unlock
- **THEN** the system SHALL set paid=true on the user record
- **THEN** the system SHALL return a success response with receipt details

#### Scenario: Already paid user
- **WHEN** an authenticated user with paid=true calls POST /api/pay/unlock
- **THEN** the system SHALL return a 200 success (idempotent)

#### Scenario: Anonymous user tries to pay
- **WHEN** an unauthenticated visitor calls POST /api/pay/unlock
- **THEN** the system SHALL return a 401 Unauthorized error

### Requirement: Locked content access gating
The system SHALL prevent unpaid normal users from viewing locked post media.

#### Scenario: Unpaid user sees blurred preview
- **WHEN** a normal user with paid=false views a post with locked=true
- **THEN** the system SHALL return the post metadata but with media_url=null
- **THEN** the response SHALL include `unlocked: false`

#### Scenario: Paid user sees full content
- **WHEN** a paid user (or admin) views a locked post
- **THEN** the system SHALL return full post with media_url and `unlocked: true`

#### Scenario: Anonymous user sees blurred preview
- **WHEN** an unauthenticated visitor views a locked post
- **THEN** the system SHALL return post metadata with media_url=null and `unlocked: false`

### Requirement: Public feed lists all posts but marks locked
The system SHALL include all posts in the public feed but indicate which are locked for the current user.

#### Scenario: Feed shows locked status per user
- **WHEN** an authenticated user requests GET /api/posts
- **THEN** the response SHALL include `unlocked` field per post indicating accessibility
- **WHEN** the request is anonymous
- **THEN** all locked posts SHALL have `unlocked: false`

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /api/pay/unlock | Session | Mock payment — marks user as paid |

## Removed Requirements

### Requirement: Cookie-based item purchase
**Reason**: Replaced by user-scoped paid flag and role-based paywall.
**Migration**: The `/api/subscribe` endpoint and cookie-based (`royal_member`, `unlocked_%d`) approach is deprecated. Existing cookies will be ignored.

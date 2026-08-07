# user-auth Specification

## Purpose
TBD - created by archiving change phase-1-auth-paywall-media. Update Purpose after archive.
## Requirements
### Requirement: User registration with email and password
The system SHALL allow new users to register with email and password.

#### Scenario: Successful registration
- **WHEN** a visitor submits a registration with valid email (non-empty, contains @) and password (min 6 characters)
- **THEN** the system SHALL create a new user account with role="normal" and paid=false  
- **THEN** the system SHALL return a success response with a session token

#### Scenario: First user is admin
- **WHEN** the users table is empty and a visitor submits registration
- **THEN** the system SHALL create the user with role="admin" and paid=true (admin always has access)

#### Scenario: Duplicate email registration
- **WHEN** a visitor submits registration with an email already in use
- **THEN** the system SHALL return a 409 Conflict error

#### Scenario: Invalid email format
- **WHEN** a visitor submits registration with empty email or missing @
- **THEN** the system SHALL return a 400 Bad Request error

#### Scenario: Password too short
- **WHEN** a visitor submits registration with password fewer than 6 characters
- **THEN** the system SHALL return a 400 Bad Request error

### Requirement: User login with email and password
The system SHALL allow registered users to login with email and password.

#### Scenario: Successful login
- **WHEN** a user submits valid email and matching password
- **THEN** the system SHALL create a session and return a session token
- **THEN** the response SHALL include the user's role (admin|normal)

#### Scenario: Invalid credentials
- **WHEN** a user submits wrong password or non-existent email
- **THEN** the system SHALL return a 401 Unauthorized error

#### Scenario: Session duration
- **WHEN** a session is created after login
- **THEN** the session SHALL expire after 30 days of inactivity
- **THEN** the session cookie SHALL be HttpOnly, SameSite=Lax

### Requirement: User logout
The system SHALL allow users to end their session.

#### Scenario: Successful logout
- **WHEN** an authenticated user calls logout
- **THEN** the system SHALL delete the session and clear the cookie

### Requirement: Session-based auth middleware
The system SHALL use session tokens to authenticate requests for protected endpoints.

#### Scenario: Valid session
- **WHEN** a request includes a valid session cookie
- **THEN** the system SHALL identify the user and make `user_id` and `role` available to downstream handlers

#### Scenario: Invalid or expired session
- **WHEN** a request includes no session cookie, or an invalid/expired token
- **THEN** the system SHALL treat the request as unauthenticated (anonymous)

### Requirement: Existing admin migration
The system SHALL migrate the existing admin username-based account to the new model.

#### Scenario: Existing user gets role=admin
- **WHEN** the database has users in the old schema (username column)
- **THEN** the first existing user SHALL get role="admin" and paid=true on migration
- **THEN** username SHALL be copied to email field as `username@local` if no email exists
- **THEN** password_hash SHALL remain unchanged


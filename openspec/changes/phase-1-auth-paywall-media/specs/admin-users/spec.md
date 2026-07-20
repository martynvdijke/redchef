# Admin Users Management

## ADDED Requirements

### Requirement: Admin users list view
The system SHALL provide an authenticated admin endpoint listing all registered users.

#### Scenario: Admin views all users
- **WHEN** an authenticated admin calls GET /api/admin/users
- **THEN** the response SHALL include all users with: id, email, role, paid, created_at
- **THEN** the response SHALL NOT include password hashes

#### Scenario: Non-admin denied
- **WHEN** a normal user calls GET /api/admin/users
- **THEN** the system SHALL return a 403 Forbidden error

### Requirement: Admin can toggle user paid status
The system SHALL allow an admin to grant or revoke paid status for a user.

#### Scenario: Admin grants paid access
- **WHEN** an admin calls PATCH /api/admin/users/{id} with `paid: true`
- **THEN** the system SHALL set the user's paid flag to true

#### Scenario: Admin revokes paid access
- **WHEN** an admin calls PATCH /api/admin/users/{id} with `paid: false`
- **THEN** the system SHALL set the user's paid flag to false

#### Scenario: Admin cannot change roles
- **WHEN** an admin calls PATCH /api/admin/users/{id} with `role: admin`
- **THEN** the system SHALL ignore the role field (role changes not supported in Phase 1)

#### Scenario: Non-admin denied
- **WHEN** a normal user calls PATCH /api/admin/users/{id}
- **THEN** the system SHALL return a 403 Forbidden error

### Requirement: Admin can delete users
The system SHALL allow an admin to delete user accounts.

#### Scenario: Admin deletes user
- **WHEN** an admin calls DELETE /api/admin/users/{id}
- **THEN** the system SHALL delete the user and their sessions
- **THEN** the system SHALL NOT allow deleting the last admin user

### Requirement: Admin UI users tab
The admin HTML interface SHALL have a "Users" tab showing the user table.

#### Scenario: Admin sees users in dashboard
- **WHEN** an admin is on the admin page
- **THEN** a "Users" tab/panel SHALL display the users table
- **THEN** each row SHALL show email, role, paid status, created date
- **THEN** the admin can toggle paid status with a button per row
- **THEN** the admin can delete a user with a confirmation dialog

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/admin/users | Admin | List all users |
| PATCH | /api/admin/users/{id} | Admin | Update user (paid status) |
| DELETE | /api/admin/users/{id} | Admin | Delete user |

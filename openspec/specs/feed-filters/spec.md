# feed-filters Specification

## Purpose
TBD - created by archiving change phase-1-auth-paywall-media. Update Purpose after archive.
## Requirements
### Requirement: Sort by latest
The system SHALL support sorting posts by creation date.

#### Scenario: Default sort is latest first
- **WHEN** GET /api/posts is called without sort parameter
- **THEN** the system SHALL return posts ordered by created_at DESC (latest first)

#### Scenario: Explicit sort parameter
- **WHEN** GET /api/posts?sort=latest is called
- **THEN** the system SHALL return posts ordered by created_at DESC

### Requirement: Filter by media type
The system SHALL support filtering posts by media type (photo or video).

#### Scenario: Filter photos only
- **WHEN** GET /api/posts?type=photo is called
- **THEN** the system SHALL return only posts with media_type="photo"

#### Scenario: Filter videos only
- **WHEN** GET /api/posts?type=video is called
- **THEN** the system SHALL return only posts with media_type="video"

#### Scenario: Invalid type value
- **WHEN** GET /api/posts?type=invalid is called
- **THEN** the system SHALL return a 400 Bad Request error

### Requirement: Filter by date range
The system SHALL support filtering posts by creation date range.

#### Scenario: Filter by start date
- **WHEN** GET /api/posts?date_from=2026-01-01 is called
- **THEN** the system SHALL return only posts created on or after 2026-01-01

#### Scenario: Filter by end date
- **WHEN** GET /api/posts?date_to=2026-06-30 is called
- **THEN** the system SHALL return only posts created on or before 2026-06-30

#### Scenario: Filter by both dates
- **WHEN** GET /api/posts?date_from=2026-01-01&date_to=2026-06-30 is called
- **THEN** the system SHALL return posts within that range

#### Scenario: Combined filters
- **WHEN** GET /api/posts?type=photo&sort=latest&date_from=2026-01-01 is called
- **THEN** the system SHALL apply all filters simultaneously

### Requirement: Front-end filter UI
The public feed page SHALL provide UI controls for filtering and sorting.

#### Scenario: Filter bar visible on feed
- **WHEN** a user visits the public feed
- **THEN** a filter bar SHALL be visible above the posts with:
  - Sort toggle: Latest
  - Type toggle: All / Photos / Videos
  - Date range picker (optional, collapsible)

#### Scenario: Filters update feed without page reload
- **WHEN** a user changes a filter value
- **THEN** the system SHALL re-fetch posts with the applied filter parameters
- **THEN** the feed SHALL update without a full page reload


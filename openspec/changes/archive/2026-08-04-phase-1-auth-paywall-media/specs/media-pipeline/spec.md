# Media Pipeline

## ADDED Requirements

### Requirement: Image resize and compression after upload
The system SHALL process uploaded images to reduce file size and enforce maximum dimensions.

#### Scenario: Image resized to max width
- **WHEN** an image is uploaded with width greater than 1920px
- **THEN** the system SHALL resize it to 1920px wide, maintaining aspect ratio

#### Scenario: Image compressed with quality reduction
- **WHEN** an image is uploaded
- **THEN** the system SHALL save it at 85% JPEG quality (or equivalent WebP quality)
- **THEN** the original uploaded file SHALL be replaced with the processed version

#### Scenario: Small images left as-is
- **WHEN** an image is uploaded with dimensions under 1920px and under 500KB
- **THEN** the system MAY skip processing to preserve quality

#### Scenario: Image format support
- **WHEN** an image is uploaded as JPEG, PNG, or WebP
- **THEN** the system SHALL process it to JPEG output with configurable quality
- **WHEN** a PNG with transparency is uploaded
- **THEN** the system SHALL preserve the original format (output as PNG)

### Requirement: Video resize and transcoding after upload
The system SHALL process uploaded videos using ffmpeg to enforce maximum resolution and web-compatible encoding.

#### Scenario: Video resized to 1080p max
- **WHEN** a video is uploaded with resolution above 1920x1080
- **THEN** the system SHALL scale it to fit within 1920x1080, maintaining aspect ratio

#### Scenario: Video transcoded to H.264
- **WHEN** a video is uploaded
- **THEN** the system SHALL transcode it to H.264 video, AAC audio in MP4 container
- **THEN** the original uploaded file SHALL be replaced with the transcoded version

#### Scenario: Video bitrate management
- **WHEN** a video is transcoded
- **THEN** the system SHALL apply a max video bitrate of 8 Mbps to balance quality and size

#### Scenario: Audio-only passthrough
- **WHEN** a video has no video stream (audio-only upload)
- **THEN** the system SHALL reject it with an error

### Requirement: Thumbnail generation for videos
The system SHALL generate a thumbnail image from uploaded videos.

#### Scenario: Video thumbnail extracted
- **WHEN** a video is uploaded and transcoded
- **THEN** the system SHALL extract a frame at the 2-second mark as a JPEG thumbnail
- **THEN** the thumbnail SHALL be saved alongside the processed video
- **THEN** the post record SHALL reference the thumbnail filename

#### Scenario: Thumbnail dimensions
- **WHEN** a video thumbnail is generated
- **THEN** it SHALL be 640px wide, maintaining aspect ratio, max 360p height

### Requirement: Processing performance
The system SHALL process media asynchronously after upload to keep the HTTP upload response fast.

#### Scenario: Post created immediately, processed async
- **WHEN** a file is uploaded via the admin upload endpoint
- **THEN** the system SHALL save the file temporarily, create the post record immediately
- **THEN** the system SHALL process (resize/compress/transcode) in a goroutine
- **THEN** the post record SHALL be updated when processing completes
- **THEN** the initial upload response SHALL include a `processing: true` flag

#### Scenario: Processing status visible
- **WHEN** a post is still being processed
- **THEN** GET /api/posts and GET /api/posts/{id} SHALL include `processing: true`
- **WHEN** processing is done
- **THEN** `processing` SHALL be false and `filename` SHALL point to the processed file

## API Changes

| Endpoint | Change |
|----------|--------|
| POST /api/admin/upload | Now returns `processing: true` immediately, processes async |
| GET /api/posts | Response includes `processing` field |
| GET /api/posts/{id} | Response includes `processing` field |

## Dependencies

- **Go**: `github.com/disintegration/imaging` for image processing (pure Go, no CGO)
- **System**: ffmpeg + ffprobe in Docker for video processing

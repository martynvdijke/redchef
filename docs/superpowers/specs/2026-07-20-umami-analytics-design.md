# Umami Self-Hosted Analytics — Admin-Configurable Tracking

## Overview

Add Umami self-hosted analytics tracking to RedChef, configurable directly from the admin dashboard. The admin can set the Umami script URL and website ID through a settings panel in the admin UI, with settings persisted in SQLite and applied immediately — no server restart needed.

## Architecture

```
┌──────────────────────┐     ┌─────────────────┐     ┌──────────────┐
│  Admin Dashboard     │────▶│  PUT /api/admin │────▶│  SQLite      │
│  (admin.html + JS)   │     │  /settings/     │     │  analytics_  │
│                      │     │  analytics      │     │  settings    │
│  Settings Panel:     │     └─────────────────┘     └──────┬───────┘
│  - Script URL        │                                    │
│  - Website ID        │     ┌─────────────────┐            │
│  - Enable toggle     │     │  GET /api/      │            │
├──────────────────────┤     │  settings/      │◀───────────┘
│  Public Fan Page     │◀────│  analytics      │
│  (index.html + JS)   │     └─────────────────┘
│                      │
│  → Dynamically loads │
│    Umami tracking    │
│    script on init    │
└──────────────────────┘
```

**Key design decisions:**
- Dedicated `analytics_settings` table (typed columns for clarity)
- Separate admin (write) and public (read) API endpoints — public endpoint exposes only tracking config, no auth needed
- Dynamic script injection via JavaScript — no template system needed with embedded static files
- Settings apply immediately on save, no restart

## Data Model

New table in `db/db.go` migration:

```sql
CREATE TABLE IF NOT EXISTS analytics_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    umami_script_url TEXT NOT NULL DEFAULT '',
    umami_website_id TEXT NOT NULL DEFAULT '',
    tracking_enabled INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

A single row is inserted on migration if the table is empty (singleton pattern). All reads/writes use `WHERE id = 1`.

**Go struct:**

```go
type AnalyticsSettings struct {
    ID               int64     `json:"id"`
    UmamiScriptURL   string    `json:"umami_script_url"`
    UmamiWebsiteID   string    `json:"umami_website_id"`
    TrackingEnabled  bool      `json:"tracking_enabled"`
    UpdatedAt        time.Time `json:"updated_at"`
}
```

**DB methods (new):**
- `GetAnalyticsSettings() (*AnalyticsSettings, error)` — returns current settings
- `UpdateAnalyticsSettings(url, websiteID string, enabled bool) error` — upserts settings row
- Migration seeds default row in `migrate()`

## API Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `GET` | `/api/settings/analytics` | None | Public: returns `{ umami_script_url, umami_website_id, tracking_enabled }` |
| `GET` | `/api/admin/settings/analytics` | Admin | Admin: returns full settings |
| `PUT` | `/api/admin/settings/analytics` | Admin | Admin: update settings |

**PUT request body:**
```json
{
    "umami_script_url": "https://analytics.example.com/script.js",
    "umami_website_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "tracking_enabled": true
}
```

## Admin UI: Settings Panel

New section in the admin dashboard (`admin.html`) below the upload form and above the posts table:

**Layout:**
- Section card with heading "⚙️ Analytics Settings"
- Row 1: Enable/disable toggle (checkbox with label "Enable Umami Analytics")
- Row 2: Input "Script URL" — placeholder `https://analytics.example.com/script.js`
- Row 3: Input "Website ID" — placeholder `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`
- Save button → shows status message
- On save success, status "Settings saved" in green

**JavaScript (`admin.js`):**
- `loadSettings()` — called on dashboard show, GETs settings, populates form
- `handleSaveSettings(e)` — PUTs form data, shows status
- Wire up to new "Settings" section in the HTML

## Frontend Tracking: Dynamic Script Injection

**In both `index.html` and `admin.html`:** A `<script>` tag (or inline code in the respective JS files) runs on page load:

1. `fetch('/api/settings/analytics')` — get tracking config
2. If `tracking_enabled` and both URL/website ID are filled:
   - Create `<script async defer src="{umami_script_url}" data-website-id="{umami_website_id}"></script>`
   - Append to `<head>`
3. If disabled or no config, do nothing

This logic lives in a shared helper, added to both `app.js` (public) and `admin.js` (admin):

```javascript
async function initUmamiTracking() {
    try {
        const res = await fetch('/api/settings/analytics');
        if (!res.ok) return;
        const settings = await res.json();
        if (!settings.tracking_enabled || !settings.umami_script_url || !settings.umami_website_id) return;

        const script = document.createElement('script');
        script.async = true;
        script.defer = true;
        script.src = settings.umami_script_url;
        script.setAttribute('data-website-id', settings.umami_website_id);
        document.head.appendChild(script);
    } catch (_) { /* analytics unavailable — no-op */ }
}
```

## Files Modified

| File | Change |
|------|--------|
| `db/db.go` | Add `analytics_settings` table migration, `AnalyticsSettings` struct, `GetAnalyticsSettings()`, `UpdateAnalyticsSettings()` |
| `handlers/admin.go` | Add `AdminGetAnalyticsSettings` + `AdminUpdateAnalyticsSettings` handlers |
| `handlers/public.go` | Add `PublicGetAnalyticsSettings` handler |
| `main.go` | Register 3 new routes |
| `static/admin.html` | Add settings section HTML to dashboard |
| `static/admin.js` | Add `loadSettings()`, `handleSaveSettings()`, `initUmamiTracking()` |
| `static/app.js` | Add `initUmamiTracking()` call on DOMContentLoaded |

## Validation

- **Script URL**: Must start with `https://` or `http://`, validated server-side. If invalid, return 400.
- **Website ID**: Must be a valid UUID format (optional validation — warn but don't block).
- **Tracking enabled**: Boolean toggle. Disabling removes the script from the next page load (already-loaded scripts continue running until refresh).

## Edge Cases

- **No settings saved yet**: Default row with empty strings and `tracking_enabled = 0`. Public API returns disabled state, no script loads.
- **Invalid/malformed URL**: Server rejects with 400 and descriptive error. Form shows error.
- **Umami server down**: Script fails to load silently — no impact on page. Standard `<script>` async behavior.
- **Concurrent admin sessions**: Last writer wins. Acceptable for single-admin use case.
- **Public user hits settings endpoint**: Returns only tracking config, no auth needed. No sensitive data.

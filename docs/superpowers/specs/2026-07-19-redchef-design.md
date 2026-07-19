# RedChef — OnlyFans for the 5-Minute Red Copper Chef

## Overview
A parody OnlyFans-style web page themed around the "5 Minute Red Copper Chef" — a tiny, delusional kitchen appliance that thinks it's a world-famous celebrity chef. Built as a single Go binary with embedded frontend, SQLite storage, and Docker deployment.

## Architecture

```
redchef/
├── main.go               # Entry point, server + routing
├── go.mod / go.sum
├── handlers/
│   ├── auth.go           # Admin login/logout/session
│   ├── admin.go          # Upload, delete, manage posts
│   └── public.go         # Fan-facing feed & content API
├── db/
│   └── db.go             # SQLite init, migrations, queries
├── static/               # Embedded via //go:embed
│   ├── index.html        # Fan landing page
│   ├── admin.html        # Admin dashboard
│   ├── style.css         # As-Seen-On-TV kitsch theme
│   └── app.js            # Client-side logic
├── uploads/              # Runtime volume for media files
├── Dockerfile            # Multi-stage: Go build → scratch
├── docker-compose.yml
└── Makefile
```

**Key technical decisions:**
- Pure Go SQLite via `modernc.org/sqlite` — zero CGO, fully static binary
- `embed` for frontend assets — zero runtime file deps for UI
- Uploaded media stored on a Docker volume or host path
- Cookie-based sessions for admin, simple cookie for "subscriber" gate
- Single-binary deployment: `docker compose up`

## Data Model (SQLite)

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL
);

CREATE TABLE posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    media_type TEXT NOT NULL,      -- 'photo' | 'video'
    filename TEXT NOT NULL,
    thumbnail TEXT DEFAULT '',
    locked INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT UNIQUE NOT NULL,
    user_id INTEGER NOT NULL,
    expires_at DATETIME NOT NULL
);
```

- Default admin user seeded via `ADMIN_USERNAME` / `ADMIN_PASSWORD` env vars on first startup.
- Posts with `locked=1` show only thumbnails; full media is revealed when the user clicks "Pay 5¢" — a parody microtransaction that charges nothing.

## API Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/api/posts` | None | Public feed — returns posts list with thumbnail URLs + locked status |
| GET | `/api/posts/:id` | None | Full post details, always returns all data (gate is client-side) |
| POST | `/api/unlock` | None | Records that user "paid 5¢" — sets cookie tracking unlocked posts |
| POST | `/api/admin/login` | None | Username+password → admin session cookie |
| POST | `/api/admin/upload` | Admin | Multipart upload: file + title + description |
| GET | `/api/admin/posts` | Admin | List all posts with full details |
| DELETE | `/api/admin/posts/:id` | Admin | Delete post + media file |
| POST | `/api/admin/logout` | Admin | Clear session |

## Design Language

- **Colors:** Rich red `#D42B2B`, copper/rust `#C76B3A`, gold `#F5C518`, off-black `#1A1A1A`
- **Vibe:** late-night infomercial meets tiny appliance delusion of grandeur
- **UI motifs:** "AS SEEN ON TV" starburst badges, "5 MINUTE" countdown rings, chef hat crown icon
- **Layout:** Instagram-style card grid for content, dark admin panel with sidebar

## Pages

### Fan Page (`/`)
- Hero section: The Chef (tiny appliance) with dramatic pose, "THE 5 MINUTE RED COPPER CHEF" title
- Content grid: 3-column card layout, each showing thumbnail + title
- Locked cards: blurred preview + "🔒 Pay 5¢ to see this" button overlay
- Unlock flow: click "Pay 5¢" → comedic "Processing payment..." animation → "Thank you! Your card has been charged $0.05" (mock toast) → content slides open in-place
- Unlocked state is tracked client-side via cookie (list of unlocked post IDs)

### Admin Login (`/admin/`)
- Centered login form with Chef branding
- Username + password fields

### Admin Dashboard (`/admin/dashboard`)
- Upload form: file picker, title, description fields
- Content table: thumbnail preview, title, type, date, delete button

## Implementation Order

1. Go project scaffolding (go.mod, main.go, directory structure)
2. SQLite database layer (db.go — init, migrations, CRUD)
3. Auth handlers (login, logout, session middleware)
4. Admin handlers (upload, list, delete posts)
5. Public handlers (list posts, get post, subscribe)
6. Frontend: fan page (index.html + CSS + JS)
7. Frontend: admin pages (login + dashboard)
8. Docker setup (Dockerfile, docker-compose.yml)
9. Makefile for dev commands

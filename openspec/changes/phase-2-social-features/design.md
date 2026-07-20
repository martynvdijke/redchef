# Phase 2 Design: Comments, Favourites, Tips, WhatsApp, Linked Posts

## Features

### 1. Comments (Threaded, Registered Users)
- `comments` table: id, post_id, user_id, parent_id (nullable), body, created_at
- GET /api/posts/{id}/comments — flat list with parent_id for client-side tree
- POST /api/posts/{id}/comments — auth required, body + optional parent_id
- DELETE /api/admin/comments/{id} — admin delete
- Frontend: expandable thread under post, reply inline form, "Login to comment" for guests

### 2. Favourites
- `favourites` table: id, user_id, post_id, created_at (UNIQUE user+post)
- POST /api/posts/{id}/favourite — toggle on/off (insert/delete)
- GET /api/favourites — user's favourited posts (same feed format + filters)
- Post API returns `favourited: bool` and `favourite_count: int` per user
- Frontend: heart icon + count on cards, "My Favourites" nav link

### 3. Tips (Mock)
- `tips` table: id, user_id, post_id, created_at
- POST /api/posts/{id}/tip — auth required, inserts tip, returns count
- Post API returns `tip_count: int`
- Frontend: $ tip button + count, "Thanks!" toast

### 4. WhatsApp Sharing
- Frontend only: share icon → `wa.me/?text=` with post title + URL

### 5. Linked Posts (Directional Series)
- `post_links` table: id, post_id, linked_post_id, order_index
- Admin upload/edit: multi-select to link posts, order by selection
- Post API returns `linked_posts: [{id, title, order}]`
- Frontend: "Part of this series:" prev/next links below post

## Architecture
- Monolithic pattern (same as Phase 1): new files in db/, handlers/, updates to static/
- All new tables added to existing migrate() function
- All new routes added to main.go under appropriate middleware

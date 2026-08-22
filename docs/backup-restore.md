# Backup & Restore Runbook

RedChef is a single binary with two stateful locations:

| Data | Location | Configured via |
|------|----------|----------------|
| SQLite database | `/db/redchef.db` (+ `-wal`/`-shm`) | `DB_PATH` env |
| Uploaded media | `/app/media` | `UPLOAD_DIR` env |

## Exporting a backup

**Option A — Admin UI:** Dashboard → *Backup* → **⬇️ Export backup (zip)**.
Your browser downloads `redchef-backup-<timestamp>.zip`.

**Option B — API:**

```bash
curl -H "Cookie: session=<your-admin-session>" \
     -o redchef-backup.zip https://your-host/api/admin/export
```

The zip contains:

- `redchef.db` — consistent snapshot taken with SQLite `VACUUM INTO`
  (safe while the server keeps running; writers are not blocked)
- `media/<filename>` — every file under the uploads dir, including
  thumbnails and any orphaned files (harmless)

> The endpoint requires an admin session. Exports of large media dirs can
> take a while; the response streams entry-by-entry.

## What to keep

Download the zip off the box. For automated backups, wrap the API call in a
cron job and push the artifact to object storage — the export endpoint is
the only primitive you need.

## Restoring

1. Stop the RedChef container/binary.
2. Replace the database:

   ```bash
   unzip redchef-backup.zip redchef.db
   # remove stale WAL/SHM from the old DB
   rm -f "$DB_PATH"-wal "$DB_PATH"-shm
   cp redchef.db "$DB_PATH"
   ```

3. Restore media into the uploads dir:

   ```bash
   unzip -j redchef-backup.zip 'media/*' -d "$UPLOAD_DIR"/
   ```

4. Start RedChef again. Migrations are additive and idempotent — an older
   backup restores fine into a newer binary.

### Verify after restore

- `GET /api/posts` returns your posts
- Media thumbnails render (`GET /uploads/<filename>`)
- Admin dashboard loads and lists posts

## Notes

- The snapshot is crash-consistent at the moment of `VACUUM INTO`; posts
  uploaded during a long export may be newer than the snapshot but their
  media files still ride along in `media/`.
- `VACUUM INTO` needs free disk roughly equal to the DB size in the temp
  dir of the container. The temp file is removed on success and error.

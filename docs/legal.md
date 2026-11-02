# Legal pages and GDPR account deletion

## Public routes
- `GET /privacy` — privacy policy, no authentication required.
- `GET /terms` — terms of use, no authentication required.
- `GET /account/delete` — public deletion instructions and signed-in self-service entry point.
- `GET /api/me/export` — signed-in JSON export of the account's own data.
- `DELETE /api/me` — signed-in account deletion (JSON, typed confirmation = email).
- `POST /settings/account/delete` — browser form variant.

These routes are registered before the authentication middleware in `main.go`.

## Configuration
Set in the environment (or `.env`):
- `LEGAL_CONTROLLER_NAME`
- `LEGAL_CONTACT_EMAIL`
- `LEGAL_CONTACT_ADDRESS`
- `LEGAL_HOSTING_REGION`
- `LEGAL_BASE_URL` (optional, defaults to https://redchef.vandijke.xyz)

Missing values fall back to neutral wording; no contact details are invented.

## Deletion semantics
`db.DeleteUser` runs in one transaction:
- refuses to delete the last admin
- deletes sessions, API tokens, password resets
- promotes replies, then deletes comments, favourites, tips, purchases
- deletes the user row

Comments are removed with the account; replies remain as top-level comments. Tips/purchases are mock iDEAL records and are deleted because no real payment is processed.

## Backups
Backups taken before deletion can still contain old data until overwritten/expired. Deletion is not applied retroactively to backups; backup retention should be documented in the operator's schedule.

## EU launch checklist
- Set LEGAL_* to real values.
- Confirm hosting region and processors.
- Publish privacy URL in Play Console and in app.
- Provide test account credentials for Play review.
- Verify Android manifest: HTTPS-only, allowBackup false, data extraction rules.
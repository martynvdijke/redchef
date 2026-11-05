# Legal pages, account deletion and the demo account

Implementation notes for the public legal surface and the shared demo account. For the detailed processing record, retention and breach runbooks, see [gdpr.md](./gdpr.md).

## Public routes

| Route | Method | Auth | Purpose |
|---|---|---|---|
| `/privacy` | GET | Public | Privacy policy rendered from `LEGAL_*` values. |
| `/terms` | GET | Public | Terms of use. |
| `/account/delete` | GET | Public | Deletion instructions and the public deletion form; shows a success panel for `?deleted=1`. |
| `/account/delete` | POST | Public | Self-service deletion: email + password + typed email when signed out; typed email only when signed in. Immediate, then redirects to `/account/delete?deleted=1`. |
| `/api/auth/config` | GET | Public | Demo account availability. Returns `{"demo":{"enabled":false}}` when disabled, or `{"demo":{"enabled":true,"email":"...","password":"..."}}` when enabled. |
| `/api/me/export` | GET | Session | JSON export of the signed-in user's own data. |
| `/api/me` | DELETE | Session | JSON deletion; body `{"confirm":"<email>"}`. |
| `/settings/account/delete` | POST | Session | Browser form deletion; typed email; redirects to `/?deleted=1`. |

These handlers are registered directly on the root `http.ServeMux` in `main.go`, not behind a global auth layer. The legal pages and `/account/delete` are deliberately public so store reviewers and people locked out of an account can reach them. The API export/deletion routes add `AuthMiddleware` + `RequireAuth` per route. Do not move the public legal pages behind `RequireAuth`: Google Play and the GDPR both require them to work without a session.

`GET /api/auth/config` is also public by design and can expose the demo credentials to anyone who can reach the instance. See [Demo account](#demo-account).

## Demo account

The demo account is a normal, paid (all-content) account that is seeded or repaired on boot by `handlers.EnsureDemoUser`, which calls `db.EnsureDemoUser`. It is intended for store reviewers and visitors trying the hosted instance.

### Configuration

| Variable | Default | Behaviour |
|---|---|---|
| `DEMO_USER_ENABLED` | Auto | `true`/`1`/`yes`/`on` enables; `false`/`0`/`no`/`off` disables. When unset or empty, the demo is enabled only when the host from `LEGAL_BASE_URL` (or `BASE_URL`) is exactly `redchef.vandijke.xyz`. Any other host, or an invalid URL, leaves it disabled. |
| `DEMO_USER_EMAIL` | `demo@redchef.local` | Lowercased and trimmed. Use the non-routable default; do not point it at a real person's mailbox. |
| `DEMO_USER_PASSWORD` | `redchef-demo` | Must be at least 6 characters. An invalid value is logged and the demo is not seeded. |

### Boot behaviour

`db.EnsureDemoUser` runs on every start (and again after first-run setup):

- If the email does not exist and at least one user already exists, it creates a `role = 'demo'`, `paid = 1` account with a bcrypt hash.
- On a fresh install with zero users it creates nothing, so the next real registration can still become the first admin.
- If the email exists and already has `role = 'demo'`, it repairs `username` to the demo email, forces `paid = 1`, and resets the password to `DEMO_USER_PASSWORD` when it no longer matches.
- If the email belongs to an admin or any other non-demo account, it refuses to touch the account and logs an error. This prevents the demo seed from hijacking a real member or admin address. Never use a real address as the demo email.
- Failures are logged and never block startup.

The demo is deliberately **not** an admin and uses its own `demo` role. It can read all paid content and can write comments, favourites, tips and purchases like any normal account. Its comments are public and shared by every visitor, so do not put personal data in the demo account.

### Deletion protection

While `DEMO_USER_ENABLED` resolves to true, the account cannot be deleted through any path:

- `DELETE /api/me`
- `POST /settings/account/delete`
- `POST /account/delete`
- `DELETE /api/admin/users/{id}`

Each returns HTTP 400 with `the shared demo account cannot be deleted`. To remove the demo row, disable the feature first, then delete the account with admin tools (or leave it; it is a non-routable local address).

### Login surfaces

The web login modal and the Android login screen fetch `/api/auth/config` and show a "Use demo account" button/hint only when the demo is enabled. Credentials are never hardcoded in the clients. If the endpoint is unreachable or disabled, the button stays hidden.

For Play Console, use `demo@redchef.local` / `redchef-demo`. Keep the variables unchanged while the listing is under review. Changing `DEMO_USER_PASSWORD` updates the account on the next boot; also update the Play Console instructions when you do.

## Legal configuration

All values are environment variables (or `.env`). Missing values fall back to neutral wording; the code does not invent contact details.

| Variable | Used for | Default |
|---|---|---|
| `LEGAL_CONTROLLER_NAME` | Controller name on `/privacy` | "The operator of this RedChef instance" |
| `LEGAL_CONTACT_EMAIL` | Privacy/deletion contact and mailto links | Empty; the page points to the administrator |
| `LEGAL_CONTACT_ADDRESS` | Postal address | Empty |
| `LEGAL_EU_REPRESENTATIVE` | Art. 27 representative (name, address, email) | Empty; only set when one is appointed |
| `LEGAL_DPO_EMAIL` | DPO contact | Empty; only set when a DPO is appointed |
| `LEGAL_SUPERVISORY_AUTHORITY` | Complaint authority name/URL | "the data protection authority in your EU/EEA country of residence" |
| `LEGAL_HOSTING_REGION` | Hosting statement | "the European Union" |
| `LEGAL_BASE_URL` | Canonical public URL and demo host detection | `https://redchef.vandijke.xyz` |

`BASE_URL` is separate: it builds absolute password-reset links and is a fallback for demo host detection. Set it behind a reverse proxy when the request host is not the public host.

The privacy page publishes the controller, contact details, EU representative, DPO and supervisory authority when set, and explains lawful bases, recipients, transfers, retention, deletion, rights and cookies. The `/account/delete` page explains the public form, the signed-in form, what is removed and what may remain.

## Deletion semantics

`db.DeleteUser` runs in one transaction:

1. Refuses to delete the last admin.
2. Deletes browser sessions, legacy admin sessions, API tokens and password-reset tokens.
3. Promotes replies to the user's comments to top-level comments (`parent_id = NULL`).
4. Deletes the user's comments, favourites, tips and purchases.
5. Deletes the user row.

The demo guard runs before this in the HTTP handlers. Tips/purchases are mock iDEAL records, so there is no financial-retention exception. Uploads are not author-linked and are not deleted with the account. Backups and logs are not rewritten. Full detail, including caveats and the one-month assisted-request SLA, is in [gdpr.md](./gdpr.md#6-deletion-semantics-as-implemented).

## Backups

Backups taken before a deletion can still contain the old account until they are overwritten or expire. Deletion is not applied retroactively to backups; a restore can reintroduce deleted data. See [backup-restore.md](./backup-restore.md) for the export/restore procedure and define a rotation period in the operator's schedule.

## Verification

Public pages and config:

```bash
curl -s -o /dev/null -w '%{http_code}\n' https://redchef.vandijke.xyz/privacy
curl -s -o /dev/null -w '%{http_code}\n' https://redchef.vandijke.xyz/terms
curl -s -o /dev/null -w '%{http_code}\n' https://redchef.vandijke.xyz/account/delete
curl -s https://redchef.vandijke.xyz/api/auth/config
```

Export (session required):

```bash
curl -s -c /tmp/redchef.cookies -H 'Content-Type: application/json' \
  -d '{"email":"demo@redchef.local","password":"redchef-demo"}' \
  https://redchef.vandijke.xyz/api/auth/login

curl -s -b /tmp/redchef.cookies \
  https://redchef.vandijke.xyz/api/me/export -o /tmp/redchef-export.json
python3 -m json.tool /tmp/redchef-export.json
```

Deletion without a session (use a throwaway account, never the demo):

```bash
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' \
  -d 'email=throwaway@example.com' \
  -d 'password=throwaway-password' \
  -d 'confirm=throwaway@example.com' \
  https://redchef.vandijke.xyz/account/delete
```

Expected: `303` to `/account/delete?deleted=1`. Repeat with the demo credentials and expect HTTP 400.

## EU launch checklist

- [ ] Set all applicable `LEGAL_*` values, including `LEGAL_EU_REPRESENTATIVE`, `LEGAL_DPO_EMAIL` and `LEGAL_SUPERVISORY_AUTHORITY` where the Art. 27/37 assessments require them.
- [ ] Verify `/privacy` shows the real controller, contact, representative (if any), DPO (if any), hosting region and complaint authority — no placeholders.
- [ ] Confirm the hosting/backup region and every processor; sign DPAs and document transfer mechanisms.
- [ ] Publish `https://redchef.vandijke.xyz/privacy` in Play Console and link it in the app.
- [ ] Keep the demo account enabled only on the intended public host; put `demo@redchef.local` / `redchef-demo` in the Play Console sign-in instructions.
- [ ] Test the public deletion form with a throwaway account; confirm immediate deletion, the `?deleted=1` panel and that the demo is refused.
- [ ] Test export on web and Android; confirm profile, comments, favourites, tips and purchases are present and that password/session/token fields are absent.
- [ ] Verify Android manifest: HTTPS-only, `allowBackup=false`, data extraction rules exclude app data.
- [ ] Implement or disable UGC before Play review; there is no report/block feature yet (see [play-store.md](./play-store.md#remaining-review-risks)).
- [ ] Record the operational runbooks: breach register, DSAR log, retention schedule and backup rotation.

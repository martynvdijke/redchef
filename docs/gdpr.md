# GDPR compliance notes

Practical operator notes for the RedChef deployment at `https://redchef.vandijke.xyz`. This is an engineering runbook, not legal advice. Items marked **operator-specific** depend on where the controller is established, which suppliers are used, and the operator's own contracts.

Related pages: [legal pages and deletion](./legal.md), [backup and restore](./backup-restore.md), [API tokens](./api-tokens.md), [Google Play notes](./play-store.md).

## 1. Roles

| Role | Who | Notes |
|---|---|---|
| Controller | `LEGAL_CONTROLLER_NAME` (default: "The operator of this RedChef instance") | Decides why and how account, content and log data are processed. |
| Processor | Hosting/backup provider, SMTP provider, Gotify server, Umami instance, OpenTelemetry collector | Act on the controller's instructions. **Operator-specific:** a signed Art. 28 DPA is required for each. |
| Independent controller | The OIDC identity provider (Authelia) | Authenticates the user and decides how the identity account is managed. RedChef only receives the subject identifier, verified email and group/role information. |
| Independent controller | Google (Play distribution and review) | Receives the app, store listing and any review credentials. Not a runtime processor for RedChef data. |

There is no joint controllership in the code. **Operator-specific:** confirm this for your supplier contracts.

## 2. EU representative (Art. 27) and DPO

### Art. 27 assessment

Art. 27 requires a representative in the EU/EEA only when:

1. the controller or processor is **not established in the EU/EEA**, and
2. it processes personal data of EU/EEA data subjects in connection with offering goods/services to them or monitoring their behaviour.

If the controller is established in the EU/EEA, no Art. 27 representative is required. Hosting the server in the EU does not by itself create an establishment. The Art. 27(2) exemption can apply to occasional processing that is unlikely to result in risk and does not involve large-scale special-category data, but a public recipe/comment service with accounts is normally regular processing, so do not assume the exemption without a documented assessment.

If a representative is required:

- Appoint one in writing in a Member State where data subjects are located.
- The representative must be established in the EU/EEA and act as the contact point for data subjects and supervisory authorities. It is not a processor and not a DPO.
- Publish the representative's name and address.
- Set `LEGAL_EU_REPRESENTATIVE` to a single line containing name, postal address and email, for example:
  `Example EU Rep GmbH, Musterstraße 1, 10115 Berlin, Germany, privacy@example.eu`
- Leave the variable empty when no representative is appointed. Do not invent a representative.

### DPO

A statutory DPO is required only under Art. 37 (public authority, large-scale regular monitoring, or large-scale special-category/criminal data). A small recipe site will often not meet that threshold. If a DPO is appointed, set `LEGAL_DPO_EMAIL`; the privacy page then publishes the contact. If no DPO is appointed, leave it empty and use `LEGAL_CONTACT_EMAIL` for privacy requests. Do not label a contact as "DPO" unless the Art. 37 assessment actually supports it.

### Supervisory authority

Set `LEGAL_SUPERVISORY_AUTHORITY` to the competent authority's name and complaint URL, for example `Autoriteit Persoonsgegevens (https://www.autoriteitpersoonsgegevens.nl)`. The default wording points users to the authority in their country of residence. **Operator-specific:** determine the lead authority for your establishment.

## 3. Records of Processing Activities (Art. 30)

The Art. 30(5) small-organisation exemption does not apply when processing is regular and not occasional. Keep this record current. Fields: activity, data subjects/data, purpose, Art. 6 basis, recipients, retention, security.

| # | Activity | Data subjects and data | Purpose | Lawful basis | Recipients / processors | Retention | Security |
|---|---|---|---|---|---|---|---|
| 1 | Account registration and profile | Users; email address (also username), bcrypt password hash, role, paid/member flag, created time, OIDC subject if linked | Create and operate the account; authorise paid content | Art. 6(1)(b) contract | Hosting, backup; OIDC provider if used | Life of account; deleted with account | TLS; bcrypt; server-side sessions; least-privilege DB volume |
| 2 | Authentication, sessions and API tokens | Users; `session_token` cookie, `user_sessions.token`, legacy `sessions.token`, API token SHA-256 hash, expiry/last-used timestamps | Keep the user signed in; allow programmatic public-API access | Art. 6(1)(b); Art. 6(1)(f) for security | None outside the instance | Browser session valid 30 days; legacy admin session 24 hours; API token until revoked or account deleted | Opaque random tokens; HttpOnly/SameSite=Lax cookie; token hashes only; revocation on logout, password reset and deletion |
| 3 | Comments / user-generated content | Users; user ID, comment body, post ID, parent ID, timestamp; email/username is shown as the public author | Publish and display comments and replies | Art. 6(1)(b) | Public visitors; hosting, backup | Until comment/post deleted or account deleted | TLS; HTML escaping; comments public by design |
| 4 | Favourites | Users; user ID, post ID, timestamp | Let a user save posts; show aggregate counts | Art. 6(1)(b) | Hosting, backup; aggregate count is public | Until removed, post deleted or account deleted | Per-account access; aggregate only in public views |
| 5 | Tips and purchases (mock iDEAL) | Users; user ID, post ID, amount/price in cents, timestamp | Demonstrate the tip/unlock flow | Art. 6(1)(b) | Hosting, backup; aggregate counts public | Until post/account deleted; no financial-retention duty because no payment is processed | No bank, card or payment-provider data is collected |
| 6 | Email and notifications | Users; email address, message content, reset link | Welcome, reset, tip, purchase, subscription and operator notifications | Art. 6(1)(b) for account messages; Art. 6(1)(f) for operator alerts | SMTP provider; Gotify server | Message retention is supplier-defined; the app stores only settings, not sent messages | TLS/STARTTLS as configured by the operator; credentials in admin settings |
| 7 | Uploads / media | Users and visitors; photos/videos, filenames, thumbnails | Publish post media | Art. 6(1)(b) | Hosting, backup; public if the post is public | Until the post is deleted; **not** deleted automatically with the account because posts have no author column | Files served over HTTPS; filesystem/volume permissions |
| 8 | Server and reverse-proxy logs | Visitors and users; IP address, user agent, method, path, timestamp | Security, abuse prevention, diagnostics | Art. 6(1)(f) | Hosting/proxy provider | Operator/proxy-defined; the app writes to stdout and does not rotate logs | Access-controlled host logs; short rotation recommended |
| 9 | Analytics and telemetry (optional) | Visitors; page views via cookieless Umami; request traces/metrics/logs via OTLP | Aggregate usage and failure diagnosis | Art. 6(1)(f) | Umami instance; OTel collector | Supplier/operator-defined; no app-level retention | Self-hosted where possible; no advertising profile; no cross-site tracking |
| 10 | OIDC identity link | Users; provider subject, verified email, group/role mapping | Single sign-on and account linking | Art. 6(1)(b) | Identity provider (independent controller) | Until account deleted at RedChef; IdP account is separate | OIDC with PKCE; state/nonce; no provider tokens stored in the app |
| 11 | Backups | All of the above | Disaster recovery | Art. 6(1)(b), 6(1)(f); 6(1)(c) only where a legal retention duty applies | Backup/object-storage provider | Operator's rotation schedule; deletion is not applied retroactively | **Operator-specific:** encrypt backups at rest and in transit; restrict access |

Note on the shared demo account: it is intentionally public, holds no real person's data, and uses the non-routable default `demo@redchef.local`. Do not put personal data in it. See [legal.md](./legal.md#demo-account).

## 4. Data subject rights runbook

Intake: the self-service pages, the in-app actions, or `LEGAL_CONTACT_EMAIL` / `LEGAL_DPO_EMAIL`. Log every assisted request (date received, right, action, response date). The statutory response period is **one month** from receipt; it may be extended by **two further months** for complex or numerous requests, with notice and reasons inside the first month (Art. 12(3)). Requests are normally free; a manifestly unfounded or excessive request may be charged or refused under Art. 12(5).

| Right | Self-service | Assisted route | Identity verification |
|---|---|---|---|
| Access (Art. 15) | `GET /api/me/export` while signed in; Profile → download in the web UI; Settings → Account → **Download my data** in Android | Operator supplies the same JSON or answers specific questions | Require an authenticated session. For email-only requests, verify control of the account email before disclosing anything. |
| Rectification (Art. 16) | No profile-edit UI for email/role; ask the operator | Admin tools / database correction | Verify the requester controls the account or email. |
| Erasure (Art. 17) | `DELETE /api/me` (JSON, typed email), `POST /settings/account/delete` (web form), public `POST /account/delete`, or Android Settings → Account → Delete account | Operator deletes via admin tools if the user cannot sign in | Signed-in: typed email. Signed-out public form: email + password + typed email. Email fallback: verify control of the address. |
| Restriction (Art. 18) | No dedicated UI | Operator marks/handles the account manually and pauses processing | Verify the requester; document the restriction and its scope. |
| Portability (Art. 20) | Same JSON export as access | Operator can transmit the JSON to another controller on request where technically feasible | Same as access. |
| Objection (Art. 21) | No per-user analytics opt-out | Operator assesses the objection; for legitimate-interests processing, stop unless compelling grounds exist. Analytics/telemetry can be disabled instance-wide. | Verify the requester. |
| Automated decisions (Art. 22) | Not applicable | No automated decision-making or profiling with legal/significant effects is implemented. | — |
| Complaint (Art. 77) | Link on the privacy page | Point to `LEGAL_SUPERVISORY_AUTHORITY` or the authority in the user's country | — |

Practical rules:

- Do not ask for more identity evidence than needed. Use the existing session where possible; for email fallback, use a reply from the account address or another proportionate check.
- The public deletion form deletes immediately. Do not promise a waiting period.
- The export deliberately omits the password hash, OIDC subject, session tokens and API token hashes. Do not add them to a DSAR response.
- If a request concerns a comment that has already been replied to, deleting the account removes the comment and promotes replies; explain this before deletion.

## 5. Retention schedule

| Data | Store | Active retention | Cleanup behaviour / caveat |
|---|---|---|---|
| Account row (`users`) | SQLite `/db/redchef.db` | Life of the account | Deleted immediately by `db.DeleteUser`. |
| Browser sessions (`user_sessions`) | SQLite | Valid 30 days from creation | Expired rows are deleted lazily when presented; otherwise they remain until account deletion or manual DB maintenance. |
| Legacy admin sessions (`sessions`) | SQLite | 24 hours | Checked on use; stale rows may remain until deletion/maintenance. |
| API tokens (`api_tokens`) | SQLite | No default expiry | Until revoked by the user, rotated, or the account is deleted. |
| Password resets (`password_resets`) | SQLite | 1 hour | Single use; deleted on use, on the next reset request, or with the account. |
| Comments, favourites, tips, purchases | SQLite | Until the content/account is deleted | Removed in the deletion transaction. Tips/purchases need no financial-retention period because no payment occurs. |
| Uploaded media | `UPLOAD_DIR` (default `/app/media`) | Until the post is deleted | Not linked to the account row; account deletion does not remove uploads. |
| Server/proxy logs | Host stdout / proxy | Operator-defined | The app does not rotate logs; configure retention at the host/proxy. |
| Umami / OTel data | Operator's services | Operator-defined | Configure retention in those services. |
| Backups | Operator's storage | Operator-defined rotation | A pre-deletion backup can still contain deleted data until it expires. Do not restore an individual account from backup. |

## 6. Deletion semantics (as implemented)

All self-service paths converge on `db.DeleteUser`, which runs in a single SQLite transaction:

1. Refuse to delete the last remaining admin (`ErrLastAdmin`, HTTP 400).
2. Refuse to delete the shared demo account while the demo feature is enabled (HTTP 400 on the JSON, settings, public-form and admin paths).
3. Delete `user_sessions` and legacy `sessions` rows.
4. Delete `api_tokens` and `password_resets`.
5. Set `parent_id = NULL` on replies to the user's comments, so other people's replies survive as top-level comments.
6. Delete the user's comments, favourites, tips and purchases.
7. Delete the `users` row.
8. Clear the `session_token` cookie and, for the public form, redirect to `/account/delete?deleted=1` (see `handlers/account.go`).

Web/API entry points:

- `DELETE /api/me` — signed-in JSON; body `{"confirm":"<email>"}`.
- `POST /settings/account/delete` — signed-in browser form; redirects to `/?deleted=1`.
- `POST /account/delete` — public form. Signed out: `email`, `password` and `confirm`; signed in: `confirm` only. Success redirects to `/account/delete?deleted=1`.
- Android: Settings → Account → Delete account (calls `DELETE /api/me`).

What is **not** deleted or not immediate:

- **Uploads.** Posts have no author column, so media files are not author-linked and are not removed by account deletion. They remain until the post is deleted or the operator removes the files.
- **Backups.** Pre-deletion backups retain old rows until rotation/expiry. A restore reintroduces them; re-delete or document the restore decision.
- **Logs.** Host/proxy access logs rotate on the operator's schedule.
- **Identity provider.** The Authelia/OIDC account is separate and is not deleted by RedChef.
- **Caches/copies.** Comments are public, so search-engine caches, screenshots, other users' copies and exports may persist beyond deletion.
- **Demo account.** Cannot be deleted while enabled; disable `DEMO_USER_ENABLED` first if the operator really needs to remove the row.

The live database deletion is immediate. "Within one month" applies only to assisted requests that the operator must handle manually, not to the self-service flows.

## 7. Breach response runbook

A personal-data breach is a security incident leading to accidental or unlawful destruction, loss, alteration, unauthorised disclosure of, or access to personal data. When in doubt, assess and record; do not wait for certainty.

| Time | Action | Owner |
|---|---|---|
| Immediately | Contain: revoke sessions/tokens, rotate secrets, isolate the host, preserve evidence. Record what happened. | Operator / on-call |
| Within 24 hours (internal target) | Assess likelihood and severity of risk to rights and freedoms; decide whether Art. 33 notification is required. Identify affected data subjects and categories. | Controller (with DPO/rep if appointed) |
| **Within 72 hours of awareness** | If the breach is likely to result in a risk, notify the competent supervisory authority (`LEGAL_SUPERVISORY_AUTHORITY`) unless it is unlikely to result in a risk. If notification is late, include the reasons. | Controller |
| Without undue delay | If the breach is likely to result in **high risk**, notify affected users in clear, plain language (Art. 34). If not feasible individually, use a public notice. | Controller |
| Without undue delay | Notify processors/controllers in the chain. A processor must inform the controller without undue delay (Art. 33(2)); the controller's 72-hour clock runs from its own awareness. | Processor / controller |
| After containment | Complete the internal breach register even if no notification was made (Art. 33(5)); update security measures and this runbook. | Controller |

Art. 33(3) notification content: nature of the breach (categories and approximate numbers), DPO/representative/contact point, likely consequences, and measures taken/proposed. Do not include unnecessary personal data in the notification.

Breach register fields: date/time of awareness and discovery, description, cause, data categories and approximate number of subjects/records, systems affected, containment, risk assessment, whether the SA and users were notified (with dates), remediation and owner.

Useful contacts to fill in before launch:

- Controller: `LEGAL_CONTROLLER_NAME`, `LEGAL_CONTACT_EMAIL`, `LEGAL_CONTACT_ADDRESS`.
- EU representative: `LEGAL_EU_REPRESENTATIVE`.
- DPO (if appointed): `LEGAL_DPO_EMAIL`.
- Supervisory authority: `LEGAL_SUPERVISORY_AUTHORITY`.
- Hosting and backup provider incident contacts: **operator-specific**.
- SMTP / Gotify / OTel / Umami supplier contacts: **operator-specific**.

## 8. Processors, DPAs and international transfers

| Supplier / service | Role | Data seen | Location | Action |
|---|---|---|---|---|
| Hosting and backup provider | Processor | Database, media, backups, logs | **Operator-specific** (default assumption: EU) | Sign DPA; list subprocessors; agree deletion/return and breach notice; encrypt backups. |
| SMTP provider | Processor | Email address, message content, reset links | **Operator-specific** | Sign DPA; check retention and whether content is scanned. |
| Authelia / OIDC provider | Independent controller (or same operator if self-hosted) | Identity account, email, groups, authentication events | **Operator-specific** | Confirm role in the privacy notice; no DPA if it is genuinely an independent controller. |
| Gotify server | Processor | Operator notification title/message; may include email/activity | **Operator-specific** | Sign DPA if operated by a third party; self-host where possible. |
| Umami | Processor | Page views, referrer, user agent; cookieless by design | **Operator-specific** | Sign DPA; assess ePrivacy consent requirements for your jurisdiction. |
| OpenTelemetry collector | Processor | Request paths, timings, service logs; may include IP/user agent depending on exporter/proxy | **Operator-specific** | Sign DPA; configure scrubbing and retention; do not export request bodies or secrets. |
| Google Play | Independent controller for distribution/review | App bundle, store listing, review account credentials and data | Google's locations | Not a runtime processor; keep the Data safety form and deletion URL accurate. |

DPA checklist: Art. 28 written contract; subject matter/duration/nature/purpose; confidentiality; security (Art. 32); subprocessor authorisation, list and notice; assistance with data-subject rights and breach notification; deletion/return at end; audit rights; breach notification without undue delay; transfer mechanism.

International transfers:

- The default deployment region is the EU (`LEGAL_HOSTING_REGION`, default "the European Union").
- If any supplier processes outside the EEA, use an adequacy decision or the 2021/914 SCCs (normally module 2 for controller-to-processor) plus a transfer impact assessment and any supplementary measures.
- **Operator-specific:** document the actual region of the host, backup bucket, SMTP relay, Gotify, Umami and OTel collector. Do not claim EU-only hosting unless the supplier contract confirms it.
- Google Play's own processing is covered by Google's terms, not by the RedChef DPA.

## 9. Security measures

Implemented in this codebase:

- HTTPS in production; the app itself serves HTTP inside the container and expects a TLS-terminating reverse proxy. Session cookies are marked `Secure` when the request is TLS or `X-Forwarded-Proto: https`.
- Password hashes use bcrypt at the current default cost. Legacy SHA-256 hashes are rehashed transparently after a successful login.
- The session cookie is `HttpOnly` and `SameSite=Lax`, with a 30-day lifetime; logout deletes the server-side session.
- Session tokens are random and server-side; API tokens are stored only as SHA-256 hashes; password-reset tokens are hashed, single-use and expire after one hour; a password reset revokes all browser sessions.
- Account deletion is transactional and revokes credentials first.
- The last admin cannot be deleted; the shared demo account cannot be deleted or promoted to admin.
- Admin routes require the admin role; API mutations require a session and a same-origin check; cross-origin browser access is denied unless `CORS_ALLOWED_ORIGINS` is set.
- Request logging records method and path, not bodies or passwords.

Operator responsibilities / known gaps:

- Enable TLS and redirect HTTP to HTTPS at the proxy; the app does not set HSTS itself.
- Encrypt the DB volume, media volume and backups at rest. The built-in backup export is a zip; protect it in transit and at rest.
- Restrict filesystem access to `DB_PATH` and `UPLOAD_DIR`; run the container as a non-root user where possible.
- There is no application-level MFA or rate limiting. Use the identity provider, reverse proxy or host controls where needed.
- There is no scheduled cleanup job for expired session/reset rows; they are cleaned lazily or with the account. Add a maintenance job if your retention policy requires it.
- Review admin accounts and API tokens regularly; remove unused ones.

## 10. Children, automated decisions, advertising

- **Children:** the service is for users aged **16 or over**. It is not directed at children and does not knowingly create child accounts. There is no age-verification mechanism; if an under-16 account is discovered, delete it. Do not rely on parental consent.
- **Automated decision-making:** none. No profiling with legal or similarly significant effects is implemented (Art. 22 not engaged).
- **Advertising and sale:** no advertising, no ad SDKs, no sale/rental of personal data, no cross-site tracking. Analytics, when enabled, is self-hosted and cookieless.
- **Mock payments:** tips and purchases are mock iDEAL records. No bank, card or payment-provider data is collected and no money is charged.

## 11. What is still missing

| Gap | Why it matters | Suggested action |
|---|---|---|
| No UGC report/block/moderation feature | Google Play's UGC policy expects reporting, blocking and moderation for apps with user-generated content; the terms prohibit abuse but there is no in-product control. | Treat as a release blocker for Play if UGC is enabled; implement report/block and a moderation queue, or disable comments for the Play build. |
| Formal DPAs and subprocessor register | Art. 28 requires a written contract with each processor; the repo lists likely processors but cannot sign contracts. | Collect signed DPAs, record regions and subprocessors, and link them from the privacy page. |
| EU representative appointment (if Art. 27 applies) | The code supports `LEGAL_EU_REPRESENTATIVE`, but only the operator can appoint and mandate a representative. | Complete the Art. 27 assessment and fill the variable, or document why it is not required. |
| DPO appointment (if Art. 37 applies) | `LEGAL_DPO_EMAIL` only publishes a contact; it does not create a DPO. | Complete the Art. 37 assessment; appoint and train a DPO if required. |
| Retention automation | Expired session/reset rows can outlive their validity until lazy cleanup or account deletion. | Add a scheduled cleanup job or document manual maintenance. |
| Per-user analytics objection | There is no self-service opt-out; objection must be handled manually or by disabling analytics instance-wide. | Add a preference or document the manual process. |
| Encryption at rest / secure backup handling | The app does not encrypt the DB, media or backup zip. | Use encrypted volumes and encrypted backup storage; test restore. |
| Age assurance | The service is 16+ but has no age gate. | Keep the terms/privacy statement; add an age confirmation if the risk assessment requires it. |

## 12. Operator configuration checklist

| Variable | Purpose | Default / note |
|---|---|---|
| `LEGAL_CONTROLLER_NAME` | Controller identity on `/privacy` | Falls back to "The operator of this RedChef instance". |
| `LEGAL_CONTACT_EMAIL` | Privacy and deletion contact | Empty means the page points to the administrator. |
| `LEGAL_CONTACT_ADDRESS` | Postal address | Optional. |
| `LEGAL_EU_REPRESENTATIVE` | Art. 27 representative | Empty unless appointed; include name, address and email. |
| `LEGAL_DPO_EMAIL` | DPO contact | Empty unless a DPO is appointed. |
| `LEGAL_SUPERVISORY_AUTHORITY` | Complaint authority | Default points to the user's national authority. |
| `LEGAL_HOSTING_REGION` | Hosting statement | Default "the European Union"; must match reality. |
| `LEGAL_BASE_URL` | Public base URL | Default `https://redchef.vandijke.xyz`; also used for demo auto-enable. |
| `BASE_URL` | Absolute links (password reset) and demo fallback host | Optional; set behind a proxy if the request host is not public. |

Before publishing, verify on the live instance:

```bash
curl -s https://redchef.vandijke.xyz/privacy | grep -E 'Controller|representative|DPO|supervisory'
curl -s https://redchef.vandijke.xyz/account/delete | grep -E 'Delete my account|deleted'
curl -s https://redchef.vandijke.xyz/api/auth/config
```

Do not enable the demo account on a private instance unless you intend to publish its credentials. The endpoint returns the password by design when the demo is enabled (see [legal.md](./legal.md#demo-account)).

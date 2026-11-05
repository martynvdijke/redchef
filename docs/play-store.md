# Google Play submission notes

Operational checklist for the Android listing. The privacy and deletion behaviour is implemented by the server, so these answers must match the deployed instance at `https://redchef.vandijke.xyz`.

## App identity

- Package: `xyz.vandijke.redchef`
- Privacy policy URL: https://redchef.vandijke.xyz/privacy
- Account deletion URL: https://redchef.vandijke.xyz/account/delete
- Target audience: 16+; not child-directed
- Ads: none
- Real-money purchases: none (tips/unlocks are mock records)

## Sign-in details for Play review

Play Console → **App content → App access** needs working credentials because comments, favourites, tips and account screens require a session. Public posts and the legal pages are readable without one.

Use the shared demo account:

| Field | Value |
|---|---|
| Email | `demo@redchef.local` |
| Password | `redchef-demo` |

Instructions to give the reviewer:

1. Open the app (default server `https://redchef.vandijke.xyz`).
2. On the login screen, tap **Use demo account**; the web login modal has the same button. Alternatively enter the credentials above.
3. All paid content is unlocked on this account.
4. Do not delete the account: it is protected and deletion returns HTTP 400.

Notes for the operator:

- The demo is seeded/repaired on boot when `DEMO_USER_ENABLED` is true (auto-enabled on the `redchef.vandijke.xyz` host). See [legal.md](./legal.md#demo-account).
- The account is **read-write and paid**, but never an admin. Reviewer comments/favourites become visible to other visitors; keep the instance clean.
- Keep the credentials valid for the whole review. Changing `DEMO_USER_PASSWORD` resets the account on the next boot; update Play Console at the same time.
- To test deletion, create a second throwaway account (register with any email you control) and delete that instead of the demo.
- Two-account testing for UGC: sign in as the demo and post a comment, then register/sign in as a second account and reply to it. This verifies that comments are public, replies work, and deletion promotes replies. It also demonstrates the current gap: there is no report/block control.

## Data safety answers

### Data types

| Play Console data type | Collected | Shared | Purpose | Notes |
|---|---|---|---|---|
| Personal info → Email address | Yes | No, except processors | Account management, authentication, notifications | Email is also the username and the public comment author name. |
| Personal info → User IDs | Yes | No, except processors | Account and session linkage | Internal numeric user ID; no advertising ID. |
| App activity → Other user-generated content | Yes | Public by design | Comments and replies | Comments are visible without signing in. |
| App activity → Other in-app actions | Yes | No | Favourites, tips and purchases | Tips/purchases are mock; no money moves. |
| Photos and videos → Photos / Videos | Yes | Public if posted | User uploads attached to posts | Posts have no author column; uploads are not linked to the account row. |
| Financial info → Purchase history | Yes (mock records only) | No | Tip/unlock demonstration | No card, bank or payment-provider data; no real transaction. |
| Device or other IDs | No | No | — | No advertising ID or device identifier is collected by the app. |
| Location, contacts, calendar, SMS, call logs, health, web browsing | No | No | — | Not requested or collected. |
| Diagnostics / crash logs | No app SDK | No | — | Server-side OTel logging is optional and operator-controlled; do not declare an app crash SDK. |

### Security and handling questions

| Question | Answer |
|---|---|
| Is any collected data encrypted in transit? | Yes. HTTPS only; Android `usesCleartextTraffic=false`. |
| Do you provide a way for users to request data deletion? | Yes. In-app Settings → Account → Delete account, plus the public web form at the URL above. |
| Is data used for advertising or marketing? | No. No ad SDKs, no advertising ID, no cross-site tracking. |
| Is data sold? | No. |
| Is data shared with third parties? | Not for ads or sale. Hosting, SMTP, Gotify, Umami and OTel process data as service providers; disclose them if Play's definition of "shared" requires it. |
| Are cookies or similar technologies used? | Yes: one `session_token` cookie after sign-in. It maps to the user ID and app activity (authentication/session), is `HttpOnly` and `SameSite=Lax`, and is not used for ads. |
| Is account creation required? | Only for interactive features. Public posts, privacy and deletion pages work without an account. |
| Can users request a copy of their data? | Yes: Profile download on web, `GET /api/me/export`, and **Download my data** in the Android account screen. |
| Is the app child-directed or in the Families programme? | No. Target audience is 16+. |

If the deployment enables Umami or OTel, confirm whether the app itself sends any telemetry (it does not; the server does) and declare server-side logs/diagnostics consistently with the privacy policy. Do not declare data the app does not collect.

## Account deletion page

`https://redchef.vandijke.xyz/account/delete` is public and requires no session.

- **Signed out:** the form asks for the account email, current password, and a typed repeat of the email. On success the account is deleted immediately and the browser is redirected to `/account/delete?deleted=1`, which shows a confirmation panel.
- **Signed in:** the form only asks for the typed email; the password is ignored.
- **What is deleted:** account row, password hash and profile fields; all browser sessions, legacy admin sessions, API tokens and password-reset tokens; comments, favourites, tips and purchases. Replies to the user's comments are promoted to top-level comments.
- **What remains:** uploads (posts have no author link), pre-deletion backups until rotation, host/proxy logs, public caches/copies, and the separate OIDC/Authelia account.
- **Timing:** live deletion is immediate. There is no waiting period. Assisted requests handled by email are answered within one month at the latest.
- **Protected accounts:** the last admin and the shared demo account cannot be deleted (HTTP 400).

For the Play Console deletion URL, use the public URL above and describe both the in-app and web paths. The page itself explains the retention caveats.

## Permissions and Android manifest

- `INTERNET`
- `CAMERA` (runtime permission, used for uploads)

Manifest expectations verified in `mobile/android/app/src/main/AndroidManifest.xml` and the data-extraction rules:

- `usesCleartextTraffic=false`
- `allowBackup=false`
- `fullBackupContent=false`
- `dataExtractionRules` excludes all domains

Re-check these after any Flutter/Android dependency upgrade.

## Content rating and target audience

- Complete the IARC questionnaire for user-generated content, user uploads and mock purchases.
- Target audience: **16+**. The service is not designed for children and has no child-directed content.
- The app contains public comments and user-uploaded photos/videos; moderation is currently limited to admin deletion of comments/posts.
- Mock tips/purchases are not real-money transactions and do not use Google Play Billing. Do not describe them as purchases in the store listing; label the feature as a demo if it is mentioned.

## Remaining review risks

| Risk | Status / action |
|---|---|
| **UGC report/block/moderation** | **Not implemented.** Google Play's UGC policy expects users to report content and block other users, and expects moderation. The terms prohibit abuse and admins can delete comments/posts, but there is no in-product report or block control. Treat this as a release blocker for a UGC-enabled build; implement report/block and a moderation workflow, or disable comments/UGC in the Play build. |
| Content rating | UGC and uploads can trigger a higher rating. Complete the questionnaire accurately and keep the listing consistent with the 16+ audience. |
| Target audience 16+ | Do not opt into the Families programme. Remove any child-oriented wording/screenshots. |
| Mock payments | Ensure no real charge, no card fields and no Play Billing dependency. Describe tips/unlocks as mock if mentioned. |
| Demo account validity | Reviewers must be able to sign in. The demo is auto-enabled only on the official host; if the build points elsewhere, set `DEMO_USER_ENABLED=true` there and verify `/api/auth/config`. |
| Data safety accuracy | Keep the answers above aligned with the deployed privacy policy. If analytics/OTel or IP logging changes, update the form. |
| Public deletion URL | Must stay reachable without login and must not be moved behind auth. Test it from a logged-out browser before every submission. |
| Privacy policy live | The URL must return the policy, including controller/contact, retention, rights and deletion. Do not publish placeholder `LEGAL_*` values. |

## CI and release notes

- `.github/workflows/ci.yaml` runs Go build/tests, Playwright E2E and coverage.
- `.github/workflows/flutter-android.yaml` runs `flutter analyze`, `flutter test` and a release APK build.
- `.github/workflows/android-e2e.yaml` runs integration tests on an emulator.
- `.github/workflows/release.yaml` runs semantic-release, builds/pushes Docker images, attaches a release APK to the GitHub release, and calls the Play workflow.
- `.github/workflows/play-store.yaml` builds a signed AAB and uploads it. It skips publishing with a warning when `ANDROID_KEYSTORE_BASE64` or `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` is missing. It uses `ANDROID_PACKAGE_NAME` (default `xyz.vandijke.redchef`), `GOOGLE_PLAY_TRACK` (default `internal`) and `GOOGLE_PLAY_RELEASE_STATUS` (default `completed`).
- Signing secrets: `ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD`. Never commit the keystore or `mobile/android/key.properties`.
- Version name comes from the semantic-release version; the version code defaults to the git commit count unless a build number is supplied.

Before tagging a release:

- [ ] `go test ./...`, `npx playwright test`, `flutter analyze`, `flutter test` pass.
- [ ] `/privacy`, `/terms`, `/account/delete` and `/api/auth/config` return the expected responses on the target host.
- [ ] Demo sign-in works on a clean install and the credentials in Play Console match.
- [ ] Public deletion and export have been tested with a throwaway account.
- [ ] Android manifest checks still hold after dependency updates.
- [ ] The UGC gap is either fixed or the Play build has UGC disabled.

# RedChef — Flutter Android App

Native Flutter Android client for the RedChef Go backend (`xyz.vandijke.redchef`).

## Run

```bash
cd mobile
flutter pub get
flutter run                # needs Android emulator or device
flutter analyze            # must be clean
flutter test               # hermetic, mocked network
flutter build apk --release   # CI does this; requires Android SDK
```

Optional local testing: with an Android emulator, the host's `http://localhost:8080` is reachable as `http://10.0.2.2:8080` from inside the emulator. This is not a default — enter your own server address on first launch.
Physical device: use your LAN IP, e.g. `http://192.168.1.10:8080`.

## Configure

1. On first launch, enter the **Server URL** (base URL without trailing slash). Validated via `GET /api/setup/status` (public) — shows a clear error if the URL isn't a RedChef server (e.g. 404 on `/api/setup/status` + `/api/auth/me`).
2. Log in / Register, or continue as guest to browse.
3. If the server has OIDC enabled (`GET /api/auth/oidc/status → {enabled:true}`), a **Continue with SSO** button appears and opens `/api/auth/oidc/login` in an **in-app WebView** (no external browser). Sign in with your identity provider there; when the flow returns to the RedChef origin the app captures the server's `session_token` cookie and continues automatically. Server error redirects (`/?error=…`) are shown inline with a Retry option.

Auth is **session cookie** `session_token` (`HttpOnly`, `SameSite=Lax`). The app captures `Set-Cookie` from login/register and sends `Cookie: session_token=…` via a Dio interceptor, persisting to `flutter_secure_storage`. For SSO, the `HttpOnly` cookie is read from the native WebView cookie store over a small platform channel, and only when the current WebView URL is same-origin as the configured server. The server URL is also persisted there.

## Features

- **Feed** (`GET /api/posts?limit&offset&tag&q&type&sort…`, `X-Total-Count` header) — 2-column grid with `cached_network_image`, pull-to-refresh, infinite scroll, skeleton/empty/error states. Tag filter bar from `GET /api/tags`, search via `q`. Favourite toggle is optimistic and uses `POST /api/posts/{id}/favourite` when not favourited and `DELETE /api/favourites/{id}` when already favourited.
- **Detail** (`GET /api/posts/{id}`) — media (prepend base URL to `media_url` relative path `/uploads/…`), favourite toggle (as above), **Tip** affordance (`POST /api/posts/{id}/tip` with preset amounts €1/€3/€5/€10, reflects `tip_count` & `formatted`), comments (`GET/POST /api/posts/{id}/comments`), recipe (ingredients/steps) when `unlocked`, and unlock flow (`POST /api/pay/item` per item, `POST /api/pay/unlock` subscription).
- **Favourites** (`GET /api/favourites`) — 401 if anonymous.
- **Composer** — **any authenticated user** can publish via `POST /api/posts` (multipart: `file` required, `title`, `description`, `locked` ("true"), `recipe` JSON string, `tags` comma-separated, `status`, `scheduled_at`) → 201 `{id,title,media_type,locked,created_at,processing,message}`. Editing an existing post remains admin-only via `PATCH /api/admin/posts/{id}` + `PUT /api/admin/posts/{id}/media` (shown only to `role == "admin"`).
- **Forgot / Reset password** — login screen links to `POST /api/auth/forgot` `{email}` (always 200, shows `message`) and `POST /api/auth/reset` `{token,password,confirm_password}` (manual token entry; deep link out of scope).
- **Admin posts** (`GET /api/admin/posts?status=draft|published`, `DELETE /api/admin/posts/{id}`) — admin-only screen with status filter and delete confirmation.
- **API tokens** (`GET /api/tokens`, `POST /api/tokens` `{name}` → show `token` once, `DELETE /api/tokens/{id}`, `POST /api/tokens/{id}/rotate`) — list/create/revoke/rotate with copy.
- **Account / Settings** — profile (email/role/server), links to Admin posts, API tokens, Change server, and **Logout** (local `POST /api/auth/logout` + if OIDC enabled open `/api/auth/oidc/logout` via `url_launcher` to clear IdP session, plus clear the in-app WebView cookie store; otherwise local logout still works).
- **Onboarding validation** — `GET /api/setup/status` checked first; fallback to `/api/auth/me`; clear error if not a RedChef server.

## Navigation

- `/` feed, `/post/:id` detail (tip + favourite + comments), `/composer` (publish/edit), `/favourites`, `/login` (→ `/forgot`, `/reset`), `/register`, `/onboarding`, `/account`, `/admin/posts` (admin gated), `/tokens` (authenticated). Surfaced via AppBar popup menu (Account, Admin posts, API tokens, Change server, Log out) and login/forgot/reset links, detail tip bar, and FAB for New post (any authenticated user).

## Stack

- `flutter_riverpod` (state), `dio` (HTTP + cookie interceptor), `go_router` (navigation), `cached_network_image`, `image_picker`, `flutter_secure_storage`, `url_launcher`, `intl`.
- Material 3, light/dark themes, responsive grid.

## Project

- `lib/main.dart` — ProviderScope + GoRouter bootstrap
- `lib/src/api/` — `ApiClient` (dio + cookie persistence), `models.dart` (Post, TipResult, ApiToken, CreatePostResponse, SetupStatus…)
- `lib/src/auth/` — `AuthNotifier` + secure storage (covers forgot/reset, tips, admin, tokens, OIDC logout, setup status)
- `lib/src/screens/` — onboarding, login/register, forgot/reset, feed, detail (tip), composer (POST /api/posts), favourites, admin_posts, tokens, account
- `lib/src/routing/router.dart` — `GoRouter` with onboarding redirect
- `lib/src/` — screens and helpers; `utils/oidc.dart` holds same-origin/error-code helpers for the in-app SSO flow
- `test/` — `models_test.dart`, `auth_test.dart`, `widget_test.dart`, `oidc_test.dart`, `onboarding_test.dart` (mocked Dio, hermetic; includes TipResult/ApiToken parsing + forgot/reset/account smoke)

## CI

`.github/workflows/flutter-android.yaml` runs `flutter analyze`, `flutter test`, `flutter build apk --release` in `mobile/` (non-blocking, `continue-on-error: true`).

## Troubleshooting

### `flutter build apk` fails at the `jni` CMake configure step

Symptom:

```
Execution failed for task ':jni:configureCMakeRelWithDebInfo[arm64-v8a]'.
C/C++: CMake Error at .../CMakeDetermineCCompiler.cmake:226 (configure_file):
  No such file or directory
```

with an empty `CMakeError.log` compiler check and no compiler output.

Cause: the `jni` package (pulled in transitively by `path_provider_android`) keeps its CMake build directory inside the **shared** Dart pub cache (`$PUB_CACHE/hosted/pub.dev/jni-1.1.0/android/.cxx/…`). If another Flutter project on the same machine builds the same `jni` version at the same time, its Gradle run deletes/rewrites that directory mid-configure and this build fails. It is not an NDK/CMake/JDK problem — the toolchain works fine on its own.

Fix: give this project its own pub cache so the `.cxx` path is not shared. The cache must live outside the repo.

```bash
export PUB_CACHE="$HOME/.pub-cache-redchef"
cd mobile
flutter clean
flutter pub get
flutter build apk --release
```

`PUB_CACHE` is an environment variable, not a Flutter setting — export it in every shell that builds. `.dart_tool/` and `build/` are gitignored, so this leaves no repo changes and `pubspec.lock` is unchanged (locked versions are reused).

### Android toolchain requirements

- **JDK 17** is required; the default `java` on some hosts is JDK 11 and fails. Set `JAVA_HOME` to a JDK 17 install (e.g. `/usr/lib/jvm/java-17-openjdk`).
- Export `ANDROID_HOME`/`ANDROID_SDK_ROOT` if unset, and accept licenses once: `yes | flutter doctor --android-licenses`.
- The build only targets `arm64-v8a`, `armeabi-v7a`, `x86_64`; NDK r28 no longer supports the legacy `armeabi` (ARMv5) ABI.

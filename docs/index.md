# redchef

Documentation for **redchef**.

See the [project README](https://github.com/martynvdijke/redchef#readme) for an overview.

## OIDC / Authelia SSO

RedChef supports OIDC login via Authelia (Authorization Code + PKCE S256).

**Enabling:**
Set `OIDC_ENABLED=true` and configure `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET_FILE` (or `OIDC_CLIENT_SECRET`), `OIDC_REDIRECT_URL`, `OIDC_SCOPES`. See `.env.example` for defaults.

**Rollback:**
Set `OIDC_ENABLED=false` (or unset) — OIDC routes return 404/disabled and password login remains available.

**Authelia client registration** (on the Authelia host):
- `client_id: redchef`
- `redirect_uris: https://redchef.vandijke.xyz/api/auth/oidc/callback`
- `scopes: openid email profile groups`
- `pkce: S256` (required)
- `consent_mode: explicit`
- `authorization_policy: one_factor`

**NPM forward-auth bypass (required):**
In Nginx Proxy Manager for `redchef.vandijke.xyz`, bypass forward-auth for the OIDC callback path (`/api/auth/oidc/callback`) and ideally `/api/auth/oidc/*`; otherwise the callback request is intercepted and OIDC login breaks.

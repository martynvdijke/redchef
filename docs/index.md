# redchef

Documentation for **redchef**.

See the [project README](https://github.com/martynvdijke/redchef#readme) for an overview.

## OIDC / Authelia SSO

RedChef can authenticate via OIDC against Authelia (`https://authelia.vandijke.xyz`) using Authorization Code + **PKCE S256**. Password login stays available as a fallback and as the rollback path.

### 1. Register the Authelia client (host)

Generate a plain secret (for RedChef) and its hash (for Authelia):

```bash
# plain secret — goes to RedChef only (never into git)
docker exec authelia authelia crypto rand --length 72 --charset rfc3986
printf '%s' '<plain-secret>' > /config/authelia/secrets/oidc/redchef.secret

# hashed secret — goes into Authelia configuration.yml
docker exec authelia authelia crypto hash generate pbkdf2 --variant sha512 \
  --random --random.length 72 --random.charset rfc3986
```

Add to `/config/authelia/configuration.yml` under `identity_providers.oidc.clients`:

```yaml
      - client_id: redchef
        client_name: RedChef
        client_secret: '$pbkdf2-sha512$...'   # from the hash command above
        public: false
        authorization_policy: one_factor
        require_pkce: true
        token_endpoint_auth_method: client_secret_post
        consent_mode: explicit
        redirect_uris:
          - https://redchef.vandijke.xyz/api/auth/oidc/callback
        scopes:
          - openid
          - email
          - profile
          - groups
```

Restart Authelia, then confirm discovery through the proxy:

```bash
curl -s https://authelia.vandijke.xyz/.well-known/openid-configuration \
  | grep -o '"issuer":"[^"]*"'
```

### 2. Configure RedChef (env)

```dotenv
OIDC_ENABLED=true
OIDC_ISSUER_URL=https://authelia.vandijke.xyz
OIDC_CLIENT_ID=redchef
OIDC_CLIENT_SECRET_FILE=/run/secrets/oidc_client_secret
OIDC_REDIRECT_URL=https://redchef.vandijke.xyz/api/auth/oidc/callback
OIDC_SCOPES=openid email profile groups
# OIDC_LOGOUT_URL=   # optional; defaults to Authelia end-session
```

`OIDC_CLIENT_SECRET_FILE` takes precedence over `OIDC_CLIENT_SECRET`; mount the plain secret as a file or Docker secret. Never commit the secret. The full set of variables is listed in `.env.example`.

### 3. NPM forward-auth bypass (required)

In Nginx Proxy Manager for `redchef.vandijke.xyz`, bypass forward-auth for `/api/auth/oidc/*` (at minimum `/api/auth/oidc/callback`). If the Authelia access rule intercepts the callback, OIDC login breaks.

### 4. Verify

```bash
# public reads stay anonymous
curl -s -o /dev/null -w '%{http_code}\n' https://redchef.vandijke.xyz/            # 200
curl -s -o /dev/null -w '%{http_code}\n' https://redchef.vandijke.xyz/api/posts   # 200
curl -s -o /dev/null -w '%{http_code}\n' https://redchef.vandijke.xyz/api/posts/1 # 200
# unauthenticated private mutation is rejected for API clients (browsers get a 302 to OIDC)
curl -s -o /dev/null -w '%{http_code}\n' -X POST https://redchef.vandijke.xyz/api/posts # 401
# OIDC availability
curl -s https://redchef.vandijke.xyz/api/auth/oidc/status
```

Then open the site → **Login with Authelia** and confirm: login succeeds; an existing account links by verified email (new emails are auto-provisioned); the `admins` group grants admin; the session cookie works for subsequent API calls; and logout clears the local session and redirects to Authelia.

### Rollback

Set `OIDC_ENABLED=false` (or unset) — OIDC routes return 404/disabled and password login remains available.

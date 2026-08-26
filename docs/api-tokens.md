# API tokens (public API)

API tokens are **only for the public API** — programmatic access from scripts, bots, and external apps.

In-app actions (comments, favourites, tips, paywall unlock/item) require only a logged-in session (cookie). No bearer token is needed when using the website.

Public reads (`GET /api/posts`, `/api/trmnl/latest`, feeds, …) stay open and do not require a token either, but tokens are available for authenticated public-API integrations that need to act as a user.

## Creating a token

In the web UI, sign in, open **Profile** (top nav), scroll to **Public API Access**, enter a name and press **Generate Token**.
The secret (`rc_…`) is shown **once** — copy it immediately; only a SHA-256 hash is stored server-side.

Or via the API (session cookie required):

```sh
curl -X POST https://your-instance/api/tokens \
  -H 'Content-Type: application/json' \
  -b 'session_token=...' \
  -d '{"name":"my-script"}'
```

## Using a token

Add the token as a bearer for public-API requests that support it:

```sh
curl https://your-instance/api/posts \
  -H 'Authorization: Bearer rc_...'
```

Tokens are owned by the session user; mismatched, expired, revoked, or unknown tokens return `401 {"error":"valid API token required"}`.

## Managing tokens

| Action  | Endpoint                     | Notes                          |
|---------|------------------------------|--------------------------------|
| List    | `GET /api/tokens`            | metadata only, never secrets   |
| Revoke  | `DELETE /api/tokens/{id}`    | immediate                      |
| Rotate  | `POST /api/tokens/{id}/rotate` | revokes old, returns new secret once |

## Notes

- Never commit real secrets — use environment variables or secret managers.
- The `RequireMutationToken` middleware remains available for securing future public-API mutation endpoints, but is no longer enforced for in-app `POST /api/posts/*` and `POST /api/pay/*` routes.

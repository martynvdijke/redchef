# API tokens

Mutations (comments, favourites, tips, paywall unlock/item) require **two** credentials:

1. a logged-in session (cookie), and
2. a bearer API token owned by the session user.

Public reads (`GET /api/posts`, `/api/trmnl/latest`, feeds, …) stay open.

## Creating a token

In the web UI, open the **API tokens** section while logged in, enter a name, and press create.
The secret (`rc_…`) is shown **once** — copy it immediately; only a SHA-256 hash is stored server-side.

Or via the API (session cookie required):

```sh
curl -X POST https://your-instance/api/tokens \
  -H 'Content-Type: application/json' \
  -b 'session_token=...' \
  -d '{"name":"my-script"}'
```

## Using a token

```sh
curl -X POST https://your-instance/api/posts/42/favourite \
  -b 'session_token=...' \
  -H 'Authorization: Bearer rc_...'
```

The token must belong to the session user; mismatched, expired, revoked, or unknown tokens all
return the same `401 {"error":"valid API token required"}`.

## Managing tokens

| Action  | Endpoint                     | Notes                          |
|---------|------------------------------|--------------------------------|
| List    | `GET /api/tokens`            | metadata only, never secrets   |
| Revoke  | `DELETE /api/tokens/{id}`    | immediate                      |
| Rotate  | `POST /api/tokens/{id}/rotate` | revokes old, returns new secret once |

## Client migration

Existing scripts that mutated via session cookie alone will now receive `401`. Create one token per
client, store it in the client's secret store, and send the `Authorization` header on mutations.
Never commit real secrets — use environment variables or secret managers.

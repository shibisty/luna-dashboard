# luna-dashboard

The Luna developer console: developers sign up, register their applications and create API
keys with scopes for them. It is the only service that knows keys; luna-auth and luna-pages ask
it about each key through [luna-keys](../luna-keys).

A [fullstack](https://github.com/shibisty/fullstack.go) application (`gtr new --template
fullstack`) on Postgres: pages in `resources/views`, routes in `routes/`, handlers in
`app/http/controllers`, migrations in `database/migrations`. Developers' sessions are in the
database (`passport_sessions`, passport-session-sql).

State: stage 4 of the migration plan. Developers sign up and log in (sessions in the database),
create applications (`/apps`) and their keys: server keys (`sk_live_…`, any scope, optional
network allow-list) and browser keys (`pk_live_…`, allowed origins, `auth:login` only). A key's
secret is shown once (`Cache-Control: no-store`); afterwards only its hint. Keys can get a new
secret or be revoked; services notice within their cache time (60 s). The introspection endpoint
answers with the key's application, kind, scopes, origins and networks and records the key's last
use (at most once a minute). Lists are stored as JSONB (`models.StringList`).

| Page | |
|---|---|
| `GET /apps`, `POST /apps` | the developer's applications, a new one |
| `GET /apps/{app}`, `POST /apps/{app}`, `POST /apps/{app}/delete` | keys and settings (name, password reset URL) |
| `GET /apps/{app}/keys/new`, `POST /apps/{app}/keys` | a new key; the secret page |
| `POST /apps/{app}/keys/{key}/regenerate`, `.../revoke` | a new secret; revoke |

## Run

From the workspace root (`luna/`): `gtr install`. Then here:

```bash
cp .env.example .env     # DATABASE_DSN, LUNA_SERVICE_TOKENS
gtr dev                  # http://localhost:8080, restarts on changes
go run . migrate         # in production; DB_MIGRATE=true does it at start locally
```

| Variable | |
|---|---|
| `DATABASE_DSN` | Postgres, e.g. `postgres://dashboard_user:…@postgres:5432/dashboard_db` |
| `LUNA_SERVICE_TOKENS` | tokens of the services allowed to introspect keys, comma-separated |
| `HTTP_TRUSTED_PROXIES` | nginx's network, so login throttling sees the client |
| `AUTH_SESSION_TTL` | developers' session lifetime |

## Tests

```bash
TEST_DATABASE_DSN=postgres://…/dashboard_test gtr run test   # each test rolls back its transaction
```

Without `TEST_DATABASE_DSN` the HTTP tests are skipped.

## License

MIT

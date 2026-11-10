# luna-dashboard

The Luna developer console: developers sign up, register their applications and create API
keys with scopes for them. It is the only service that knows keys; luna-auth and luna-pages ask
it about each key through [luna-keys](../luna-keys).

A [fullstack](https://github.com/shibisty/fullstack.go) application (`gtr new --template
fullstack`) on Postgres: pages in `resources/views`, routes in `routes/`, handlers in
`app/http/controllers`, migrations in `database/migrations`. Developers' sessions are in the
database (`passport_sessions`, passport-session-sql).

State: stage 2 of the migration plan — sign-up, login and sessions work; the introspection
endpoint answers, every key is unknown until applications and keys arrive (stage 4).

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

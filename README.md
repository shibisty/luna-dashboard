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
network allow-list) and browser keys (`pk_live_…`, allowed origins, `auth:login` and `pages:read` only). A key's
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
| `GET /apps/{app}/users`, `POST /apps/{app}/users` | the application's end users (search, pages), a new one |
| `GET /apps/{app}/users/{user}`, `POST …/{user}`, `POST …/{user}/delete` | a user: account (email, username, verified, new password), delete |
| `POST …/{user}/data`, `POST …/{user}/data/{key}/delete` | the application's data about the user: set a value, delete a key |
| `GET /apps/{app}/pages` | the site editor (luna-pages) |
| `GET /apps/{app}/pages/preview?id=` | a page or an entry as the site shows it |
| `GET /apps/{app}/pages/entries`, `POST …/entries` | quick forms: the templates' entries, a new entry |
| `GET /apps/{app}/pages/entries/{entry}`, `POST …/{entry}`, `…/publish`, `…/delete` | an entry's values: save the draft, save and publish, delete |

The users pages call luna-auth's internal API (`app/authapi`, `LUNA_AUTH_URL` with
`LUNA_AUTH_TOKEN`) after checking that the application is the developer's; without them the
pages say that luna-auth is not connected.

## Staying signed in

A login is a short session (`session_id`, `AUTH_SESSION_TTL`) and a refresh token
(`app/sessions/keeper.go`, passport/refresh in `passport_refresh_tokens`). When the session runs
out, the next request (a page, the editor's autosave) spends the token for a new session and a
new token, so whoever keeps working stays signed in. The token is a browser-session cookie
`luna_refresh` that lasts `DASHBOARD_REFRESH_TTL` from its last use; with *Remember me* it is
`luna_remember`, kept for `DASHBOARD_REMEMBER_TTL` (30 days), so a restarted browser is still
signed in. Tokens rotate: one presented again after its minute of grace (parallel requests) ends
the whole login, as a stolen copy would. Logging out revokes it. The grace is per process: with
several dashboard replicas keep sessions sticky.

## Look and language

The pages follow Material UI's look (an AppBar, Paper cards, contained buttons, outlined fields),
written as plain CSS in `resources/views/layouts/app.html` with the palette in
`partials/theme.html`; the editor's `luna-theme.css` uses the same palette. The theme is light,
dark, or the system's (`prefers-color-scheme`), and the dashboard speaks English or Russian. The
switches in the bar go through `GET /prefs?lang=&theme=&back=`, which keeps the choice for a year
in the `luna_lang` and `luna_theme` cookies; without them the language comes from the browser's
`Accept-Language` and the theme from the system.

Templates are written in English and translate with `{{.L.T "Text"}}` (`{{.L.H …}}` for text with
markup, `{{.L.N 3 "user"}}` for counts); `app/locale/ru.go` is the Russian catalogue, and a test
fails when a template's text has no translation there. Validation messages and the services'
errors are translated by pattern (`The {field} field is required.`), field names by their
`field:` keys. The editor gets GrapesJS' Russian locale, Russian block labels and button
tooltips; plugins without a Russian locale stay English.

## The site editor

`/apps/{app}/pages` is GrapesJS 0.23 with
[grapesjs-pages](https://github.com/shibisty/grapesjs-pages) (pages, layouts, templates, the
site's styles) and [grapesjs-storage](https://github.com/shibisty/grapesjs-storage) (templates'
fields, entries), plus the stand's plugins (`C:\OSPanel6\home\grapesjs`) but its plugins
manager: grapesjs-preset-webpage, -blocks-basic, -plugin-forms, -navbar, -tabs,
-component-countdown, -custom-code, -tooltip, -typed, -parser-postcss, -style-gradient,
-style-bg, -style-filter, -plugin-export, -tui-image-editor (its TOAST UI files served here,
usage statistics off), and ours: grapesjs-cloud-assets (Dropbox, Google Drive, OneDrive; Box
needs a token server and is left out), -sidebar-tools, -context-menu,
grapesjs-fonts and grapesjs-scripts.

- **Panels** (grapesjs-sidebar-tools 2): every view (Styles, Settings, Layers, Blocks, Pages,
  Content, Scripts, Fonts) is a tab; drag it to the left, right or bottom zone, or split a side
  zone in two. All tabs start on the right (320 px); the layout is kept in the browser.
  Since 2.2 a device toolbar sits over the canvas (devices, a free size, zoom) and a tab bar of
  documents under the top bar: each opened page (grapesjs-pages) and each blueprint
  (grapesjs-scripts) is a tab; the workspace splits into columns, rows or 2 × 2.
- **Context menu** (grapesjs-context-menu): right click in the canvas or the layers; its *Font*
  submenu applies the project's fonts (grapesjs-fonts).

- **Fonts** (grapesjs-fonts; the "Aa" button, the Style Manager's font family, the rich text
  toolbar): free fonts from Google Fonts, Bunny Fonts, Fontsource and Fontshare. Their catalogues
  send no CORS headers, so the browser reads them from the dashboard: `GET /editor/fonts/…`
  (`app/fontsproxy`; public, since the plugin fetches without cookies; only the four catalogue
  routes, a day's cache in memory, a stale copy when an upstream fails). The fonts themselves come
  from the CDNs. A record's fonts are saved with it and published as `@import` in its CSS, which
  luna-pages keeps for these font hosts.
- **Scripts** (grapesjs-scripts): page behaviour as blueprints, node graphs compiled to plain
  JavaScript, and CDN libraries. They are saved with the record; publishing adds the compiled
  script (always inline: luna-pages has no files) and the libraries' tags after the HTML
  (grapesjs-pages' `remote.markup`). Only sites with `custom_code` keep them; otherwise the publish
  answers with a warning, shown in the editor's bar.

On the published site: components that work with a script (tabs, the navbar's menu button, the
countdown, typed text) run only on sites that allow custom code, luna-pages' `custom_code`
setting (grapesjs-pages publishes their script after the page's HTML; other sites get it
removed and show the components still). Form fields (`input`, `select`, `textarea`) are not
published at all: luna-pages drops them, a site has nowhere to send a form yet. Luna's own theme (`resources/editor/luna-theme.css`: light and dark after MUI, fixed sizes) replaces GrapesJS' dark one. No Node and no CDN for the editor itself (fonts and script libraries come from their CDNs): the files are vendored in
[`resources/editor`](resources/editor/README.md) with their versions and hashes, embedded into
the binary and served at `/assets/editor/<file>?v=<hash>` (immutable). Font Awesome, which
GrapesJS would fetch from cdnjs, is among them.

The plugins talk to `/apps/{app}/pages/api/…` on the dashboard: the developer's session cookie,
the CSRF token in `X-CSRF-Token`. The dashboard checks that the application is the developer's,
then forwards the request to luna-pages' internal API (`app/pagesapi`, `LUNA_PAGES_URL` with
`LUNA_PAGES_TOKEN`): the path, the query, the body (4 MiB at most), `If-Match` /
`If-None-Match`, and back the status, the body and `ETag`. Only `editor/…`, `content/…` and
`GET render`, `site`, `pages[/{id}]` pass; no key ever reaches the browser.

- **Preview** (the bar's button, or an entry's link) renders the published design, an entry
  with its draft values, under `Content-Security-Policy: sandbox allow-scripts …`: scripts of a
  site that allows custom code run in an opaque origin, away from the dashboard's cookies.
- **Fields** of a template are marked in its design (an element's settings) or written as
  placeholders, `{{price}}` in a text or an address; `{{entry.slug}}` and friends show the entry's
  own (see grapesjs-storage's README). The rich text toolbar's **{…}** inserts them.
- **Entry forms** edit an entry's values without the canvas: one input per field by type
  (text, rich text as HTML, number, date, select, yes/no, image and link as URL + text, a list
  as items with a remove box and an empty row for a new one). Saving sends `If-Match`; a newer
  version (the editor saved meanwhile) answers 409 with the form kept.

## Files

`/apps/{app}/files` is the application's drive in luna-files (`app/filesapi`, `LUNA_FILES_URL` with
`LUNA_FILES_TOKEN`): folders with breadcrumbs, search in a folder or everywhere, the storage
used, uploads (several files, dropped or chosen), a file's page with its preview, details and
link (the public URL, or a signed one that expires for private files), rename, move, public or
private, delete; a folder is renamed, moved, or deleted with everything in it.

Uploads up to `LUNA_FILES_MAX_UPLOAD` (10 MiB) go through the dashboard as multipart, streamed
to luna-files; larger ones the page's script sends straight to the storage (a presigned `PUT`
from `POST …/uploads`, then `…/complete`), so the bucket needs CORS for the dashboard's origin.
Without JavaScript the form posts to the dashboard, up to the same limit. A name that is taken
gets a free one (`logo (2).png`).

The page editor's asset manager lists the application's public images and uploads new ones
there (public), the same way. Both talk to `/apps/{app}/files/api/…`: the session cookie, the
CSRF token in `X-CSRF-Token`; only listing, reading, uploading and completing pass.

## Internal API

The other services call `POST /internal/v1/keys/introspect` (with a service token) and
`GET /healthz`. nginx never routes `/internal/`.

- **OpenAPI 3.1**: [`docs/openapi.yaml`](docs/openapi.yaml), served at `GET /internal/openapi.yaml`;
  Swagger UI at `/internal/docs` with `API_DOCS=true`. A test keeps the document and the routes
  in step.
- **Postman**: [`docs/postman/luna-dashboard-internal.postman_collection.json`](docs/postman/luna-dashboard-internal.postman_collection.json)
  with [`docs/postman/luna-dashboard-local.postman_environment.json`](docs/postman/luna-dashboard-local.postman_environment.json):
  set `serviceToken` (one of `LUNA_SERVICE_TOKENS`) and `apiKey` (a key created here); the
  collection sends the key's SHA-256, as luna-keys does.

The developer console's pages are HTML forms, not an API.

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
| `LUNA_AUTH_URL`, `LUNA_AUTH_TOKEN` | luna-auth's internal address and its `LUNA_DASHBOARD_TOKEN`, for the users pages |
| `LUNA_PAGES_URL`, `LUNA_PAGES_TOKEN` | luna-pages' internal address and its `LUNA_DASHBOARD_TOKEN`, for the site editor |
| `LUNA_FILES_URL`, `LUNA_FILES_TOKEN` | luna-files' internal address and its `LUNA_DASHBOARD_TOKEN`, for the files and the editor's images |
| `LUNA_FILES_MAX_UPLOAD` | bytes sent through the dashboard (luna-files' `FILES_MAX_UPLOAD`, 10 MiB); larger go straight to the storage |
| `HTTP_TRUSTED_PROXIES` | nginx's network, so login throttling sees the client |
| `AUTH_SESSION_TTL` | a session's lifetime (`30m`; renewed by the refresh token) |
| `DASHBOARD_REFRESH_TTL` | how long a developer may be away and stay signed in (`2h`) |
| `DASHBOARD_REMEMBER_TTL` | the same with *Remember me*, across browser restarts (`720h`) |

## Tests

```bash
TEST_DATABASE_DSN=postgres://…/dashboard_test gtr run test   # each test rolls back its transaction
```

Without `TEST_DATABASE_DSN` the HTTP tests are skipped.

## License

MIT

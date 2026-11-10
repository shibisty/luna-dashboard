# Nuxt + Passport.js Starter Project

A complete authentication starter kit featuring login/registration, password recovery, OAuth (Google fully implemented, Meta/X included as placeholders), a protected Dashboard, i18n (English/Ukrainian), and light/dark theme support.

## Features

- User registration and login
- Password recovery
- Google OAuth (Meta/X ready to implement)
- Protected dashboard
- English and Ukrainian localization
- Light/Dark theme
- Server-side session authentication
- PostgreSQL with Prisma ORM

## Tech Stack

- **Nuxt 3** (Vue 3 + Nitro) — frontend and backend in a single project
- **PostgreSQL + Prisma** — database
- **Passport.js** — local authentication (email/password)
- **Manual OAuth 2.0 Authorization Code Flow** — Google implementation (Meta/X follow the same approach)
- **@nuxtjs/i18n** — English and Ukrainian localization
- **zxcvbn** — password strength indicator
- **nodemailer** — password recovery email delivery

---

# Architecture

## Server-side Sessions (No JWT)

Instead of JWT authentication, this project uses traditional **server-side sessions**.

When a user logs in or registers:

1. A new record is created in the `sessions` table.
2. Only the session ID is stored inside an HTTP-only cookie.
3. User data never leaves the server.

This approach allows sessions to be revoked instantly and keeps authentication logic simple and secure.

Session management is implemented in:

```text
server/utils/session.ts
```

because Nitro does not support `express-session`.

---

## Passport.js

Passport is used **only** for the Local Strategy.

```ts
passport.authenticate(
    "local",
    { session: false },
    callback
)
```

Using a custom callback avoids any dependency on Express sessions, making Passport compatible with Nitro.

Implementation:

```text
server/utils/passport.ts
```

---

## OAuth

OAuth providers are implemented manually using the standard OAuth 2.0 Authorization Code Flow instead of Passport OAuth strategies.

Flow:

```text
Browser
    │
    ▼
Google Login
    │
Authorization Code
    │
    ▼
Exchange Code for Access Token
    │
    ▼
Retrieve User Profile
```

The OAuth `state` value is stored inside a short-lived HTTP-only cookie using native h3 helpers.

Google is fully implemented.

Meta and X can be added using the same implementation.

Implementation:

```text
server/utils/oauth.ts
```

---

## Password Recovery

`POST /api/auth/forgot-password`

The endpoint:

1. Generates a secure random password.
2. Stores its hash.
3. Sends the new password by email using Nodemailer.

If preferred, this can easily be replaced with the more common password reset link + one-time token workflow.

---

# Project Structure

```text
server/
  api/auth/
    register.post.ts
    login.post.ts
    logout.post.ts
    me.get.ts
    forgot-password.post.ts
    google.get.ts
    google/callback.get.ts

  utils/
    prisma.ts
    session.ts
    password.ts
    passport.ts
    oauth.ts
    mailer.ts

pages/
  index.vue
  forgot-password.vue
  about.vue
  privacy.vue
  dashboard.vue

components/
  LoginForm.vue
  RegisterForm.vue
  PasswordStrengthMeter.vue
  LangSwitcher.vue
  ThemeSwitcher.vue

composables/
  useAuth.ts
  useTheme.ts

middleware/
  auth.global.ts

prisma/
  schema.prisma

i18n/
  locales/
    en.json
    uk.json
```

---

# Local Development

## Install dependencies

```bash
npm install
```

## Configure environment

```bash
cp .env.example .env
```

Configure the following variables:

```text
DATABASE_URL

SMTP_HOST
SMTP_PORT
SMTP_USER
SMTP_PASSWORD

GOOGLE_CLIENT_ID
GOOGLE_CLIENT_SECRET
```

## Run database migrations

```bash
npx prisma migrate dev --name init
```

## Start the development server

```bash
npm run dev
```

If SMTP is not configured, password recovery emails will be printed to the server console.

---

# Google OAuth Setup

1. Open **Google Cloud Console**
2. Navigate to **APIs & Services → Credentials**
3. Create an **OAuth Client ID (Web application)**
4. Add the following Authorized Redirect URI:

```text
http://localhost:3000/api/auth/google/callback
```

5. Copy the credentials into your `.env` file:

```text
GOOGLE_CLIENT_ID
GOOGLE_CLIENT_SECRET
```

---

# Production Checklist

- Add rate limiting for authentication endpoints
- Periodically remove expired sessions
- Require email verification
- Implement Meta OAuth
- Implement X (Twitter) OAuth
- Serve the application over HTTPS
- Enable secure cookies (`NODE_ENV=production`)

# Nuxt + Passport.js — Starter Project

A complete project skeleton featuring login/registration, password recovery, OAuth (Google fully implemented, Meta/X placeholders), a dashboard, i18n (en/uk), and light/dark theme support.

## Technologies

- **Nuxt 3** (Vue 3, Nitro server) — frontend and backend in a single project
- **PostgreSQL + Prisma** — database
- **Passport.js** — local authentication strategy (email/password)
- **Manual OAuth2 Authorization Code Flow** — Google (Meta/X follow the same pattern)
- **@nuxtjs/i18n** — English and Ukrainian localization
- **zxcvbn** — password strength indicator
- **nodemailer** — sends password recovery emails

## Key Architectural Decisions (and Why)

### 1. Sessions use classic server-side storage instead of JWT

When a user logs in or registers, the server creates a record in the `sessions` table (via Prisma) and stores **only the session ID** in an httpOnly cookie. The user object is never stored on the client as a token. This is the traditional session-based approach, unlike JWT where user data is embedded inside the token itself.

This is implemented manually in `server/utils/session.ts` because Nitro does not support `express-session` (Nitro is not Express). The required logic is straightforward: create, retrieve, and delete a database record, plus manage the cookie.

### 2. Passport.js is used properly, but only for the local strategy

`server/utils/passport.ts` contains a standard `passport.use(new LocalStrategy(...))` configuration. Authentication is performed via:

```ts
passport.authenticate('local', { session: false }, callback)
```

This is the officially supported Passport pattern and **does not rely on `req.session` or `req.res`** when a custom callback is provided. As a result, it can be safely used inside Nitro request handlers without wrapping Express.

### 3. OAuth (Google/Meta/X) is implemented manually instead of using `passport-google-oauth20`

Passport OAuth strategies generally expect `req.session` (for CSRF `state`) and `req.res`, neither of which exists in Nitro. Building compatibility layers for this is unnecessarily fragile.

Instead, `server/utils/oauth.ts` implements a standard three-step Authorization Code Flow (redirect → exchange authorization code for an access token → fetch user profile). The `state` parameter is stored in a short-lived httpOnly cookie using h3's native helpers.

Google is fully implemented and functional. Meta and X can be added by following the exact same pattern—see the comments at the end of the file.

### 4. Password recovery sends a newly generated password

`POST /api/auth/forgot-password` generates a random password, stores its hash, and sends the new password via email using `nodemailer`.

If you prefer a more secure approach (sending a one-time password reset link instead of emailing a password), it can be changed in a single place.

## Project Structure

```
server/
  api/auth/
    register.post.ts         POST /api/auth/register
    login.post.ts            POST /api/auth/login
    logout.post.ts           POST /api/auth/logout
    me.get.ts                GET  /api/auth/me
    forgot-password.post.ts  POST /api/auth/forgot-password
    google.get.ts            GET  /api/auth/google            (redirects to Google)
    google/callback.get.ts   GET  /api/auth/google/callback
  utils/
    prisma.ts        Prisma Client singleton
    session.ts       Session creation, retrieval, deletion, requireUser()
    password.ts      Password hashing and temporary password generation
    passport.ts      Passport local strategy
    oauth.ts         Manual OAuth2 flow, Google + template for Meta/X
    mailer.ts        Sends emails (or logs them to the console if SMTP is not configured)

pages/
  index.vue            Login/registration (switches without changing the URL)
  forgot-password.vue
  about.vue
  privacy.vue
  dashboard.vue        Accessible only to authenticated users

components/
  LoginForm.vue / RegisterForm.vue / PasswordStrengthMeter.vue
  LangSwitcher.vue / ThemeSwitcher.vue

composables/
  useAuth.ts     User state + login/register/logout/forgotPassword
  useTheme.ts    Light/dark theme stored in a cookie

middleware/
  auth.global.ts  Protects /dashboard and redirects authenticated users away from "/"

prisma/schema.prisma   User / Session / OAuthAccount
i18n/locales/{en,uk}.json
```

## Running Locally

```bash
# 1. Install dependencies
npm install

# 2. Environment variables
cp .env.example .env
# Fill in DATABASE_URL, SMTP_*, and GOOGLE_CLIENT_ID/SECRET

# 3. Initialize the database
npx prisma migrate dev --name init

# 4. Start the development server
npm run dev
```

If SMTP is not configured, password recovery emails are simply printed to the server console, which is convenient during development.

## Configuring Google OAuth

1. Google Cloud Console → APIs & Services → Credentials → OAuth Client ID (Web Application)
2. Authorized Redirect URI: `http://localhost:3000/api/auth/google/callback`
3. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in your `.env` file.

## Recommended Additions Before Production

- Rate limiting for `/api/auth/login` and `/api/auth/forgot-password` (for example, using `unstorage` with an IP-based counter)
- Periodic cleanup of expired records in the `sessions` table (cron job or Prisma `deleteMany` based on `expiresAt`)
- Email verification during registration
- Meta and X OAuth providers (the template is already available in `server/utils/oauth.ts`)
- `secure: true` for cookies is already enabled automatically when `NODE_ENV=production`, but make sure the application is served over HTTPS.

# Nuxt + Passport.js — стартовый проект

Полноценный скелет: логин/регистрация, восстановление пароля, OAuth (Google готов, Meta/X — заглушки),
Dashboard, i18n (en/uk), light/dark тема.

## Технологии

- **Nuxt 3** (Vue 3, Nitro-сервер) — фронтенд и бэкенд в одном проекте
- **PostgreSQL + Prisma** — база данных
- **Passport.js** — локальная стратегия (email/пароль)
- **Ручной OAuth2 Authorization Code Flow** — Google (Meta/X по аналогии)
- **@nuxtjs/i18n** — английский и украинский
- **zxcvbn** — индикатор сложности пароля
- **nodemailer** — отправка письма при восстановлении пароля

## Важные архитектурные решения (и почему)

### 1. Сессии — не JWT, а классические server-side

При логине/регистрации сервер создаёт запись в таблице `sessions` (Prisma) и кладёт **только id
этой записи** в httpOnly cookie. Сам объект пользователя на клиенте никогда не хранится в виде
токена — это и есть "классическая" сессия, в отличие от JWT, где данные лежат в самом токене.

Реализовано вручную в `server/utils/session.ts`, потому что пакета `express-session` в Nitro нет
(Nitro — не Express), а его логика здесь тривиальна: создать/прочитать/удалить строку в БД + cookie.

### 2. Passport.js используется по-настоящему, но только для local-стратегии

`server/utils/passport.ts` — это самый obычный `passport.use(new LocalStrategy(...))`. Вызывается
через `passport.authenticate('local', { session: false }, callback)` — это официально
поддерживаемый Passport-паттерн, который **не трогает `req.session`/`req.res`**, если передан
кастомный callback. Поэтому его можно безопасно дёрнуть из Nitro-хендлера, не оборачивая в Express.

### 3. OAuth (Google/Meta/X) — вручную, а не через passport-google-oauth20 и т.п.

OAuth2-стратегии Passport по умолчанию рассчитывают на `req.session` (для CSRF `state`) и `req.res`
— в Nitro этого нет и городить шim для этого хрупко. Вместо этого в `server/utils/oauth.ts` — обычный
3-шаговый Authorization Code Flow (редирект → обмен code на token → получение профиля), а `state`
хранится в отдельной короткоживущей httpOnly cookie через нативные хелперы h3. Google полностью
реализован и работает; Meta и X добавляются по идентичной схеме — см. комментарий в конце файла.

### 4. Восстановление пароля — как и просили, письмо с новым паролем

`POST /api/auth/forgot-password` генерирует случайный пароль, сохраняет его хэш и отправляет письмо
через `nodemailer`. Если хотите обсудить более безопасную альтернативу (ссылка на смену пароля
с одноразовым токеном вместо пароля в теле письма) — это меняется в одном месте, дайте знать.

## Структура проекта

```
server/
  api/auth/
    register.post.ts        POST  /api/auth/register
    login.post.ts            POST  /api/auth/login
    logout.post.ts           POST  /api/auth/logout
    me.get.ts                 GET  /api/auth/me
    forgot-password.post.ts  POST  /api/auth/forgot-password
    google.get.ts              GET /api/auth/google           (редирект на Google)
    google/callback.get.ts     GET /api/auth/google/callback
  utils/
    prisma.ts        singleton Prisma-клиента
    session.ts        создание/чтение/удаление сессий, requireUser()
    password.ts       хэш пароля, генерация временного пароля
    passport.ts       local-стратегия Passport
    oauth.ts           ручной OAuth2 flow, Google + шаблон для Meta/X
    mailer.ts          отправка письма (или лог в консоль, если SMTP не настроен)

pages/
  index.vue            логин/регистрация (переключаются без перехода по URL)
  forgot-password.vue
  about.vue
  privacy.vue
  dashboard.vue        только для авторизованных

components/
  LoginForm.vue / RegisterForm.vue / PasswordStrengthMeter.vue
  LangSwitcher.vue / ThemeSwitcher.vue

composables/
  useAuth.ts     user state + login/register/logout/forgotPassword
  useTheme.ts    light/dark, хранится в cookie

middleware/
  auth.global.ts  защищает /dashboard, уводит авторизованных с "/"

prisma/schema.prisma   User / Session / OAuthAccount
i18n/locales/{en,uk}.json
```

## Запуск локально

```bash
# 1. Зависимости
npm install

# 2. Переменные окружения
cp .env.example .env
# заполните DATABASE_URL, SMTP_*, GOOGLE_CLIENT_ID/SECRET

# 3. База данных
npx prisma migrate dev --name init

# 4. Запуск
npm run dev
```

Без настроенного SMTP письма восстановления пароля просто печатаются в консоль сервера —
удобно для разработки.

## Настройка Google OAuth

1. Google Cloud Console → APIs & Services → Credentials → OAuth Client ID (Web application)
2. Authorized redirect URI: `http://localhost:3000/api/auth/google/callback`
3. `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` в `.env`

## Что стоит добавить перед продакшеном

- Рейт-лимит на `/api/auth/login` и `/api/auth/forgot-password` (например, `unstorage` + счётчик по IP)
- Периодическую очистку истёкших строк в таблице `sessions` (cron / Prisma `deleteMany` по `expiresAt`)
- Подтверждение e-mail при регистрации
- Meta и X стратегии (шаблон уже в `server/utils/oauth.ts`)
- `secure: true` для cookie уже включается автоматически в `NODE_ENV=production`, но убедитесь, что
  сайт отдаётся по HTTPS

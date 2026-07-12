import passport from 'passport'
import { Strategy as LocalStrategy } from 'passport-local'
import { prisma } from './prisma'
import { verifyPassword } from './password'
import type { User } from '@prisma/client'

/**
 * Почему только local-стратегия настроена через сам Passport, а OAuth (google/meta/x) —
 * через ручной handshake в server/utils/oauth.ts:
 *
 * Passport исторически рассчитан на Express: req.session, req.login(), req.res и т.д.
 * Local-стратегии это не нужно — verify-колбэк синхронный по сути (сверить email/пароль),
 * поэтому её честно можно вызвать через passport.authenticate() даже без сессий Express,
 * подключив как connect-style middleware через h3's fromNodeMiddleware.
 *
 * OAuth2-стратегии (passport-google-oauth20 и подобные) внутри используют req.session
 * для хранения `state` (защита от CSRF) и полагаются на req.res, которых в Nitro
 * гарантированно нет. Городить для этого самодельный шim — источник трудноуловимых багов.
 * Поэтому для OAuth мы делаем протокол Authorization Code Flow вручную (это всего
 * 3 понятных шага: редирект → обмен code на token → получение профиля), а state храним
 * в обычной httpOnly cookie через h3 — см. server/utils/oauth.ts.
 */

passport.use(
  new LocalStrategy({ usernameField: 'email', passwordField: 'password' }, async (email, password, done) => {
    try {
      const user = await prisma.user.findUnique({ where: { email: email.toLowerCase() } })
      if (!user || !user.passwordHash) {
        return done(null, false, { message: 'Неверный e-mail или пароль' })
      }
      const valid = await verifyPassword(password, user.passwordHash)
      if (!valid) {
        return done(null, false, { message: 'Неверный e-mail или пароль' })
      }
      return done(null, user)
    } catch (err) {
      return done(err as Error)
    }
  })
)

/**
 * Запускает passport.authenticate('local', ...) с кастомным callback вместо
 * стандартного поведения. В этом режиме Passport ничего не делает с req.session
 * и req.res сам — просто вызывает verify-колбэк local-стратегии и отдаёт результат
 * нам, поэтому вызов безопасен и вне Express.
 */
type LocalAuthResult = { user: User; message?: undefined } | { user: null; message?: string }

export function authenticateLocal(email: string, password: string) {
  return new Promise<LocalAuthResult>((resolve, reject) => {
    const fakeReq = { body: { email, password } }
    const middleware = passport.authenticate(
      'local',
      { session: false },
      (err: Error | null, user: User | false, info: { message?: string } | undefined) => {
        if (err) return reject(err)
        resolve(user ? { user } : { user: null, message: info?.message })
      }
    )
    // res не используется passport'ом, когда передан кастомный callback — передаём заглушку
    middleware(fakeReq as any, {} as any, () => {})
  })
}

export default passport

import type { H3Event } from 'h3'
import { getCookie, setCookie, deleteCookie, getRequestHeader, getRequestIP } from 'h3'
import { prisma } from './prisma'
import type { User } from '@prisma/client'

const COOKIE_NAME = process.env.SESSION_COOKIE_NAME || 'sid'
const TTL_DAYS = Number(process.env.SESSION_TTL_DAYS || 30)

/**
 * Создаёт запись сессии в БД и выставляет httpOnly cookie с её id.
 * Cookie хранит только непрозрачный идентификатор — никаких данных пользователя на клиенте,
 * это и есть "классическая" server-side сессия (в отличие от JWT).
 */
export async function createSession(event: H3Event, userId: string) {
  const expiresAt = new Date(Date.now() + TTL_DAYS * 24 * 60 * 60 * 1000)

  const session = await prisma.session.create({
    data: {
      userId,
      expiresAt,
      userAgent: getRequestHeader(event, 'user-agent') || null,
      ip: getRequestIP(event) || null,
    },
  })

  setCookie(event, COOKIE_NAME, session.id, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    expires: expiresAt,
  })

  return session
}

/** Достаёт пользователя по cookie текущего запроса (или null, если сессии нет/истекла). */
export async function getSessionUser(event: H3Event): Promise<User | null> {
  const sessionId = getCookie(event, COOKIE_NAME)
  if (!sessionId) return null

  const session = await prisma.session.findUnique({
    where: { id: sessionId },
    include: { user: true },
  })

  if (!session || session.expiresAt < new Date()) {
    if (session) await prisma.session.delete({ where: { id: session.id } }).catch(() => {})
    deleteCookie(event, COOKIE_NAME)
    return null
  }

  return session.user
}

/** Как getSessionUser, но кидает 401, если пользователь не авторизован — для защиты API-роутов. */
export async function requireUser(event: H3Event): Promise<User> {
  const user = await getSessionUser(event)
  if (!user) {
    throw createError({ statusCode: 401, statusMessage: 'Не авторизован' })
  }
  return user
}

/** Уничтожает текущую сессию (logout). */
export async function destroySession(event: H3Event) {
  const sessionId = getCookie(event, COOKIE_NAME)
  if (sessionId) {
    await prisma.session.delete({ where: { id: sessionId } }).catch(() => {})
  }
  deleteCookie(event, COOKIE_NAME)
}

import type { H3Event } from 'h3'
import { getCookie, setCookie, deleteCookie } from 'h3'
import crypto from 'node:crypto'

const STATE_COOKIE = 'oauth_state'

/** Кладём одноразовый CSRF state в короткоживущую httpOnly cookie перед редиректом к провайдеру. */
export function createOAuthState(event: H3Event) {
  const state = crypto.randomUUID()
  setCookie(event, STATE_COOKIE, state, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: 60 * 10, // 10 минут на прохождение флоу
  })
  return state
}

/** Сверяет state, пришедший от провайдера, с тем, что мы сохранили перед редиректом. */
export function verifyOAuthState(event: H3Event, providedState: string | undefined) {
  const saved = getCookie(event, STATE_COOKIE)
  deleteCookie(event, STATE_COOKIE)
  return Boolean(saved && providedState && saved === providedState)
}

export const googleOAuth = {
  authUrl: 'https://accounts.google.com/o/oauth2/v2/auth',
  tokenUrl: 'https://oauth2.googleapis.com/token',
  profileUrl: 'https://www.googleapis.com/oauth2/v3/userinfo',

  buildAuthUrl(state: string) {
    const params = new URLSearchParams({
      client_id: process.env.GOOGLE_CLIENT_ID || '',
      redirect_uri: process.env.GOOGLE_CALLBACK_URL || '',
      response_type: 'code',
      scope: 'openid email profile',
      state,
      prompt: 'select_account',
    })
    return `${this.authUrl}?${params.toString()}`
  },

  async exchangeCode(code: string) {
    const res = await fetch(this.tokenUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        client_id: process.env.GOOGLE_CLIENT_ID || '',
        client_secret: process.env.GOOGLE_CLIENT_SECRET || '',
        redirect_uri: process.env.GOOGLE_CALLBACK_URL || '',
        grant_type: 'authorization_code',
        code,
      }),
    })
    if (!res.ok) throw new Error(`Google token exchange failed: ${res.status}`)
    return res.json() as Promise<{ access_token: string }>
  },

  async fetchProfile(accessToken: string) {
    const res = await fetch(this.profileUrl, {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
    if (!res.ok) throw new Error(`Google profile fetch failed: ${res.status}`)
    return res.json() as Promise<{ sub: string; email: string; email_verified: boolean }>
  },
}

/**
 * Meta (Facebook) и X (Twitter) добавляются по идентичной схеме:
 * 1. свой authUrl/tokenUrl/profileUrl по документации провайдера
 *    (Meta: https://developers.facebook.com/docs/facebook-login/guides/advanced/manual-flow ,
 *     X: https://developer.x.com/en/docs/authentication/oauth-2-0/authorization-code — у X ещё нужен PKCE)
 * 2. пара роутов server/api/auth/<provider>.get.ts и server/api/auth/<provider>/callback.get.ts,
 *    зеркальных google.get.ts / google/callback.get.ts
 * 3. в OAuthAccount.provider хранить 'meta' / 'x' соответственно
 */

import { verifyOAuthState, googleOAuth } from '../../../utils/oauth'
import { prisma } from '../../../utils/prisma'
import { createSession } from '../../../utils/session'

export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const code = query.code as string | undefined
  const state = query.state as string | undefined

  if (!code || !verifyOAuthState(event, state)) {
    throw createError({ statusCode: 400, statusMessage: 'Некорректный OAuth state — попробуйте войти ещё раз' })
  }

  const { access_token } = await googleOAuth.exchangeCode(code)
  const profile = await googleOAuth.fetchProfile(access_token)

  if (!profile.email_verified) {
    throw createError({ statusCode: 400, statusMessage: 'E-mail в Google-аккаунте не подтверждён' })
  }

  // Ищем существующую привязку google-аккаунта, иначе — существующего пользователя по e-mail,
  // иначе создаём нового.
  let account = await prisma.oAuthAccount.findUnique({
    where: { provider_providerUserId: { provider: 'google', providerUserId: profile.sub } },
    include: { user: true },
  })

  if (!account) {
    const existingUser = await prisma.user.findUnique({ where: { email: profile.email.toLowerCase() } })
    const user =
      existingUser ??
      (await prisma.user.create({ data: { email: profile.email.toLowerCase(), passwordHash: null } }))

    account = await prisma.oAuthAccount.create({
      data: { provider: 'google', providerUserId: profile.sub, userId: user.id },
      include: { user: true },
    })
  }

  await createSession(event, account.user.id)
  return sendRedirect(event, '/dashboard')
})

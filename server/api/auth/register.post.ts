import { z } from 'zod'
import { prisma } from '../../utils/prisma'
import { hashPassword } from '../../utils/password'
import { createSession } from '../../utils/session'

const bodySchema = z.object({
  email: z.string().email('Некорректный e-mail'),
  password: z.string().min(6, 'Пароль должен быть не короче 6 символов'),
})

export default defineEventHandler(async (event) => {
  const raw = await readBody(event)
  const parsed = bodySchema.safeParse(raw)
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.errors[0]?.message || 'Некорректные данные' })
  }
  const body = parsed.data
  const email = body.email.toLowerCase().trim()

  const existing = await prisma.user.findUnique({ where: { email } })
  if (existing) {
    throw createError({ statusCode: 409, statusMessage: 'Пользователь с таким e-mail уже существует' })
  }

  const passwordHash = await hashPassword(body.password)
  const user = await prisma.user.create({ data: { email, passwordHash } })

  await createSession(event, user.id)

  return { id: user.id, email: user.email }
})

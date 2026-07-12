import { z } from 'zod'
import { authenticateLocal } from '../../utils/passport'
import { createSession } from '../../utils/session'

const bodySchema = z.object({
  email: z.string().email('Некорректный e-mail'),
  password: z.string().min(1, 'Введите пароль'),
})

export default defineEventHandler(async (event) => {
  const raw = await readBody(event)
  const parsed = bodySchema.safeParse(raw)
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.errors[0]?.message || 'Некорректные данные' })
  }

  const { user, message } = await authenticateLocal(parsed.data.email.toLowerCase().trim(), parsed.data.password)

  if (!user) {
    throw createError({ statusCode: 401, statusMessage: message || 'Неверный e-mail или пароль' })
  }

  await createSession(event, user.id)

  return { id: user.id, email: user.email }
})

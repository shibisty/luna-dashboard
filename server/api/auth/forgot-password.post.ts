import { z } from 'zod'
import { prisma } from '../../utils/prisma'
import { hashPassword, generateTempPassword } from '../../utils/password'
import { sendPasswordResetEmail } from '../../utils/mailer'

const bodySchema = z.object({
  email: z.string().email('Некорректный e-mail'),
})

export default defineEventHandler(async (event) => {
  const raw = await readBody(event)
  const parsed = bodySchema.safeParse(raw)
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.errors[0]?.message || 'Некорректные данные' })
  }

  const email = parsed.data.email.toLowerCase().trim()
  const user = await prisma.user.findUnique({ where: { email } })

  // Намеренно не раскрываем, существует ли e-mail в системе — одинаковый ответ в обоих случаях.
  if (user) {
    const tempPassword = generateTempPassword()
    const passwordHash = await hashPassword(tempPassword)
    await prisma.user.update({ where: { id: user.id }, data: { passwordHash } })
    await sendPasswordResetEmail(email, tempPassword)
  }

  return { ok: true, message: 'Если такой e-mail зарегистрирован, мы отправили новый пароль на почту.' }
})

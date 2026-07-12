import nodemailer from 'nodemailer'

const transporter = nodemailer.createTransport({
  host: process.env.SMTP_HOST,
  port: Number(process.env.SMTP_PORT || 587),
  secure: Number(process.env.SMTP_PORT) === 465,
  auth: process.env.SMTP_USER
    ? { user: process.env.SMTP_USER, pass: process.env.SMTP_PASS }
    : undefined,
})

export async function sendPasswordResetEmail(to: string, newPassword: string) {
  const from = process.env.SMTP_FROM || 'no-reply@example.com'

  // Если SMTP не настроен (локальная разработка) — просто логируем в консоль,
  // чтобы не блокировать разработку без реального почтового сервера.
  if (!process.env.SMTP_HOST) {
    console.warn(`[mailer] SMTP не настроен. Новый пароль для ${to}: ${newPassword}`)
    return
  }

  await transporter.sendMail({
    from,
    to,
    subject: 'Восстановление пароля',
    text: `Ваш новый пароль: ${newPassword}\n\nРекомендуем сменить его после входа в систему.`,
    html: `<p>Ваш новый пароль: <b>${newPassword}</b></p><p>Рекомендуем сменить его после входа в систему.</p>`,
  })
}

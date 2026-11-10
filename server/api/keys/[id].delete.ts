import { prisma } from '../../utils/prisma'
import { requireUser } from '../../utils/session'

export default defineEventHandler(async (event) => {
  const user = await requireUser(event)
  const id = getRouterParam(event, 'id')!

  const existing = await prisma.apiKey.findFirst({ where: { id, userId: user.id } })
  if (!existing) throw createError({ statusCode: 404, statusMessage: 'Ключ не найден' })

  await prisma.apiKey.delete({ where: { id } })
  return { ok: true }
})

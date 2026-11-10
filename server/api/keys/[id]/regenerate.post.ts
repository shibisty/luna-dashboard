import { prisma } from '../../../utils/prisma'
import { requireUser } from '../../../utils/session'
import { generatePrivateKey } from '../../../utils/api-keys'
import { hashPassword } from '../../../utils/password'

export default defineEventHandler(async (event) => {
  const user = await requireUser(event)
  const id = getRouterParam(event, 'id')!

  const existing = await prisma.apiKey.findFirst({ where: { id, userId: user.id } })
  if (!existing) throw createError({ statusCode: 404, statusMessage: 'Ключ не найден' })

  const privateKey = generatePrivateKey()
  await prisma.apiKey.update({ where: { id }, data: { privateKeyHash: await hashPassword(privateKey) } })

  return { id, privateKey }
})

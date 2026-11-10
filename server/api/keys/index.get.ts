import { prisma } from '../../utils/prisma'
import { requireUser } from '../../utils/session'

export default defineEventHandler(async (event) => {
  const user = await requireUser(event)
  return prisma.apiKey.findMany({
    where: { userId: user.id },
    orderBy: { createdAt: 'desc' },
    select: {
      id: true, name: true, publicKey: true,
      allowedDomains: true, allowedIps: true, scopes: true,
      lastUsedAt: true, createdAt: true,
    },
  })
})

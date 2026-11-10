import { z } from 'zod'
import { prisma } from '../../utils/prisma'
import { requireUser } from '../../utils/session'
import { parseCommaList } from '../../utils/api-keys'
import { API_KEY_SCOPE_VALUES } from '../../../shared/scopes'

const bodySchema = z.object({
  name: z.string().min(1).max(100).optional(),
  domains: z.string().optional(),
  ips: z.string().optional(),
  scopes: z.array(z.enum(API_KEY_SCOPE_VALUES as [string, ...string[]])).min(1).optional(),
})

export default defineEventHandler(async (event) => {
  const user = await requireUser(event)
  const id = getRouterParam(event, 'id')!
  const parsed = bodySchema.safeParse(await readBody(event))
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.errors[0]?.message || 'Некорректные данные' })
  }

  const existing = await prisma.apiKey.findFirst({ where: { id, userId: user.id } })
  if (!existing) throw createError({ statusCode: 404, statusMessage: 'Ключ не найден' })

  return prisma.apiKey.update({
    where: { id },
    data: {
      ...(parsed.data.name !== undefined && { name: parsed.data.name }),
      ...(parsed.data.domains !== undefined && { allowedDomains: parseCommaList(parsed.data.domains) }),
      ...(parsed.data.ips !== undefined && { allowedIps: parseCommaList(parsed.data.ips) }),
      ...(parsed.data.scopes !== undefined && { scopes: parsed.data.scopes }),
    },
  })
})

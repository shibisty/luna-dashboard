import { z } from 'zod'
import { prisma } from '../../utils/prisma'
import { requireUser } from '../../utils/session'
import { generatePublicKey, generatePrivateKey, parseCommaList } from '../../utils/api-keys'
import { hashPassword } from '../../utils/password'
import { API_KEY_SCOPE_VALUES } from '../../../shared/scopes'

const bodySchema = z.object({
  name: z.string().min(1, 'Укажите название ключа').max(100),
  domains: z.string().optional(),
  ips: z.string().optional(),
  scopes: z.array(z.enum(API_KEY_SCOPE_VALUES as [string, ...string[]])).min(1, 'Выберите хотя бы один scope'),
})

export default defineEventHandler(async (event) => {
  const user = await requireUser(event)
  const parsed = bodySchema.safeParse(await readBody(event))
  if (!parsed.success) {
    throw createError({ statusCode: 400, statusMessage: parsed.error.errors[0]?.message || 'Некорректные данные' })
  }

  const publicKey = generatePublicKey()
  const privateKey = generatePrivateKey()
  const privateKeyHash = await hashPassword(privateKey)

  const apiKey = await prisma.apiKey.create({
    data: {
      name: parsed.data.name,
      publicKey,
      privateKeyHash,
      allowedDomains: parseCommaList(parsed.data.domains),
      allowedIps: parseCommaList(parsed.data.ips),
      scopes: parsed.data.scopes,
      userId: user.id,
    },
  })

  return {
    id: apiKey.id,
    name: apiKey.name,
    publicKey: apiKey.publicKey,
    privateKey, // единственный раз в открытом виде — дальше только хэш
    allowedDomains: apiKey.allowedDomains,
    allowedIps: apiKey.allowedIps,
    scopes: apiKey.scopes,
    createdAt: apiKey.createdAt,
  }
})

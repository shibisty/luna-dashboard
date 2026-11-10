import { z } from 'zod'
import { prisma } from '../../utils/prisma'
import { verifyPassword } from '../../utils/password'

const bodySchema = z.object({ publicKey: z.string(), privateKey: z.string() })

function hostFromOrigin(event: any): string | null {
  const origin = getRequestHeader(event, 'origin') || getRequestHeader(event, 'referer')
  if (!origin) return null
  try { return new URL(origin).hostname.toLowerCase() } catch { return null }
}

export default defineEventHandler(async (event) => {
  const parsed = bodySchema.safeParse(await readBody(event))
  if (!parsed.success) throw createError({ statusCode: 400, statusMessage: 'publicKey и privateKey обязательны' })

  const apiKey = await prisma.apiKey.findUnique({ where: { publicKey: parsed.data.publicKey } })
  if (!apiKey) throw createError({ statusCode: 401, statusMessage: 'Неверный ключ' })

  const validSecret = await verifyPassword(parsed.data.privateKey, apiKey.privateKeyHash)
  if (!validSecret) throw createError({ statusCode: 401, statusMessage: 'Неверный ключ' })

  if (apiKey.allowedDomains.length > 0) {
    const host = hostFromOrigin(event)
    if (!host || !apiKey.allowedDomains.includes(host)) {
      throw createError({ statusCode: 403, statusMessage: 'Домен не разрешён для этого ключа' })
    }
  }

  if (apiKey.allowedIps.length > 0) {
    const ip = getRequestIP(event, { xForwardedFor: true })
    if (!ip || !apiKey.allowedIps.includes(ip)) {
      throw createError({ statusCode: 403, statusMessage: 'IP не разрешён для этого ключа' })
    }
  }

  await prisma.apiKey.update({ where: { id: apiKey.id }, data: { lastUsedAt: new Date() } }).catch(() => {})

  return { valid: true, name: apiKey.name, scopes: apiKey.scopes }
})

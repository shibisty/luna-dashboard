import { z } from 'zod'
import { prisma } from '../../utils/prisma'

const bodySchema = z.object({ publicKey: z.string() })

export default defineEventHandler(async (event) => {
  const parsed = bodySchema.safeParse(await readBody(event))
  if (!parsed.success) throw createError({ statusCode: 400, statusMessage: 'publicKey обязателен' })

  const apiKey = await prisma.apiKey.findUnique({
    where: { publicKey: parsed.data.publicKey },
    select: { scopes: true, name: true },
  })
  if (!apiKey) throw createError({ statusCode: 404, statusMessage: 'Ключ не найден' })

  return { name: apiKey.name, scopes: apiKey.scopes }
})

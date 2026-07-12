import { PrismaClient } from '@prisma/client'

// В dev-режиме Nitro может пересобирать модуль при HMR — храним клиент в globalThis,
// чтобы не плодить лишние подключения к БД.
const globalForPrisma = globalThis as unknown as { prisma?: PrismaClient }

export const prisma = globalForPrisma.prisma ?? new PrismaClient()

if (process.env.NODE_ENV !== 'production') {
  globalForPrisma.prisma = prisma
}

import bcrypt from 'bcryptjs'
import crypto from 'node:crypto'

export async function hashPassword(plain: string) {
  return bcrypt.hash(plain, 12)
}

export async function verifyPassword(plain: string, hash: string) {
  return bcrypt.compare(plain, hash)
}

/**
 * Генерирует случайный пароль для отправки на e-mail при восстановлении.
 * 10 символов, буквы + цифры, без визуально похожих символов (0/O, 1/l).
 */
export function generateTempPassword(length = 10) {
  const alphabet = 'ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789'
  const bytes = crypto.randomBytes(length)
  let out = ''
  for (let i = 0; i < length; i++) {
    out += alphabet[bytes[i] % alphabet.length]
  }
  return out
}

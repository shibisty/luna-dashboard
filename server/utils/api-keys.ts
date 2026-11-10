import crypto from 'node:crypto'

export function generatePublicKey() {
  return `pub_${crypto.randomBytes(16).toString('hex')}`
}

export function generatePrivateKey() {
  return `sec_${crypto.randomBytes(32).toString('hex')}`
}

export function parseCommaList(input?: string | null): string[] {
  if (!input) return []
  return input.split(',').map((s) => s.trim().toLowerCase()).filter(Boolean)
}

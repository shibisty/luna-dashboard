interface ApiKeySummary {
  id: string; name: string; publicKey: string
  allowedDomains: string[]; allowedIps: string[]; scopes: string[]
  lastUsedAt: string | null; createdAt: string
}
interface CreatedApiKey extends ApiKeySummary { privateKey: string }

export const useApiKeys = () => {
  const keys = useState<ApiKeySummary[]>('api-keys', () => [])

  async function fetchKeys() {
    keys.value = await $fetch<ApiKeySummary[]>('/api/keys')
  }
  async function createKey(payload: { name: string; domains?: string; ips?: string; scopes: string[] }) {
    const created = await $fetch<CreatedApiKey>('/api/keys', { method: 'POST', body: payload })
    await fetchKeys()
    return created
  }
  async function updateKey(id: string, payload: { name?: string; domains?: string; ips?: string; scopes?: string[] }) {
    await $fetch(`/api/keys/${id}`, { method: 'PATCH', body: payload })
    await fetchKeys()
  }
  async function deleteKey(id: string) {
    await $fetch(`/api/keys/${id}`, { method: 'DELETE' })
    keys.value = keys.value.filter((k) => k.id !== id)
  }
  async function regenerateKey(id: string) {
    return $fetch<{ id: string; privateKey: string }>(`/api/keys/${id}/regenerate`, { method: 'POST' })
  }

  return { keys, fetchKeys, createKey, updateKey, deleteKey, regenerateKey }
}

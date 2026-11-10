export const API_KEY_SCOPES = [
  { value: 'auth:read', label: 'auth — read' },
  { value: 'auth:write', label: 'auth — write' },
] as const

export type ApiKeyScopeValue = (typeof API_KEY_SCOPES)[number]['value']
export const API_KEY_SCOPE_VALUES = API_KEY_SCOPES.map((s) => s.value) as ApiKeyScopeValue[]

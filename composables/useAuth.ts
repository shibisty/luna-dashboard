interface AuthUser {
  id: string
  email: string
}

export const useAuth = () => {
  const user = useState<AuthUser | null>('auth-user', () => null)
  const pending = useState('auth-pending', () => false)

  async function fetchMe() {
    const { data } = await useAsyncData('auth-user', () =>
      $fetch<AuthUser>('/api/auth/me').catch(() => null)
    )
    user.value = data.value
  }

  async function login(email: string, password: string) {
    pending.value = true
    try {
      user.value = await $fetch<AuthUser>('/api/auth/login', { method: 'POST', body: { email, password } })
    } finally {
      pending.value = false
    }
  }

  async function register(email: string, password: string) {
    pending.value = true
    try {
      user.value = await $fetch<AuthUser>('/api/auth/register', { method: 'POST', body: { email, password } })
    } finally {
      pending.value = false
    }
  }

  async function logout() {
    await $fetch('/api/auth/logout', { method: 'POST' })
    user.value = null
    await navigateTo('/')
  }

  async function forgotPassword(email: string) {
    return $fetch<{ ok: boolean; message: string }>('/api/auth/forgot-password', {
      method: 'POST',
      body: { email },
    })
  }

  return { user, pending, fetchMe, login, register, logout, forgotPassword }
}

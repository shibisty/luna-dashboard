export default defineNuxtRouteMiddleware(async (to) => {
  const { user, fetchMe } = useAuth()

  // Подгружаем текущего пользователя один раз за навигацию на сервере/при первой загрузке
  if (user.value === null) {
    await fetchMe()
  }

  const isAuthed = Boolean(user.value)
  const publicPaths = ['/', '/about', '/privacy', '/forgot-password']

  if (to.path === '/dashboard' && !isAuthed) {
    return navigateTo('/')
  }

  if (to.path === '/' && isAuthed) {
    return navigateTo('/dashboard')
  }

  if (!publicPaths.includes(to.path) && to.path !== '/dashboard') {
    // Неизвестные/будущие приватные страницы — по умолчанию считаем защищёнными
    if (!isAuthed) return navigateTo('/')
  }
})

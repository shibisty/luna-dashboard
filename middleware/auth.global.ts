export default defineNuxtRouteMiddleware(async (to) => {
  const { user, fetchMe } = useAuth()

  // На каждой навигации, где ещё не знаем пользователя — ждём ответ, не рендерим страницу раньше времени
  if (user.value === null && !import.meta.server) {
    await fetchMe()
  } else if (import.meta.server) {
    await fetchMe()
  }

  const isAuthed = Boolean(user.value)
  const publicPaths = ['/', '/about', '/privacy', '/forgot-password']
  const isDashboardRoute = to.path.startsWith('/dashboard')

  if (isDashboardRoute && !isAuthed) {
    return navigateTo('/')
  }

  if (to.path === '/' && isAuthed) {
    return navigateTo('/dashboard')
  }

  if (!publicPaths.includes(to.path) && !isDashboardRoute) {
    if (!isAuthed) return navigateTo('/')
  }
})

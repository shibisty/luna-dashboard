export default defineNuxtPlugin((nuxtApp) => {
  const cookie = useCookie<'light' | 'dark'>('theme', { default: () => 'light' })

  // vuetify-nuxt-module кладёт инстанс темы сюда при SSR
  nuxtApp.hook('app:created', () => {
    const vuetify = nuxtApp.vueApp.config.globalProperties.$vuetify
    if (vuetify?.theme) {
      vuetify.theme.global.name.value = cookie.value
    }
  })
})

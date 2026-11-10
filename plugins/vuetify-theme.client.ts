export default defineNuxtPlugin(() => {
  const cookie = useCookie<'light' | 'dark'>('theme', { default: () => 'light' })
  const vuetifyTheme = useNuxtApp().$vuetify?.theme

  if (vuetifyTheme && vuetifyTheme.global.name.value !== cookie.value) {
    vuetifyTheme.global.name.value = cookie.value
  }
})

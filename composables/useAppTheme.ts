import { useTheme as useVuetifyTheme } from 'vuetify'

export const useAppTheme = () => {
  const vuetifyTheme = useVuetifyTheme()
  const cookie = useCookie<'light' | 'dark'>('theme', { default: () => 'light', maxAge: 60 * 60 * 24 * 365 })

  const apply = (value: 'light' | 'dark') => {
    cookie.value = value
    vuetifyTheme.global.name.value = value
  }

  const toggle = () => apply(vuetifyTheme.global.current.value.dark ? 'light' : 'dark')

  if (import.meta.client && vuetifyTheme.global.name.value !== cookie.value) {
    vuetifyTheme.global.name.value = cookie.value
  }

  return { theme: computed(() => (vuetifyTheme.global.current.value.dark ? 'dark' : 'light')), apply, toggle }
}

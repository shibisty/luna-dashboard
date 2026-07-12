export type Theme = 'light' | 'dark'

export const useTheme = () => {
  const theme = useCookie<Theme>('theme', { default: () => 'light', maxAge: 60 * 60 * 24 * 365 })

  const apply = (value: Theme) => {
    theme.value = value
    if (import.meta.client) {
      document.documentElement.setAttribute('data-theme', value)
    }
  }

  const toggle = () => apply(theme.value === 'light' ? 'dark' : 'light')

  return { theme, apply, toggle }
}

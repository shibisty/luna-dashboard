export default defineNuxtPlugin(() => {
  const { theme } = useTheme()

  useHead({
    htmlAttrs: {
      'data-theme': computed(() => theme.value),
    },
  })
})

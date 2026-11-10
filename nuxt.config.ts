export default defineNuxtConfig({
  compatibilityDate: '2026-01-01',
  devtools: { enabled: true },

  modules: ['@nuxtjs/i18n', 'vuetify-nuxt-module'],

  vuetify: {
    moduleOptions: {
      ssrClientHints: {
        reloadOnFirstRequest: false,
        viewportSize: true,
        prefersColorScheme: false,
      },
    },
    vuetifyOptions: {
      theme: {
        defaultTheme: 'light',
        themes: {
          light: {
            dark: false,
            colors: {
              primary: '#1B1F23',
              secondary: '#5B5850',
              background: '#F7F6F3',
              surface: '#FFFFFF',
              error: '#B3432B',
              success: '#3F6B4F',
            },
          },
          dark: {
            dark: true,
            colors: {
              primary: '#E8E6DF',
              secondary: '#9A9A92',
              background: '#12151C',
              surface: '#1B1F29',
              error: '#E2624B',
              success: '#7FB894',
            },
          },
        },
      },
    },
  },

  i18n: {
    locales: [
      { code: 'en', name: 'English', file: 'en.json' },
      { code: 'uk', name: 'Українська', file: 'uk.json' },
    ],
    langDir: 'locales/',
    defaultLocale: 'en',
    strategy: 'no_prefix',
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'i18n_redirected',
      redirectOn: 'root',
    },
  },

  app: {
    head: {
      title: 'Passport App',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
    },
  },

  runtimeConfig: {
    public: {
      appUrl: process.env.APP_URL || 'http://localhost:3000',
    },
  },
})

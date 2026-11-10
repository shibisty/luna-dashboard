<script setup lang="ts">
const { user, logout } = useAuth()
const { t } = useI18n()
</script>

<template>
  <div>
    <v-app-bar flat border>
      <v-app-bar-title class="text-body-2 font-weight-medium">PASSPORT // APP</v-app-bar-title>

      <v-spacer />

      <v-btn to="/about" variant="text" size="small">{{ t('nav.about') }}</v-btn>
      <v-btn to="/privacy" variant="text" size="small">{{ t('nav.privacy') }}</v-btn>
      <v-btn v-if="user" to="/dashboard" variant="text" size="small">{{ t('nav.dashboard') }}</v-btn>

      <v-divider vertical class="mx-2" />

      <LangSwitcher class="mr-2" />
      <ThemeSwitcher />

      <v-btn v-if="user" variant="text" size="small" @click="logout">{{ t('nav.logout') }}</v-btn>
    </v-app-bar>

    <v-main>
      <slot />
    </v-main>
  </div>
</template>

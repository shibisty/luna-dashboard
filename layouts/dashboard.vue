<script setup lang="ts">
const { user, logout } = useAuth()
const { t } = useI18n()
const route = useRoute()

const nav = [
  { title: t('nav.dashboard'), icon: 'mdi-key-variant', to: '/dashboard' },
  { title: t('nav.users'), icon: 'mdi-account-group', to: '/dashboard/users' },
  { title: t('nav.pages'), icon: 'mdi-file-tree', to: '/dashboard/pages' },
]
</script>

<template>
  <div>
    <v-navigation-drawer permanent rail expand-on-hover>
      <v-list-item class="mono text-caption" title="PASSPORT" subtitle="APP" />
      <v-divider />

      <v-list nav density="compact">
        <v-list-item
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          :prepend-icon="item.icon"
          :title="item.title"
          :active="route.path === item.to"
        />
      </v-list>

      <template #append>
        <v-divider />
        <div class="d-flex flex-column ga-1 pa-2">
          <LangSwitcher />
          <ThemeSwitcher />
          <v-btn variant="text" prepend-icon="mdi-logout" @click="logout">{{ t('nav.logout') }}</v-btn>
        </div>
      </template>
    </v-navigation-drawer>

    <v-main>
      <slot />
    </v-main>
  </div>
</template>

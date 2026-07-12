<script setup lang="ts">
const { user, logout } = useAuth()
const { t } = useI18n()
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <NuxtLink to="/" class="brand mono">PASSPORT&nbsp;// APP</NuxtLink>

      <nav class="nav">
        <NuxtLink to="/about" class="nav-link">{{ t('nav.about') }}</NuxtLink>
        <NuxtLink to="/privacy" class="nav-link">{{ t('nav.privacy') }}</NuxtLink>
        <NuxtLink v-if="user" to="/dashboard" class="nav-link">{{ t('nav.dashboard') }}</NuxtLink>
      </nav>

      <div class="controls">
        <LangSwitcher />
        <ThemeSwitcher />
        <button v-if="user" class="btn" @click="logout">{{ t('nav.logout') }}</button>
      </div>
    </header>
    <div class="perforation" />

    <main class="content">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.shell {
  min-height: 100%;
  display: flex;
  flex-direction: column;
}

.topbar {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 16px 28px;
}

.brand {
  font-size: 13px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-decoration: none;
  color: var(--ink);
}

.nav {
  display: flex;
  gap: 20px;
  margin-right: auto;
}

.nav-link {
  font-size: 14px;
  text-decoration: none;
  color: var(--ink-soft);
}

.nav-link:hover,
.nav-link.router-link-exact-active {
  color: var(--ink);
}

.controls {
  display: flex;
  align-items: center;
  gap: 10px;
}

.content {
  flex: 1;
  display: flex;
  flex-direction: column;
}
</style>

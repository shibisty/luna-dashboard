<script setup lang="ts">
const { locale, locales, setLocale } = useI18n()
</script>

<template>
  <div class="lang-switcher mono">
    <button
      v-for="l in locales"
      :key="typeof l === 'string' ? l : l.code"
      class="lang-btn"
      :class="{ active: (typeof l === 'string' ? l : l.code) === locale }"
      :aria-pressed="(typeof l === 'string' ? l : l.code) === locale"
      @click="setLocale(typeof l === 'string' ? l : l.code)"
    >
      {{ (typeof l === 'string' ? l : l.code).toUpperCase() }}
    </button>
  </div>
</template>

<style scoped>
.lang-switcher {
  display: flex;
  gap: 2px;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 2px;
}

.lang-btn {
  background: transparent;
  border: none;
  color: var(--ink-soft);
  font-size: 12px;
  padding: 5px 9px;
  border-radius: 3px;
  cursor: pointer;
}

.lang-btn.active {
  background: var(--accent);
  color: var(--accent-ink);
}
</style>

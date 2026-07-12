<script setup lang="ts">
const { t } = useI18n()
const { user } = useAuth()
</script>

<template>
  <div class="page">
    <div class="card content-card">
      <div class="stamp">OK</div>
      <div>
        <h1>{{ t('dashboard.welcome') }}</h1>
        <p class="mono">{{ t('dashboard.signedInAs') }}: {{ user?.email }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 48px 24px;
}

.content-card {
  max-width: 640px;
  width: 100%;
  padding: 40px;
  display: flex;
  align-items: center;
  gap: 24px;
}

.content-card p {
  color: var(--ink-soft);
  margin: 8px 0 0;
}
</style>

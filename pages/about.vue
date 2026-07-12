<script setup lang="ts">
const { t } = useI18n()
</script>

<template>
  <div class="page">
    <div class="card content-card">
      <h1>{{ t('about.title') }}</h1>
      <p>{{ t('about.body') }}</p>
    </div>
  </div>
</template>

<style scoped>
.page {
  flex: 1;
  display: flex;
  justify-content: center;
  padding: 48px 24px;
}

.content-card {
  max-width: 640px;
  width: 100%;
  padding: 40px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.content-card p {
  color: var(--ink-soft);
  line-height: 1.6;
}
</style>

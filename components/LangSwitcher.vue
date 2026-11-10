<script setup lang="ts">
const { locale, locales, setLocale } = useI18n()

const items = computed(() => locales.value.map((l) => (typeof l === 'string' ? { code: l } : l)))
</script>

<template>
  <v-btn-toggle :model-value="locale" mandatory density="compact" variant="outlined" @update:model-value="setLocale">
    <v-btn v-for="item in items" :key="item.code" :value="item.code" size="small">
      {{ item.code.toUpperCase() }}
    </v-btn>
  </v-btn-toggle>
</template>

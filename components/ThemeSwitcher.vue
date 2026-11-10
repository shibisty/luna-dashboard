<script setup lang="ts">
const { theme, toggle } = useAppTheme()
</script>

<template>
  <v-btn :icon="theme === 'light' ? 'mdi-weather-sunny' : 'mdi-weather-night'" variant="text" @click="toggle" />
</template>

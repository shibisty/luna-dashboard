<script setup lang="ts">
const { t } = useI18n()
</script>

<template>
  <v-container class="py-12">
    <v-row justify="center">
      <v-col cols="12" md="8">
        <v-card class="pa-8">
          <h1 class="text-h5 mb-4">{{ t('about.title') }}</h1>
          <p class="text-body-1 text-medium-emphasis">{{ t('about.body') }}</p>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup lang="ts">
const { t } = useI18n()
const { forgotPassword } = useAuth()

const email = ref('')
const sent = ref(false)
const loading = ref(false)

async function onSubmit() {
  loading.value = true
  try {
    await forgotPassword(email.value)
    sent.value = true
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <v-container class="fill-height" fluid>
    <v-row justify="center" align="center">
      <v-col cols="12" sm="8" md="5" lg="4">
        <v-card class="pa-8" elevation="2">
          <v-form v-if="!sent" @submit.prevent="onSubmit">
            <h1 class="text-h5 mb-4">{{ t('auth.forgotPassword') }}</h1>
            <v-text-field v-model="email" :label="t('auth.email')" type="email" required class="mb-4" />
            <v-btn type="submit" color="primary" block :loading="loading" class="mb-4">
              {{ t('auth.sendNewPassword') }}
            </v-btn>
            <v-btn to="/" variant="text" block>{{ t('auth.backToLogin') }}</v-btn>
          </v-form>

          <div v-else class="text-center">
            <p class="mb-4">{{ t('auth.resetSent') }}</p>
            <v-btn to="/" variant="text">{{ t('auth.backToLogin') }}</v-btn>
          </div>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

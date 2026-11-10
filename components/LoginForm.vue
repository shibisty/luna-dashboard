<script setup lang="ts">
const emit = defineEmits<{ switchToRegister: []; switchToForgot: [] }>()
const { t } = useI18n()
const { login, pending } = useAuth()

const email = ref('')
const password = ref('')
const error = ref('')

async function onSubmit() {
  error.value = ''
  try {
    await login(email.value, password.value)
    await navigateTo('/dashboard')
  } catch (e: any) {
    error.value = e?.data?.statusMessage || t('auth.invalidCredentials')
  }
}

const providers = [
  { id: 'google', label: 'Google', ready: true },
  { id: 'meta', label: 'Meta', ready: false },
  { id: 'x', label: 'X', ready: false },
]
</script>

<template>
  <v-form @submit.prevent="onSubmit">
    <h1 class="text-h5 mb-4">{{ t('auth.loginTitle') }}</h1>

    <v-text-field v-model="email" :label="t('auth.email')" type="email" required class="mb-2" />
    <v-text-field v-model="password" :label="t('auth.password')" type="password" required class="mb-1" />

    <v-btn variant="text" size="small" class="mb-2 px-0" @click="emit('switchToForgot')">
      {{ t('auth.forgotPassword') }}
    </v-btn>

    <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

    <v-btn type="submit" color="primary" block :loading="pending" class="mb-4">{{ t('auth.signIn') }}</v-btn>

    <v-divider class="mb-4">
      <span class="text-caption text-medium-emphasis px-2">{{ t('auth.orContinueWith') }}</span>
    </v-divider>

    <div class="d-flex flex-column ga-2 mb-4">
      <v-btn v-for="p in providers" :key="p.id" :href="p.ready ? `/api/auth/${p.id}` : undefined" :disabled="!p.ready" variant="outlined" block>
        {{ p.label }}
      </v-btn>
    </div>

    <p class="text-center text-body-2">
      {{ t('auth.noAccount') }}
      <v-btn variant="text" size="small" @click="emit('switchToRegister')">{{ t('auth.signUp') }}</v-btn>
    </p>
  </v-form>
</template>

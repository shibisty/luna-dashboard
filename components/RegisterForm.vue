<script setup lang="ts">
const emit = defineEmits<{ switchToLogin: [] }>()
const { t } = useI18n()
const { register, pending } = useAuth()

const email = ref('')
const password = ref('')
const repeatPassword = ref('')
const error = ref('')

const validationError = computed(() => {
  if (password.value && password.value.length < 6) return t('auth.passwordTooShort')
  if (repeatPassword.value && repeatPassword.value !== password.value) return t('auth.passwordsDontMatch')
  return ''
})

async function onSubmit() {
  error.value = ''
  if (validationError.value) return
  try {
    await register(email.value, password.value)
    await navigateTo('/dashboard')
  } catch (e: any) {
    error.value = e?.data?.statusMessage || 'Error'
  }
}
</script>

<template>
  <v-form @submit.prevent="onSubmit">
    <h1 class="text-h5 mb-4">{{ t('auth.registerTitle') }}</h1>

    <v-text-field v-model="email" :label="t('auth.email')" type="email" required class="mb-2" />

    <v-text-field v-model="password" :label="t('auth.password')" type="password" required :hint="t('auth.passwordTooShort')" persistent-hint class="mb-2" />
    <PasswordStrengthMeter :password="password" />

    <v-text-field v-model="repeatPassword" :label="t('auth.repeatPassword')" type="password" required class="mt-4 mb-2" />

    <v-alert v-if="validationError" type="error" density="compact" class="mb-4">{{ validationError }}</v-alert>
    <v-alert v-else-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

    <v-btn type="submit" color="primary" block :loading="pending" :disabled="Boolean(validationError)" class="mb-4">
      {{ t('auth.signUp') }}
    </v-btn>

    <p class="text-center text-body-2">
      {{ t('auth.haveAccount') }}
      <v-btn variant="text" size="small" @click="emit('switchToLogin')">{{ t('auth.signIn') }}</v-btn>
    </p>
  </v-form>
</template>

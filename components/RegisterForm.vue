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
  <form class="form" @submit.prevent="onSubmit">
    <h1>{{ t('auth.registerTitle') }}</h1>

    <div class="field">
      <label for="reg-email">{{ t('auth.email') }}</label>
      <input id="reg-email" v-model="email" type="email" required autocomplete="email" />
    </div>

    <div class="field">
      <label for="reg-password">{{ t('auth.password') }}</label>
      <input id="reg-password" v-model="password" type="password" required autocomplete="new-password" />
      <PasswordStrengthMeter :password="password" />
      <p class="help-text">{{ t('auth.passwordTooShort') }}</p>
    </div>

    <div class="field">
      <label for="reg-repeat">{{ t('auth.repeatPassword') }}</label>
      <input id="reg-repeat" v-model="repeatPassword" type="password" required autocomplete="new-password" />
    </div>

    <p v-if="validationError" class="error-text">{{ validationError }}</p>
    <p v-else-if="error" class="error-text">{{ error }}</p>

    <button class="btn btn-primary btn-block" type="submit" :disabled="pending || Boolean(validationError)">
      {{ t('auth.signUp') }}
    </button>

    <p class="switch-text">
      {{ t('auth.haveAccount') }}
      <button type="button" class="link-btn" @click="emit('switchToLogin')">{{ t('auth.signIn') }}</button>
    </p>
  </form>
</template>

<style scoped>
.form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.link-btn {
  background: none;
  border: none;
  color: var(--accent);
  font-size: 13px;
  cursor: pointer;
  padding: 0;
}

.switch-text {
  text-align: center;
  font-size: 13px;
  color: var(--ink-soft);
  margin: 0;
}
</style>

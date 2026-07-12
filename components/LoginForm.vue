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
  <form class="form" @submit.prevent="onSubmit">
    <h1>{{ t('auth.loginTitle') }}</h1>

    <div class="field">
      <label for="login-email">{{ t('auth.email') }}</label>
      <input id="login-email" v-model="email" type="email" required autocomplete="email" />
    </div>

    <div class="field">
      <label for="login-password">{{ t('auth.password') }}</label>
      <input id="login-password" v-model="password" type="password" required autocomplete="current-password" />
    </div>

    <button type="button" class="link-btn" @click="emit('switchToForgot')">{{ t('auth.forgotPassword') }}</button>

    <p v-if="error" class="error-text">{{ error }}</p>

    <button class="btn btn-primary btn-block" type="submit" :disabled="pending">
      {{ t('auth.signIn') }}
    </button>

    <div class="divider mono">{{ t('auth.orContinueWith') }}</div>

    <div class="providers">
      <a v-for="p in providers.filter((x) => x.ready)" :key="p.id" class="btn btn-block" :href="`/api/auth/${p.id}`">
        {{ p.label }}
      </a>
      <button
        v-for="p in providers.filter((x) => !x.ready)"
        :key="p.id"
        class="btn btn-block"
        type="button"
        disabled
        title="Coming soon"
      >
        {{ p.label }}
      </button>
    </div>

    <p class="switch-text">
      {{ t('auth.noAccount') }}
      <button type="button" class="link-btn" @click="emit('switchToRegister')">{{ t('auth.signUp') }}</button>
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
  align-self: flex-start;
}

.divider {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: var(--ink-soft);
  text-transform: uppercase;
}

.divider::before,
.divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--line);
}

.providers {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.providers .btn {
  text-align: center;
  text-decoration: none;
}

.providers .btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.switch-text {
  text-align: center;
  font-size: 13px;
  color: var(--ink-soft);
  margin: 0;
}
</style>

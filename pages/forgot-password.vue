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
  <div class="stage">
    <div class="card auth-card">
      <div class="stamp">MAIL</div>

      <form v-if="!sent" class="form" @submit.prevent="onSubmit">
        <h1>{{ t('auth.forgotPassword') }}</h1>
        <div class="field">
          <label for="fp-email">{{ t('auth.email') }}</label>
          <input id="fp-email" v-model="email" type="email" required autocomplete="email" />
        </div>
        <button class="btn btn-primary btn-block" type="submit" :disabled="loading">
          {{ t('auth.sendNewPassword') }}
        </button>
        <NuxtLink to="/" class="link">{{ t('auth.backToLogin') }}</NuxtLink>
      </form>

      <div v-else class="sent">
        <p>{{ t('auth.resetSent') }}</p>
        <NuxtLink to="/" class="link">{{ t('auth.backToLogin') }}</NuxtLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.stage {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.auth-card {
  width: 100%;
  max-width: 380px;
  padding: 32px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.link {
  text-align: center;
  font-size: 13px;
  color: var(--accent);
  text-decoration: none;
}

.sent {
  display: flex;
  flex-direction: column;
  gap: 16px;
  text-align: center;
}
</style>

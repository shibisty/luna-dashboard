<script setup lang="ts">
const mode = ref<'login' | 'register'>('login')
</script>

<template>
  <div class="stage">
    <div class="card auth-card">
      <div class="stamp">AUTH</div>
      <div class="form-wrap">
        <LoginForm
          v-if="mode === 'login'"
          @switch-to-register="mode = 'register'"
          @switch-to-forgot="navigateTo('/forgot-password')"
        />
        <RegisterForm v-else @switch-to-login="mode = 'login'" />
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

.form-wrap {
  display: flex;
  flex-direction: column;
}
</style>

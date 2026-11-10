<script setup lang="ts">
const mode = ref<'login' | 'register'>('login')
</script>

<template>
  <v-container class="fill-height" fluid>
    <v-row justify="center" align="center">
      <v-col cols="12" sm="8" md="5" lg="4">
        <v-card class="pa-8" elevation="2">
          <LoginForm
            v-if="mode === 'login'"
            @switch-to-register="mode = 'register'"
            @switch-to-forgot="navigateTo('/forgot-password')"
          />
          <RegisterForm v-else @switch-to-login="mode = 'login'" />
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

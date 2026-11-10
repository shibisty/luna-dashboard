<script setup lang="ts">
import { API_KEY_SCOPES } from '~~/shared/scopes'

const props = defineProps<{
  mode: 'create' | 'edit'
  initial?: { name: string; allowedDomains: string[]; allowedIps: string[]; scopes: string[] }
}>()
const emit = defineEmits<{
  submit: [payload: { name: string; domains: string; ips: string; scopes: string[] }]
  cancel: []
}>()
const { t } = useI18n()

const name = ref(props.initial?.name ?? '')
const domains = ref(props.initial?.allowedDomains.join(', ') ?? '')
const ips = ref(props.initial?.allowedIps.join(', ') ?? '')
const scopes = ref<string[]>(props.initial?.scopes ?? [])
const error = ref('')
const pending = ref(false)

async function onSubmit() {
  error.value = ''
  if (!name.value.trim()) {
    error.value = t('apiKeys.name')
    return
  }
  if (scopes.value.length === 0) {
    error.value = t('apiKeys.scopes')
    return
  }
  pending.value = true
  try {
    emit('submit', { name: name.value.trim(), domains: domains.value, ips: ips.value, scopes: scopes.value })
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <v-form @submit.prevent="onSubmit">
    <v-text-field v-model="name" :label="t('apiKeys.name')" required class="mb-2" />
    <v-text-field v-model="domains" :label="t('apiKeys.domains')" placeholder="example.com, app.example.com" class="mb-2" />
    <v-text-field v-model="ips" :label="t('apiKeys.ips')" placeholder="203.0.113.10, 203.0.113.11" class="mb-2" />

    <v-select
      v-model="scopes"
      :items="API_KEY_SCOPES"
      item-title="label"
      item-value="value"
      :label="t('apiKeys.scopes')"
      multiple
      chips
      closable-chips
      class="mb-2"
    />

    <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>

    <div class="d-flex justify-end ga-2">
      <v-btn variant="text" @click="emit('cancel')">{{ t('apiKeys.cancel') }}</v-btn>
      <v-btn type="submit" color="primary" :loading="pending">{{ t('apiKeys.save') }}</v-btn>
    </div>
  </v-form>
</template>

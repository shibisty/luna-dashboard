<script setup lang="ts">
const { t } = useI18n()
const { keys, deleteKey, regenerateKey } = useApiKeys()

const emit = defineEmits<{ edit: [id: string]; regenerated: [payload: { publicKey: string; privateKey: string }] }>()

async function onDelete(id: string) {
  if (!confirm(t('apiKeys.confirmDelete'))) return
  await deleteKey(id)
}

async function onRegenerate(id: string, publicKey: string) {
  if (!confirm(t('apiKeys.confirmRegenerate'))) return
  const result = await regenerateKey(id)
  emit('regenerated', { publicKey, privateKey: result.privateKey })
}
</script>

<template>
  <p v-if="keys.length === 0" class="text-body-2 text-medium-emphasis">{{ t('apiKeys.noKeys') }}</p>

  <v-card v-for="key in keys" :key="key.id" class="pa-5 mb-3" variant="outlined">
    <div class="d-flex justify-space-between flex-wrap ga-4">
      <div>
        <h3 class="text-subtitle-1">{{ key.name }}</h3>
        <p class="text-caption text-medium-emphasis mb-2">{{ key.publicKey }}</p>

        <div class="d-flex flex-wrap ga-1 mb-2">
          <v-chip v-for="s in key.scopes" :key="s" size="x-small" variant="outlined">{{ s }}</v-chip>
        </div>

        <p class="text-caption text-medium-emphasis mb-0">
          <span v-if="key.allowedDomains.length">{{ t('apiKeys.domains') }}: {{ key.allowedDomains.join(', ') }}</span>
          <span v-if="key.allowedIps.length"> · {{ t('apiKeys.ips') }}: {{ key.allowedIps.join(', ') }}</span>
          <span v-if="!key.allowedDomains.length && !key.allowedIps.length">{{ t('apiKeys.noRestrictions') }}</span>
        </p>
      </div>

      <div class="d-flex align-start flex-wrap ga-2">
        <v-btn size="small" variant="outlined" @click="emit('edit', key.id)">{{ t('apiKeys.edit') }}</v-btn>
        <v-btn size="small" variant="outlined" @click="onRegenerate(key.id, key.publicKey)">
          {{ t('apiKeys.regenerate') }}
        </v-btn>
        <v-btn size="small" variant="outlined" color="error" @click="onDelete(key.id)">
          {{ t('apiKeys.delete') }}
        </v-btn>
      </div>
    </div>
  </v-card>
</template>

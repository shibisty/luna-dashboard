<script setup lang="ts">
const props = defineProps<{ publicKey: string; privateKey: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const copied = ref<'public' | 'private' | null>(null)

async function copy(value: string, which: 'public' | 'private') {
  await navigator.clipboard.writeText(value)
  copied.value = which
  setTimeout(() => (copied.value = null), 1500)
}
</script>

<template>
  <v-dialog :model-value="true" max-width="480" persistent>
    <v-card class="pa-6">
      <h2 class="text-h6 mb-2">{{ t('apiKeys.keyCreatedTitle') }}</h2>
      <v-alert type="warning" density="compact" variant="tonal" class="mb-4">
        {{ t('apiKeys.keyCreatedWarning') }}
      </v-alert>

      <div class="mb-4">
        <p class="text-caption text-medium-emphasis mb-1">{{ t('apiKeys.publicKey') }}</p>
        <div class="d-flex ga-2 align-center">
          <code class="flex-grow-1 text-caption">{{ publicKey }}</code>
          <v-btn size="small" variant="text" @click="copy(publicKey, 'public')">
            {{ copied === 'public' ? '✓' : t('apiKeys.copy') }}
          </v-btn>
        </div>
      </div>

      <div class="mb-6">
        <p class="text-caption text-medium-emphasis mb-1">{{ t('apiKeys.privateKey') }}</p>
        <div class="d-flex ga-2 align-center">
          <code class="flex-grow-1 text-caption">{{ privateKey }}</code>
          <v-btn size="small" variant="text" @click="copy(privateKey, 'private')">
            {{ copied === 'private' ? '✓' : t('apiKeys.copy') }}
          </v-btn>
        </div>
      </div>

      <v-btn color="primary" block @click="emit('close')">{{ t('apiKeys.gotIt') }}</v-btn>
    </v-card>
  </v-dialog>
</template>

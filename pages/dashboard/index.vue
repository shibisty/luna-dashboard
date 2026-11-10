<script setup lang="ts">
definePageMeta({ layout: 'dashboard' })

const { t } = useI18n()
const { user } = useAuth()
const { keys, fetchKeys, createKey, updateKey } = useApiKeys()

try {
  await fetchKeys()
} catch (e) {
  console.error('Failed to load API keys', e)
}

const showForm = ref(false)
const editingId = ref<string | null>(null)
const reveal = ref<{ publicKey: string; privateKey: string } | null>(null)

const editingKey = computed(() => keys.value.find((k) => k.id === editingId.value) ?? null)

function openCreate() {
  editingId.value = null
  showForm.value = true
}
function openEdit(id: string) {
  editingId.value = id
  showForm.value = true
}

async function onSubmit(payload: { name: string; domains: string; ips: string; scopes: string[] }) {
  if (editingId.value) {
    await updateKey(editingId.value, payload)
  } else {
    const created = await createKey(payload)
    reveal.value = { publicKey: created.publicKey, privateKey: created.privateKey }
  }
  showForm.value = false
  editingId.value = null
}

function onRegenerated(payload: { publicKey: string; privateKey: string }) {
  reveal.value = payload
}
</script>

<template>
  <v-container class="py-8" style="max-width: 880px; margin-inline: auto">
    <div class="mb-8">
      <h1 class="text-h5">{{ t('dashboard.welcome') }}</h1>
      <p class="text-caption text-medium-emphasis">{{ t('dashboard.signedInAs') }}: {{ user?.email }}</p>
    </div>

    <div class="d-flex justify-space-between align-center mb-4">
      <h2 class="text-h6">{{ t('apiKeys.title') }}</h2>
      <v-btn color="primary" @click="openCreate">{{ t('apiKeys.create') }}</v-btn>
    </div>

    <v-card v-if="showForm" class="pa-6 mb-4">
      <ApiKeyForm
        :mode="editingId ? 'edit' : 'create'"
        :initial="
          editingKey
            ? {
                name: editingKey.name,
                allowedDomains: editingKey.allowedDomains,
                allowedIps: editingKey.allowedIps,
                scopes: editingKey.scopes,
              }
            : undefined
        "
        @submit="onSubmit"
        @cancel="((showForm = false), (editingId = null))"
      />
    </v-card>

    <ApiKeyList @edit="openEdit" @regenerated="onRegenerated" />

    <ApiKeySecretReveal
      v-if="reveal"
      :public-key="reveal.publicKey"
      :private-key="reveal.privateKey"
      @close="reveal = null"
    />
  </v-container>
</template>

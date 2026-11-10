<script setup lang="ts">
definePageMeta({ layout: 'dashboard' })

const { t } = useI18n()

interface EndUser {
  id: string
  email: string
  apiKeyName: string
  scopes: string[]
  registeredAt: string
  lastLoginAt: string | null
  status: 'active' | 'blocked'
}

// Заглушка — позже заменится на $fetch('/api/end-users')
const mockUsers: EndUser[] = [
  {
    id: '1',
    email: 'anna.k@example.com',
    apiKeyName: 'Mobile App',
    scopes: ['users:read', 'dashboard:read'],
    registeredAt: '2026-06-02T10:12:00Z',
    lastLoginAt: '2026-07-13T08:45:00Z',
    status: 'active',
  },
  {
    id: '2',
    email: 'oleg.p@example.com',
    apiKeyName: 'Web Widget',
    scopes: ['users:read'],
    registeredAt: '2026-05-21T14:30:00Z',
    lastLoginAt: '2026-07-10T19:02:00Z',
    status: 'active',
  },
  {
    id: '3',
    email: 'irina.s@example.com',
    apiKeyName: 'Mobile App',
    scopes: ['users:read', 'billing:read'],
    registeredAt: '2026-04-11T09:00:00Z',
    lastLoginAt: null,
    status: 'blocked',
  },
]

const search = ref('')
const apiKeyFilter = ref<string | null>(null)

const apiKeyOptions = computed(() => [...new Set(mockUsers.map((u) => u.apiKeyName))])

const filteredUsers = computed(() =>
  mockUsers.filter((u) => {
    const matchesSearch = !search.value || u.email.toLowerCase().includes(search.value.toLowerCase())
    const matchesKey = !apiKeyFilter.value || u.apiKeyName === apiKeyFilter.value
    return matchesSearch && matchesKey
  })
)

const headers = [
  { title: t('users.email'), key: 'email' },
  { title: t('users.apiKey'), key: 'apiKeyName' },
  { title: t('users.scopes'), key: 'scopes', sortable: false },
  { title: t('users.registeredAt'), key: 'registeredAt' },
  { title: t('users.lastLoginAt'), key: 'lastLoginAt' },
  { title: t('users.status'), key: 'status' },
  { title: '', key: 'actions', sortable: false, align: 'end' as const },
]

function formatDate(value: string | null) {
  if (!value) return '—'
  return new Date(value).toLocaleString()
}

function onBlock(userItem: EndUser) {
  userItem.status = userItem.status === 'active' ? 'blocked' : 'active'
}
</script>

<template>
  <v-container class="py-8" fluid>
    <div class="d-flex justify-space-between align-center flex-wrap ga-4 mb-6">
      <div>
        <h1 class="text-h5">{{ t('users.title') }}</h1>
        <p class="text-caption text-medium-emphasis">{{ t('users.subtitle') }}</p>
      </div>
    </div>

    <div class="d-flex flex-wrap ga-4 mb-4">
      <v-text-field
        v-model="search"
        :label="t('users.search')"
        prepend-inner-icon="mdi-magnify"
        density="compact"
        variant="outlined"
        hide-details
        style="max-width: 280px"
      />

      <v-select
        v-model="apiKeyFilter"
        :items="apiKeyOptions"
        :label="t('users.filterByKey')"
        density="compact"
        variant="outlined"
        clearable
        hide-details
        style="max-width: 220px"
      />
    </div>

    <v-card variant="outlined">
      <v-data-table :headers="headers" :items="filteredUsers" :items-per-page="10">
        <template #item.scopes="{ item }">
          <div class="d-flex flex-wrap ga-1">
            <v-chip v-for="s in item.scopes" :key="s" size="x-small" variant="outlined">{{ s }}</v-chip>
          </div>
        </template>

        <template #item.registeredAt="{ item }">
          {{ formatDate(item.registeredAt) }}
        </template>

        <template #item.lastLoginAt="{ item }">
          {{ formatDate(item.lastLoginAt) }}
        </template>

        <template #item.status="{ item }">
          <v-chip
            size="small"
            :color="item.status === 'active' ? 'success' : 'error'"
            variant="tonal"
          >
            {{ t(`users.statusValues.${item.status}`) }}
          </v-chip>
        </template>

        <template #item.actions="{ item }">
          <v-btn
            size="small"
            variant="text"
            :color="item.status === 'active' ? 'error' : 'success'"
            @click="onBlock(item)"
          >
            {{ item.status === 'active' ? t('users.block') : t('users.unblock') }}
          </v-btn>
        </template>

        <template #no-data>
          <p class="text-body-2 text-medium-emphasis pa-4">{{ t('users.noUsers') }}</p>
        </template>
      </v-data-table>
    </v-card>
  </v-container>
</template>

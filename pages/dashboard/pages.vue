<script setup lang="ts">
import type { PageNode } from '~/components/PageTreeNode.vue'

definePageMeta({ layout: 'dashboard' })

const { t } = useI18n()

// Заглушка — позже заменится на $fetch('/api/pages') + $fetch(PATCH) при реордере
const tree = ref<PageNode[]>([
  {
    id: '1',
    title: 'Home',
    slug: '',
    children: [],
  },
  {
    id: '2',
    title: 'Docs',
    slug: 'docs',
    children: [
      { id: '2-1', title: 'Getting started', slug: 'docs/getting-started', children: [] },
      {
        id: '2-2',
        title: 'API Reference',
        slug: 'docs/api',
        children: [
          { id: '2-2-1', title: 'Authentication', slug: 'docs/api/authentication', children: [] },
          { id: '2-2-2', title: 'Users', slug: 'docs/api/users', children: [] },
        ],
      },
    ],
  },
  {
    id: '3',
    title: 'Pricing',
    slug: 'pricing',
    children: [],
  },
])

const dialogOpen = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingNode = ref<PageNode | null>(null)
const parentForNewNode = ref<PageNode | null>(null)

const formTitle = ref('')
const formSlug = ref('')

function openCreateRoot() {
  dialogMode.value = 'create'
  editingNode.value = null
  parentForNewNode.value = null
  formTitle.value = ''
  formSlug.value = ''
  dialogOpen.value = true
}

function openAddChild(node: PageNode) {
  dialogMode.value = 'create'
  editingNode.value = null
  parentForNewNode.value = node
  formTitle.value = ''
  formSlug.value = ''
  dialogOpen.value = true
}

function openEdit(node: PageNode) {
  dialogMode.value = 'edit'
  editingNode.value = node
  parentForNewNode.value = null
  formTitle.value = node.title
  formSlug.value = node.slug
  dialogOpen.value = true
}

function onDelete(node: PageNode) {
  if (!confirm(t('pages.confirmDelete'))) return

  function removeFrom(nodes: PageNode[]): boolean {
    const idx = nodes.findIndex((n) => n.id === node.id)
    if (idx !== -1) {
      nodes.splice(idx, 1)
      return true
    }
    return nodes.some((n) => removeFrom(n.children))
  }

  removeFrom(tree.value)
}

function onSubmit() {
  if (!formTitle.value.trim()) return

  if (dialogMode.value === 'edit' && editingNode.value) {
    editingNode.value.title = formTitle.value.trim()
    editingNode.value.slug = formSlug.value.trim()
  } else {
    const newNode: PageNode = {
      id: crypto.randomUUID(),
      title: formTitle.value.trim(),
      slug: formSlug.value.trim(),
      children: [],
    }
    if (parentForNewNode.value) {
      parentForNewNode.value.children.push(newNode)
    } else {
      tree.value.push(newNode)
    }
  }

  dialogOpen.value = false
}
</script>

<template>
  <v-container class="py-8" fluid>
    <div class="d-flex justify-space-between align-center flex-wrap ga-4 mb-6">
      <div>
        <h1 class="text-h5">{{ t('pages.title') }}</h1>
        <p class="text-caption text-medium-emphasis">{{ t('pages.subtitle') }}</p>
      </div>
      <v-btn color="primary" prepend-icon="mdi-plus" @click="openCreateRoot">{{ t('pages.addRoot') }}</v-btn>
    </div>

    <v-card variant="outlined" class="pa-2">
      <PageTreeNode
        :nodes="tree"
        @update:nodes="(n) => (tree = n)"
        @edit="openEdit"
        @delete="onDelete"
        @add-child="openAddChild"
      />

      <p v-if="tree.length === 0" class="text-body-2 text-medium-emphasis pa-4">{{ t('pages.noPages') }}</p>
    </v-card>

    <v-dialog v-model="dialogOpen" max-width="440">
      <v-card class="pa-6">
        <h2 class="text-h6 mb-4">
          {{ dialogMode === 'edit' ? t('pages.editPage') : t('pages.newPage') }}
        </h2>

        <v-text-field v-model="formTitle" :label="t('pages.pageTitle')" class="mb-2" autofocus />
        <v-text-field v-model="formSlug" :label="t('pages.pageSlug')" prefix="/" class="mb-4" />

        <div class="d-flex justify-end ga-2">
          <v-btn variant="text" @click="dialogOpen = false">{{ t('pages.cancel') }}</v-btn>
          <v-btn color="primary" @click="onSubmit">{{ t('pages.save') }}</v-btn>
        </div>
      </v-card>
    </v-dialog>
  </v-container>
</template>

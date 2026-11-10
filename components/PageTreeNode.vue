<script setup lang="ts">
import draggable from 'vuedraggable'

export interface PageNode {
  id: string
  title: string
  slug: string
  children: PageNode[]
}

const props = defineProps<{ nodes: PageNode[]; depth?: number }>()
const emit = defineEmits<{
  'update:nodes': [nodes: PageNode[]]
  edit: [node: PageNode]
  delete: [node: PageNode]
  addChild: [node: PageNode]
}>()

const depth = props.depth ?? 0
const expanded = ref<Record<string, boolean>>({})

function toggle(id: string) {
  expanded.value[id] = !expanded.value[id]
}

function onListChange(newNodes: PageNode[]) {
  emit('update:nodes', newNodes)
}

function onChildChange(node: PageNode, children: PageNode[]) {
  node.children = children
}
</script>

<template>
  <draggable
    :model-value="nodes"
    item-key="id"
    group="pages-tree"
    handle=".drag-handle"
    ghost-class="ghost-row"
    :animation="150"
    @update:model-value="onListChange"
  >
    <template #item="{ element: node }">
      <div class="tree-node">
        <div class="tree-row" :style="{ paddingLeft: `${depth * 24}px` }">
          <v-icon class="drag-handle" icon="mdi-drag-vertical" size="18" />

          <v-btn
            v-if="node.children.length > 0"
            :icon="expanded[node.id] ? 'mdi-chevron-down' : 'mdi-chevron-right'"
            variant="text"
            size="x-small"
            @click="toggle(node.id)"
          />
          <span v-else class="chevron-spacer" />

          <span class="node-title">{{ node.title }}</span>
          <span class="node-slug mono text-caption text-medium-emphasis">/{{ node.slug }}</span>

          <v-spacer />

          <v-btn icon="mdi-plus" variant="text" size="x-small" @click="emit('addChild', node)" />
          <v-btn icon="mdi-pencil" variant="text" size="x-small" @click="emit('edit', node)" />
          <v-btn icon="mdi-delete" variant="text" size="x-small" @click="emit('delete', node)" />
        </div>

        <PageTreeNode
          v-if="node.children.length > 0 && (expanded[node.id] ?? true)"
          :nodes="node.children"
          :depth="depth + 1"
          @update:nodes="(children) => onChildChange(node, children)"
          @edit="(n) => emit('edit', n)"
          @delete="(n) => emit('delete', n)"
          @add-child="(n) => emit('addChild', n)"
        />
      </div>
    </template>
  </draggable>
</template>

<style scoped>
.tree-node {
  width: 100%;
}

.tree-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-top: 4px;
  padding-bottom: 4px;
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.drag-handle {
  cursor: grab;
  opacity: 0.5;
}

.chevron-spacer {
  width: 24px;
  display: inline-block;
}

.node-title {
  font-size: 14px;
  margin-left: 4px;
}

.node-slug {
  margin-left: 8px;
}

.ghost-row {
  opacity: 0.4;
  background: rgba(var(--v-theme-primary), 0.08);
}
</style>

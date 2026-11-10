<script setup lang="ts">
import zxcvbn from 'zxcvbn'

const props = defineProps<{ password: string }>()
const { t } = useI18n()

const score = computed(() => (props.password ? zxcvbn(props.password).score : 0))
const colors = ['error', 'error', 'warning', 'success', 'success']
</script>

<template>
  <div v-if="password">
    <v-progress-linear :model-value="(score + 1) * 20" :color="colors[score]" height="4" rounded />
    <span class="text-caption" :class="`text-${colors[score]}`">{{ t(`auth.passwordStrength.${score}`) }}</span>
  </div>
</template>

<script setup lang="ts">
import zxcvbn from 'zxcvbn'

const props = defineProps<{ password: string }>()
const { t } = useI18n()

const score = computed(() => (props.password ? zxcvbn(props.password).score : 0))

const colors = ['#b3432b', '#c9722f', '#c9a02f', '#7f9a4f', '#3f6b4f']
</script>

<template>
  <div v-if="password" class="strength">
    <div class="bars">
      <span
        v-for="i in 4"
        :key="i"
        class="bar"
        :style="{ background: i <= score + 1 ? colors[score] : 'var(--line)' }"
      />
    </div>
    <span class="label mono" :style="{ color: colors[score] }">
      {{ t(`auth.passwordStrength.${score}`) }}
    </span>
  </div>
</template>

<style scoped>
.strength {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bars {
  display: flex;
  gap: 4px;
  flex: 1;
}

.bar {
  height: 4px;
  flex: 1;
  border-radius: 2px;
  transition: background-color 0.15s ease;
}

.label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
</style>

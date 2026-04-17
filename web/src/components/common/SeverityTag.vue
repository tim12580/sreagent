<template>
  <n-tag
    :type="getSeverityType(severity)"
    size="small"
    :bordered="bordered"
    round
    class="severity-tag"
    :class="`severity-tag--${severity}`"
  >
    <template v-if="dot" #icon>
      <span class="severity-tag__dot" :class="{ 'is-critical': severity === 'critical' }" />
    </template>
    <slot>{{ severity }}</slot>
  </n-tag>
</template>

<script setup lang="ts">
import { NTag } from 'naive-ui'
import { getSeverityType } from '@/utils/alert'

withDefaults(defineProps<{
  severity: string
  bordered?: boolean
  dot?: boolean
}>(), {
  bordered: true,
  dot: true,
})
</script>

<style scoped>
.severity-tag {
  font-weight: var(--sre-fw-semibold);
  letter-spacing: 0.02em;
  text-transform: capitalize;
}

.severity-tag__dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  margin-right: 1px;
  vertical-align: middle;
}
.severity-tag__dot.is-critical {
  animation: sre-pulse-dot 1.4s ease-in-out infinite;
}
</style>

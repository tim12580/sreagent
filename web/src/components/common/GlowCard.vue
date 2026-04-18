<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  variant?: 'default' | 'critical' | 'success' | 'accent'
  interactive?: boolean
  tilt?: boolean
  glow?: boolean
  conic?: boolean | 'critical' | 'strong'
  padding?: string
}>(), {
  variant: 'default',
  interactive: false,
  tilt: false,
  glow: false,
  conic: false,
  padding: 'var(--sre-space-6)',
})

const classes = computed(() => [
  'glow-card',
  'surface-glass-strong',
  'noise-overlay',
  props.interactive && 'glow-card--interactive',
  props.tilt && 'glow-card--tilt',
  props.glow && `glow-${props.variant}`,
  props.conic === true && 'conic-border',
  props.conic === 'critical' && 'conic-border conic-border--critical',
  props.conic === 'strong' && 'conic-border conic-border--strong',
])
</script>

<template>
  <div :class="classes" :style="{ padding }">
    <slot />
  </div>
</template>

<style scoped>
.glow-card {
  border-radius: var(--sre-radius-lg);
  position: relative;
  transition:
    box-shadow var(--sre-duration-base) var(--sre-ease-out),
    border-color var(--sre-duration-base) var(--sre-ease-out),
    transform     var(--sre-duration-base) var(--sre-ease-out);
  overflow: visible;
  transform-style: preserve-3d;
}

/* Pure-CSS tilt on hover — no JS, no rAF, no mousemove listeners */
.glow-card--tilt:hover {
  transform: perspective(900px) rotateX(1.5deg) rotateY(-1deg) translateY(-2px);
  box-shadow: var(--sre-shadow-soft-xl);
}
.glow-card--tilt {
  transition:
    box-shadow var(--sre-duration-base) var(--sre-ease-out),
    border-color var(--sre-duration-base) var(--sre-ease-out),
    transform 360ms cubic-bezier(0.34, 1.56, 0.64, 1);
}

.glow-card--interactive { cursor: pointer; }
.glow-card--interactive:hover { border-color: var(--sre-border-strong); }
</style>

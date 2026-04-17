<template>
  <div class="page-header" :class="{ 'page-header--flex': hasActions }">
    <div class="page-header__lede">
      <span v-if="!hideAccent" class="page-header__accent" />
      <div>
        <h2 class="page-title">{{ title }}</h2>
        <p v-if="subtitle" class="page-subtitle">{{ subtitle }}</p>
      </div>
    </div>
    <div v-if="hasActions" class="header-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useSlots } from 'vue'

withDefaults(defineProps<{
  title: string
  subtitle?: string
  hideAccent?: boolean
}>(), {
  hideAccent: false,
})

const slots = useSlots()
const hasActions = !!slots.actions
</script>

<style scoped>
.page-header {
  margin-bottom: var(--sre-space-6);
  animation: sre-fade-in var(--sre-duration-slow) var(--sre-ease-out) both;
}

.page-header--flex {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: var(--sre-space-4);
  flex-wrap: wrap;
}

.page-header__lede {
  display: flex;
  align-items: flex-start;
  gap: var(--sre-space-3);
}

.page-header__accent {
  flex-shrink: 0;
  width: 4px;
  min-height: 38px;
  border-radius: var(--sre-radius-pill);
  background: var(--sre-gradient-brand);
  box-shadow: 0 0 12px rgba(24, 160, 88, 0.35);
  margin-top: 2px;
}

.page-title {
  font-size: var(--sre-fs-2xl);
  font-weight: var(--sre-fw-semibold);
  line-height: var(--sre-lh-tight);
  margin: 0 0 4px 0;
  color: var(--sre-text-primary);
  letter-spacing: -0.01em;
}

.page-subtitle {
  font-size: var(--sre-fs-md);
  color: var(--sre-text-secondary);
  line-height: var(--sre-lh-snug);
  margin: 0;
  max-width: 680px;
}

.header-actions {
  display: flex;
  gap: var(--sre-space-2);
  align-items: center;
  flex-wrap: wrap;
}
</style>

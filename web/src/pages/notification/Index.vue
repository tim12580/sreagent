<script setup lang="ts">
import { ref, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/common/PageHeader.vue'
import AlertChannels from './AlertChannels.vue'
import Rules from './Rules.vue'
import Media from './Media.vue'
import Templates from './Templates.vue'
import Subscribe from './Subscribe.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

// Map hash/query → tab name
const tabFromRoute = () => {
  const tab = (route.query.tab as string) || ''
  const valid = ['channels', 'rules', 'media', 'templates', 'subscribe']
  return valid.includes(tab) ? tab : 'channels'
}

const activeTab = ref(tabFromRoute())

watchEffect(() => {
  const tab = tabFromRoute()
  if (tab !== activeTab.value) activeTab.value = tab
})

function handleTabChange(tab: string) {
  activeTab.value = tab
  router.replace({ path: '/notification', query: { tab } })
}
</script>

<template>
  <div class="notification-page">
    <PageHeader :title="t('menu.notification')" :subtitle="t('notification.subtitle')" />

    <n-card :bordered="false" class="content-card">
      <n-tabs :value="activeTab" type="line" animated @update:value="handleTabChange">
        <n-tab-pane name="channels" :tab="t('menu.alertChannels')">
          <AlertChannels />
        </n-tab-pane>
        <n-tab-pane name="rules" :tab="t('menu.notifyRules')">
          <Rules />
        </n-tab-pane>
        <n-tab-pane name="media" :tab="t('menu.notifyMedia')">
          <Media />
        </n-tab-pane>
        <n-tab-pane name="templates" :tab="t('menu.templates')">
          <Templates />
        </n-tab-pane>
        <n-tab-pane name="subscribe" :tab="t('menu.subscriptions')">
          <Subscribe />
        </n-tab-pane>
      </n-tabs>
    </n-card>
  </div>
</template>

<style scoped>
.notification-page {
  max-width: 1400px;
}
.content-card {
  border-radius: 12px;
}
</style>

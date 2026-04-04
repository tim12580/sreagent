<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/common/PageHeader.vue'
import UserManagement from './UserManagement.vue'
import TeamManagement from './TeamManagement.vue'
import VirtualUsers from './VirtualUsers.vue'
import BizGroupManagement from './BizGroupManagement.vue'
import AIConfig from './AIConfig.vue'
import LarkBotConfig from './LarkBotConfig.vue'
import OIDCConfig from './OIDCConfig.vue'

const { t } = useI18n()
const activeTab = ref('users')

// Ref to UserManagement so we can pass usersList to Team and BizGroup tabs
const userMgmtRef = ref<InstanceType<typeof UserManagement> | null>(null)
</script>

<template>
  <div class="settings-page">
    <PageHeader :title="t('settings.title')" :subtitle="t('settings.subtitle')" />

    <n-card :bordered="false" class="content-card">
      <n-tabs v-model:value="activeTab" type="line" animated>
        <n-tab-pane name="users" :tab="t('settings.userManagement')">
          <UserManagement ref="userMgmtRef" />
        </n-tab-pane>

        <n-tab-pane name="teams" :tab="t('settings.teamManagement')">
          <TeamManagement :all-users="userMgmtRef?.usersList ?? []" />
        </n-tab-pane>

        <n-tab-pane name="virtual" :tab="t('settings.virtualUsers')">
          <VirtualUsers />
        </n-tab-pane>

        <n-tab-pane name="bizgroups" :tab="t('bizGroup.title')">
          <BizGroupManagement :all-users="userMgmtRef?.usersList ?? []" />
        </n-tab-pane>

        <n-tab-pane name="ai" :tab="t('settings.aiConfig')">
          <AIConfig />
        </n-tab-pane>

        <n-tab-pane name="larkbot" :tab="t('settings.larkBot')">
          <LarkBotConfig />
        </n-tab-pane>

        <n-tab-pane name="oidc" :tab="t('settings.oidcConfig')">
          <OIDCConfig />
        </n-tab-pane>
      </n-tabs>
    </n-card>
  </div>
</template>

<style scoped>
.settings-page {
  max-width: 1400px;
}

.content-card {
  border-radius: 12px;
}
</style>

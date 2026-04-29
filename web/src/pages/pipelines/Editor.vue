<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton, NSpace, NInput, NSwitch, NDrawer, NDrawerContent,
  NForm, NFormItem, NSelect, NInputNumber, NDivider, NTag, useMessage
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { pipelineApi } from '@/api'
import type { EventPipeline, PipelineNode, Connections, LabelFilter } from '@/types/pipeline'
import { PROCESSOR_TYPES } from '@/types/pipeline'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const message = useMessage()

const isNew = computed(() => route.params.id === 'new')
const pipelineId = computed(() => Number(route.params.id))

const loading = ref(false)
const saving = ref(false)

// Pipeline data
const name = ref('')
const description = ref('')
const disabled = ref(false)
const filterEnable = ref(false)
const labelFilters = ref<LabelFilter[]>([])
const nodes = ref<PipelineNode[]>([])
const connections = ref<Connections>({})

// Editor state
const selectedNodeId = ref<string | null>(null)
const showNodeConfig = ref(false)
const draggingNode = ref<string | null>(null)
const dragOffset = ref({ x: 0, y: 0 })
const connectingFrom = ref<{ nodeId: string; outputIdx: number } | null>(null)

const selectedNode = computed(() =>
  nodes.value.find(n => n.id === selectedNodeId.value) || null
)

const NODE_WIDTH = 180
const NODE_HEIGHT = 60

function getProcessorInfo(type: string) {
  return PROCESSOR_TYPES.find(p => p.value === type) || PROCESSOR_TYPES[0]
}

// --- Node management ---
let nodeCounter = 0
function addNode(type: string) {
  nodeCounter++
  const info = getProcessorInfo(type)
  const node: PipelineNode = {
    id: `node_${Date.now()}_${nodeCounter}`,
    name: `${info.label} ${nodeCounter}`,
    type: type as PipelineNode['type'],
    config: getDefaultConfig(type),
    position: { x: 100 + (nodes.value.length % 3) * 220, y: 80 + Math.floor(nodes.value.length / 3) * 100 },
  }
  nodes.value.push(node)
  selectedNodeId.value = node.id
  showNodeConfig.value = true
}

function getDefaultConfig(type: string): Record<string, any> {
  switch (type) {
    case 'if': return { mode: 'tags', tag_filters: [] }
    case 'relabel': return { configs: [] }
    case 'event_drop': return { tag_filters: [] }
    case 'callback': return { url: '', method: 'POST' }
    case 'ai_summary': return { api_url: '', model: 'gpt-4o-mini' }
    default: return {}
  }
}

function removeNode(nodeId: string) {
  nodes.value = nodes.value.filter(n => n.id !== nodeId)
  const newConns: Connections = {}
  for (const [src, outputs] of Object.entries(connections.value)) {
    if (src === nodeId) continue
    const filtered: Record<number, string[]> = {}
    for (const [idx, targets] of Object.entries(outputs)) {
      const ft = (targets as string[]).filter(t => t !== nodeId)
      if (ft.length > 0) filtered[Number(idx)] = ft
    }
    if (Object.keys(filtered).length > 0) newConns[src] = filtered
  }
  connections.value = newConns
  if (selectedNodeId.value === nodeId) {
    selectedNodeId.value = null
    showNodeConfig.value = false
  }
}

// --- Connection management ---
function startConnect(nodeId: string, outputIdx: number) {
  connectingFrom.value = { nodeId, outputIdx }
}

function finishConnect(targetId: string) {
  if (!connectingFrom.value || connectingFrom.value.nodeId === targetId) {
    connectingFrom.value = null
    return
  }
  const { nodeId: srcId, outputIdx } = connectingFrom.value
  if (!connections.value[srcId]) connections.value[srcId] = {}
  if (!connections.value[srcId][outputIdx]) connections.value[srcId][outputIdx] = []
  if (!connections.value[srcId][outputIdx].includes(targetId)) {
    connections.value[srcId][outputIdx].push(targetId)
  }
  connectingFrom.value = null
}

function removeConnection(srcId: string, outputIdx: number, targetId: string) {
  if (connections.value[srcId]?.[outputIdx]) {
    connections.value[srcId][outputIdx] = connections.value[srcId][outputIdx].filter(t => t !== targetId)
    if (connections.value[srcId][outputIdx].length === 0) {
      delete connections.value[srcId][outputIdx]
    }
    if (Object.keys(connections.value[srcId]).length === 0) {
      delete connections.value[srcId]
    }
  }
}

// --- Canvas drag ---
function onNodeMouseDown(e: MouseEvent, nodeId: string) {
  if ((e.target as HTMLElement).closest('.node-port')) return
  const node = nodes.value.find(n => n.id === nodeId)
  if (!node || !node.position) return
  draggingNode.value = nodeId
  dragOffset.value = { x: e.clientX - node.position.x, y: e.clientY - node.position.y }
  selectedNodeId.value = nodeId
}

function onMouseMove(e: MouseEvent) {
  if (draggingNode.value) {
    const node = nodes.value.find(n => n.id === draggingNode.value)
    if (node?.position) {
      node.position.x = e.clientX - dragOffset.value.x
      node.position.y = e.clientY - dragOffset.value.y
    }
  }
}

function onMouseUp() {
  draggingNode.value = null
}

// --- SVG path for connections ---
function getPath(srcNode: PipelineNode, outputIdx: number, targetId: string): string {
  const tgtNode = nodes.value.find(n => n.id === targetId)
  if (!srcNode.position || !tgtNode?.position) return ''

  const srcX = srcNode.position.x + NODE_WIDTH
  const srcY = srcNode.position.y + NODE_HEIGHT / 2 + (outputIdx === 1 ? 15 : -15)
  const tgtX = tgtNode.position.x
  const tgtY = tgtNode.position.y + NODE_HEIGHT / 2

  const dx = Math.abs(tgtX - srcX) * 0.5
  return `M${srcX},${srcY} C${srcX + dx},${srcY} ${tgtX - dx},${tgtY} ${tgtX},${tgtY}`
}

// --- Load / Save ---
async function loadPipeline() {
  if (isNew.value) return
  loading.value = true
  try {
    const res = await pipelineApi.get(pipelineId.value)
    const p = res.data.data
    name.value = p.name
    description.value = p.description
    disabled.value = p.disabled
    filterEnable.value = p.filter_enable
    labelFilters.value = p.label_filters || []
    nodes.value = p.nodes || []
    connections.value = p.connections || {}
  } catch (err: any) {
    message.error(t('common.loadFailed'))
    router.back()
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!name.value.trim()) {
    message.warning(t('pipeline.nameRequired'))
    return
  }
  saving.value = true
  try {
    const data = {
      name: name.value,
      description: description.value,
      disabled: disabled.value,
      filter_enable: filterEnable.value,
      label_filters: labelFilters.value,
      nodes: nodes.value,
      connections: connections.value,
    }
    if (isNew.value) {
      const res = await pipelineApi.create(data)
      message.success(t('pipeline.created'))
      router.replace({ name: 'PipelineEditor', params: { id: res.data.data.id } })
    } else {
      await pipelineApi.update(pipelineId.value, data)
      message.success(t('pipeline.updated'))
    }
  } catch (err: any) {
    message.error(err.message || t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

function addLabelFilter() {
  labelFilters.value.push({ key: '', op: '==', value: '' })
}

function removeLabelFilter(idx: number) {
  labelFilters.value.splice(idx, 1)
}

onMounted(loadPipeline)
</script>

<template>
  <div class="pipeline-editor" @mousemove="onMouseMove" @mouseup="onMouseUp">
    <!-- Toolbar -->
    <div class="editor-toolbar">
      <NSpace align="center">
        <NButton quaternary @click="router.back()">{{ t('pipeline.back') }}</NButton>
        <NDivider vertical />
        <NInput v-model:value="name" :placeholder="t('pipeline.pipelineName')" style="width: 200px" />
        <NInput v-model:value="description" :placeholder="t('pipeline.pipelineDesc')" style="width: 300px" />
        <span style="font-size: 13px; color: var(--sre-text-secondary)">{{ t('pipeline.disabled') }}</span>
        <NSwitch v-model:value="disabled" size="small" />
        <NDivider vertical />
        <NButton type="primary" :loading="saving" @click="handleSave">{{ t('pipeline.save') }}</NButton>
      </NSpace>
    </div>

    <div class="editor-body">
      <!-- Node palette -->
      <div class="node-palette">
        <div class="palette-title">{{ t('pipeline.processors') }}</div>
        <div
          v-for="pt in PROCESSOR_TYPES"
          :key="pt.value"
          class="palette-item"
          :style="{ borderLeftColor: pt.color }"
          :title="t(`pipeline.proc${pt.value.charAt(0).toUpperCase() + pt.value.slice(1).replace(/_/g, '')}Desc`)"
          @click="addNode(pt.value)"
        >
          {{ pt.label }}
        </div>

        <NDivider />
        <div class="palette-title">{{ t('pipeline.filters') }}</div>
        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
          <NSwitch v-model:value="filterEnable" size="small" />
          <span style="font-size: 12px; color: var(--sre-text-secondary)">{{ t('pipeline.filterEnable') }}</span>
        </div>
        <div v-if="filterEnable" style="font-size: 11px; color: var(--sre-text-tertiary, #aaa); margin-bottom: 8px">
          {{ t('pipeline.filterHint') }}
        </div>

        <template v-if="filterEnable">
          <div v-for="(f, i) in labelFilters" :key="i" class="filter-row">
            <NInput v-model:value="f.key" size="tiny" placeholder="key" style="width: 60px" />
            <NSelect v-model:value="f.op" size="tiny" style="width: 55px"
              :options="['==','!=','=~','!~','in','not_in'].map(v => ({ label: v, value: v }))" />
            <NInput v-model:value="f.value" size="tiny" placeholder="value" style="width: 60px" />
            <NButton size="tiny" quaternary type="error" @click="removeLabelFilter(i)">x</NButton>
          </div>
          <NButton size="tiny" @click="addLabelFilter">{{ t('pipeline.addFilter') }}</NButton>
        </template>

        <!-- Help card -->
        <NDivider />
        <div style="font-size: 11px; color: var(--sre-text-tertiary, #aaa); line-height: 1.6">
          <p><strong>How to use:</strong></p>
          <ol style="padding-left: 16px; margin: 4px 0">
            <li>Click a processor to add it</li>
            <li>Drag nodes to reposition</li>
            <li>Click output port (●) then input port (●) to connect</li>
            <li>Click a node to configure it</li>
            <li>Click connection line to remove</li>
          </ol>
        </div>
      </div>

      <!-- Canvas -->
      <div class="canvas-container">
        <svg class="connections-svg" width="100%" height="100%">
          <defs>
            <marker id="arrowhead" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="#999" />
            </marker>
          </defs>
          <template v-for="node in nodes" :key="node.id">
            <template v-for="(targets, outIdx) in (connections[node.id] || {})" :key="outIdx">
              <path
                v-for="tgt in (targets as string[])"
                :key="tgt"
                :d="getPath(node, Number(outIdx), tgt)"
                stroke="#999"
                stroke-width="2"
                fill="none"
                marker-end="url(#arrowhead)"
                class="connection-path"
                @click="removeConnection(node.id, Number(outIdx), tgt)"
              />
            </template>
          </template>

          <!-- Connecting line preview -->
          <line
            v-if="connectingFrom"
            :x1="nodes.find(n => n.id === connectingFrom?.nodeId)?.position?.x ?? 0"
            :y1="nodes.find(n => n.id === connectingFrom?.nodeId)?.position?.y ?? 0"
            :x2="0"
            :y2="0"
            stroke="#18a058"
            stroke-width="2"
            stroke-dasharray="5,5"
          />
        </svg>

        <div
          v-for="node in nodes"
          :key="node.id"
          class="pipeline-node"
          :class="{ selected: selectedNodeId === node.id, disabled: node.disabled }"
          :style="{
            left: (node.position?.x || 0) + 'px',
            top: (node.position?.y || 0) + 'px',
            borderColor: getProcessorInfo(node.type).color,
          }"
          @mousedown="onNodeMouseDown($event, node.id)"
        >
          <!-- Input port -->
          <div
            class="node-port input-port"
            @mouseup="finishConnect(node.id)"
          />

          <div class="node-header" :style="{ background: getProcessorInfo(node.type).color + '20' }">
            <span class="node-type">{{ getProcessorInfo(node.type).label }}</span>
          </div>
          <div class="node-name">{{ node.name }}</div>

          <!-- Output ports -->
          <template v-if="node.type === 'if'">
            <div class="node-port output-port-true" @mousedown.stop="startConnect(node.id, 0)">
              <span class="port-label">T</span>
            </div>
            <div class="node-port output-port-false" @mousedown.stop="startConnect(node.id, 1)">
              <span class="port-label">F</span>
            </div>
          </template>
          <template v-else>
            <div class="node-port output-port" @mousedown.stop="startConnect(node.id, 0)" />
          </template>

          <NButton
            class="node-delete"
            size="tiny"
            quaternary
            type="error"
            @click.stop="removeNode(node.id)"
          >x</NButton>
        </div>
      </div>

      <!-- Node config panel -->
      <NDrawer v-model:show="showNodeConfig" width="360" placement="right">
        <NDrawerContent :title="selectedNode ? `${t('pipeline.configureNode', { name: selectedNode.name })}` : t('pipeline.nodeConfig')">
          <template v-if="selectedNode">
            <NForm label-placement="top" size="small">
              <NFormItem :label="t('pipeline.nodeName')">
                <NInput v-model:value="selectedNode.name" />
              </NFormItem>
              <NFormItem :label="t('pipeline.nodeType')">
                <NTag :style="{ color: getProcessorInfo(selectedNode.type).color }">
                  {{ getProcessorInfo(selectedNode.type).label }}
                </NTag>
              </NFormItem>
              <NFormItem :label="t('pipeline.nodeDisabled')">
                <NSwitch v-model:value="selectedNode.disabled" />
              </NFormItem>
              <NFormItem :label="t('pipeline.continueOnFail')">
                <NSwitch v-model:value="selectedNode.continue_on_fail" />
              </NFormItem>
              <NFormItem :label="t('pipeline.retryOnFail')">
                <NSwitch v-model:value="selectedNode.retry_on_fail" />
              </NFormItem>
              <NFormItem v-if="selectedNode.retry_on_fail" :label="t('pipeline.maxRetries')">
                <NInputNumber v-model:value="selectedNode.max_retries" :min="1" :max="5" />
              </NFormItem>

              <NDivider />

              <!-- Type-specific config -->
              <template v-if="selectedNode.type === 'if'">
                <NFormItem :label="t('pipeline.mode')">
                  <NSelect v-model:value="selectedNode.config.mode"
                    :options="[{ label: t('pipeline.modeTags'), value: 'tags' }, { label: t('pipeline.modeExpression'), value: 'expression' }]" />
                </NFormItem>
                <template v-if="selectedNode.config.mode === 'expression'">
                  <NFormItem :label="t('pipeline.field')">
                    <NSelect v-model:value="selectedNode.config.expression.field"
                      :options="['severity', 'status', 'source', 'alert_name'].map(v => ({ label: v, value: v }))" />
                  </NFormItem>
                  <NFormItem :label="t('pipeline.operator')">
                    <NSelect v-model:value="selectedNode.config.expression.op"
                      :options="['==', '!=', '=~', '!~'].map(v => ({ label: v, value: v }))" />
                  </NFormItem>
                  <NFormItem :label="t('pipeline.value')">
                    <NInput v-model:value="selectedNode.config.expression.value" />
                  </NFormItem>
                </template>
              </template>

              <template v-else-if="selectedNode.type === 'event_drop'">
                <NFormItem :label="t('pipeline.dropCondition')">
                  <div style="font-size: 12px; color: var(--sre-text-tertiary, #999); margin-bottom: 8px">
                    {{ t('pipeline.dropConditionHint') }}
                  </div>
                </NFormItem>
                <NFormItem :label="t('pipeline.field')">
                  <NSelect v-model:value="selectedNode.config.expression.field"
                    :options="['severity', 'status', 'source', 'alert_name'].map(v => ({ label: v, value: v }))" />
                </NFormItem>
                <NFormItem :label="t('pipeline.operator')">
                  <NSelect v-model:value="selectedNode.config.expression.op"
                    :options="['==', '!=', '=~', '!~'].map(v => ({ label: v, value: v }))" />
                </NFormItem>
                <NFormItem :label="t('pipeline.value')">
                  <NInput v-model:value="selectedNode.config.expression.value" />
                </NFormItem>
              </template>

              <template v-else-if="selectedNode.type === 'callback'">
                <NFormItem :label="t('pipeline.url')">
                  <NInput v-model:value="selectedNode.config.url" placeholder="https://example.com/webhook" />
                </NFormItem>
                <NFormItem :label="t('pipeline.method')">
                  <NSelect v-model:value="selectedNode.config.method"
                    :options="['POST', 'GET'].map(v => ({ label: v, value: v }))" />
                </NFormItem>
                <NFormItem :label="t('pipeline.timeout')">
                  <NInputNumber v-model:value="selectedNode.config.timeout" :min="1" :max="60" />
                </NFormItem>
              </template>

              <template v-else-if="selectedNode.type === 'ai_summary'">
                <NFormItem :label="t('pipeline.apiUrl')">
                  <NInput v-model:value="selectedNode.config.api_url" placeholder="https://api.openai.com/v1/chat/completions" />
                </NFormItem>
                <NFormItem :label="t('pipeline.apiKey')">
                  <NInput v-model:value="selectedNode.config.api_key" type="password" show-password-on="click" />
                </NFormItem>
                <NFormItem :label="t('pipeline.model')">
                  <NInput v-model:value="selectedNode.config.model" />
                </NFormItem>
                <NFormItem :label="t('pipeline.timeout')">
                  <NInputNumber v-model:value="selectedNode.config.timeout" :min="5" :max="120" />
                </NFormItem>
              </template>
            </NForm>
          </template>
        </NDrawerContent>
      </NDrawer>
    </div>
  </div>
</template>

<style scoped>
.pipeline-editor {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--sre-bg-page, #f5f5f5);
}
.editor-toolbar {
  padding: 8px 16px;
  background: var(--sre-bg-card, #fff);
  border-bottom: 1px solid var(--sre-border, #e0e0e0);
  display: flex;
  align-items: center;
}
.editor-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.node-palette {
  width: 190px;
  background: var(--sre-bg-card, #fff);
  border-right: 1px solid var(--sre-border, #e0e0e0);
  padding: 12px;
  overflow-y: auto;
}
.palette-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--sre-text-secondary, #666);
  text-transform: uppercase;
  margin-bottom: 8px;
}
.palette-item {
  padding: 8px 12px;
  margin-bottom: 4px;
  border-radius: 6px;
  border-left: 3px solid;
  background: var(--sre-bg-hover, #fafafa);
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s;
}
.palette-item:hover {
  background: var(--sre-bg-active, #f0f0f0);
}
.filter-row {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 4px;
}
.canvas-container {
  flex: 1;
  position: relative;
  overflow: auto;
  background: var(--sre-bg-canvas, #fafbfc);
}
.connections-svg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
.connection-path {
  pointer-events: stroke;
  cursor: pointer;
}
.connection-path:hover {
  stroke: #d03050;
  stroke-width: 3;
}
.pipeline-node {
  position: absolute;
  width: 180px;
  border: 2px solid #ddd;
  border-radius: 8px;
  background: var(--sre-bg-card, #fff);
  cursor: grab;
  user-select: none;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
}
.pipeline-node.selected {
  box-shadow: 0 0 0 2px #18a058;
}
.pipeline-node.disabled {
  opacity: 0.5;
}
.node-header {
  padding: 4px 8px;
  border-radius: 6px 6px 0 0;
  font-size: 11px;
  font-weight: 600;
}
.node-name {
  padding: 6px 8px;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.node-port {
  position: absolute;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #999;
  border: 2px solid #fff;
  cursor: crosshair;
  z-index: 10;
}
.input-port {
  left: -6px;
  top: 50%;
  transform: translateY(-50%);
}
.output-port {
  right: -6px;
  top: 50%;
  transform: translateY(-50%);
}
.output-port-true {
  right: -6px;
  top: 30%;
  background: #18a058;
}
.output-port-false {
  right: -6px;
  top: 70%;
  background: #d03050;
}
.port-label {
  position: absolute;
  right: 14px;
  top: -4px;
  font-size: 10px;
  font-weight: 600;
}
.node-delete {
  position: absolute;
  top: -8px;
  right: -8px;
}
</style>

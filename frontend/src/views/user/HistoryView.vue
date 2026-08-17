<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconApps, IconCheckCircleFill, IconClockCircle, IconCloseCircleFill, IconImage, IconLoading, IconRefresh, IconVideoCamera } from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../../services/api'
import MediaPreview from '../../components/MediaPreview.vue'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const rows = ref<any[]>([])
const stats = ref<any>({ total: 0, success: 0, failed: 0, pending: 0 })
const loading = ref(false)
const filter = ref('all')
const query = ref('')
const preview = ref<any>(null)
let pollTimer: number | undefined
let requestSerial = 0
const filtered = computed(() => rows.value.filter((row) =>
  (filter.value === 'all' || row.kind === filter.value) &&
  (!query.value || String(row.prompt || '').toLowerCase().includes(query.value.toLowerCase())),
))
const successRate = computed(() => Number(stats.value.total || 0) > 0 ? Math.round(Number(stats.value.success || 0) / Number(stats.value.total) * 100) : 0)

function fileUrl(row: any, thumbnail = false) {
  if (!row?.file) return ''
  const value = String(row.file).replace(/^\/+/, '')
  return imageUrl(`/images/${value}${thumbnail ? '.thumb.jpg' : ''}`)
}
function formatTime(value: any) {
  if (!value) return '--'
  const numeric = Number(value)
  const date = Number.isFinite(numeric) ? new Date(numeric < 10_000_000_000 ? numeric * 1000 : numeric) : new Date(value)
  return Number.isNaN(date.getTime()) ? '--' : date.toLocaleString('zh-CN', { hour12: false })
}
function isPending(row: any) {
  return ['pending', 'queued', 'running', 'processing'].includes(String(row?.status || '').toLowerCase())
}
function schedulePoll() {
  if (pollTimer) window.clearTimeout(pollTimer)
  pollTimer = undefined
  if (!rows.value.some(isPending)) return
  pollTimer = window.setTimeout(() => load(true), 3000)
}
async function load(silent = false) {
  const serial = ++requestSerial
  if (!silent) loading.value = true
  const hadPending = rows.value.some(isPending)
  try {
    const response = await api('/logs?limit=50')
    if (serial !== requestSerial) return
    if (response.ok) {
      rows.value = response.data?.data || []
      stats.value = response.data?.stats || stats.value
    }
    if (hadPending && !rows.value.some(isPending)) await auth.refreshUser()
  } finally {
    if (serial === requestSerial) {
      if (!silent) loading.value = false
      schedulePoll()
    }
  }
}
function openPreview(row: any) {
	if (row.status !== 'success' || !row.file) return Message.info('该记录还没有可预览的文件')
  preview.value = row
}
onMounted(() => load())
onUnmounted(() => { requestSerial += 1; if (pollTimer) window.clearTimeout(pollTimer) })
const columns = [
  { title: '作品', slotName: 'asset', width: 86 },
  { title: '任务', slotName: 'task', width: 330 },
  { title: '模型', slotName: 'model', width: 210 },
  { title: '状态', slotName: 'status', width: 120 },
  { title: '耗时', slotName: 'elapsed', width: 90 },
  { title: '额度', slotName: 'cost', width: 90 },
  { title: '创建时间', slotName: 'created', width: 180 },
]
</script>

<template>
  <div class="history-page">
    <div class="section-heading"><div><h2>生成记录</h2><p>查看生成任务、扣费状态以及失败后的自动退款。</p></div><a-button :loading="loading" @click="load()"><IconRefresh />刷新</a-button></div>
    <div class="history-stats" aria-label="我的生成统计">
      <div class="history-stat total"><span class="stat-icon"><IconApps /></span><div><small>累计任务</small><strong>{{ Number(stats.total || 0).toLocaleString() }}</strong><em>整体成功率 {{ successRate }}%</em></div></div>
      <div class="history-stat success"><span class="stat-icon"><IconCheckCircleFill /></span><div><small>生成成功</small><strong>{{ Number(stats.success || 0).toLocaleString() }}</strong><em>作品已完成并确认扣费</em></div></div>
      <div class="history-stat failed"><span class="stat-icon"><IconCloseCircleFill /></span><div><small>生成失败</small><strong>{{ Number(stats.failed || 0).toLocaleString() }}</strong><em>对应额度已自动退回</em></div></div>
      <div class="history-stat pending"><span class="stat-icon"><IconClockCircle /></span><div><small>正在处理</small><strong>{{ Number(stats.pending || 0).toLocaleString() }}</strong><em>状态将自动同步更新</em></div></div>
    </div>
    <div class="toolbar"><a-radio-group v-model="filter" type="button"><a-radio value="all">全部</a-radio><a-radio value="image">图像</a-radio><a-radio value="video">视频</a-radio></a-radio-group><a-input-search v-model="query" placeholder="搜索任务或提示词" style="width:260px" /></div>
    <div class="table-shell"><a-table :data="filtered" :columns="columns" :loading="loading" :pagination="{ pageSize: 10 }" :scroll="{ x: 1110 }" row-key="id">
      <template #asset="{ record }"><button class="asset-thumb" :class="record.status" :disabled="record.status !== 'success' || !record.file" @click="openPreview(record)"><img v-if="record.status === 'success' && record.file && !record._thumb_error" :src="fileUrl(record, true)" :alt="record.prompt" @error="record._thumb_error = true" /><span v-else-if="isPending(record)" class="pending-loader"><IconLoading /></span><IconVideoCamera v-else-if="record.kind === 'video'" /><IconImage v-else /></button></template>
      <template #task="{ record }"><div class="task-cell"><strong>{{ record.prompt || '未填写提示词' }}</strong><small>{{ record.id }}</small></div></template>
      <template #model="{ record }"><span class="model-cell" :title="record.model">{{ record.model || '--' }}</span></template>
      <template #status="{ record }"><a-tag :color="record.status === 'success' ? 'green' : record.status === 'failed' ? 'red' : 'orange'">{{ record.status === 'success' ? '已完成' : record.status === 'failed' ? '失败 · 已退款' : '处理中' }}</a-tag></template>
      <template #elapsed="{ record }">{{ record.elapsed_ms ? `${(record.elapsed_ms / 1000).toFixed(1)}s` : '--' }}</template>
      <template #cost="{ record }">{{ record.status === 'success' ? `-${record.cost || 0}` : '0' }}</template>
      <template #created="{ record }"><span class="time-cell">{{ formatTime(record.created_at || record.ts) }}</span></template>
    </a-table></div>
    <MediaPreview :visible="Boolean(preview)" :src="preview ? fileUrl(preview) : ''" :kind="preview?.kind" :filename="preview?.file?.split('/').pop()" downloadable @close="preview = null" />
  </div>
</template>

<style scoped>
.history-page{max-width:1260px}.history-stats{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin:0 0 18px}.history-stat{min-width:0;min-height:104px;padding:17px 18px;position:relative;overflow:hidden;display:flex;align-items:flex-start;gap:13px;border:1px solid #daddd6;border-radius:8px;background:#f1f2ee}.history-stat::after{content:'';width:56px;height:56px;position:absolute;right:-22px;bottom:-24px;border:12px solid currentColor;opacity:.045;transform:rotate(24deg)}.history-stat .stat-icon{width:34px;height:34px;flex:0 0 auto;display:grid;place-items:center;border-radius:7px;background:rgba(255,255,255,.72);box-shadow:0 1px 0 rgba(30,40,31,.05)}.history-stat .stat-icon :deep(svg){width:17px;height:17px}.history-stat>div{min-width:0;display:flex;flex-direction:column}.history-stat small{font-size:10px;font-weight:600;opacity:.76}.history-stat strong{margin:5px 0 7px;font-size:23px;line-height:1;letter-spacing:0}.history-stat em{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:9px;font-style:normal;opacity:.66}.history-stat.total{color:#455048;background:#e8ece7;border-color:#d0d7cf}.history-stat.success{color:#3f7049;background:#e5f1e5;border-color:#c9dfca}.history-stat.failed{color:#965a4d;background:#f7e9e4;border-color:#ebcec5}.history-stat.pending{color:#846a22;background:#f6efd9;border-color:#e6d8a8}.toolbar{display:flex;align-items:center;justify-content:space-between;margin-bottom:14px}.table-shell{border:1px solid var(--ns-line);border-radius:8px;overflow:hidden;background:#fff}.asset-thumb{width:58px;height:58px;padding:0;position:relative;display:grid;place-items:center;overflow:hidden;border:1px solid var(--ns-line);border-radius:6px;background:#edf0ea;color:#899087;cursor:pointer}.asset-thumb:disabled{cursor:default}.asset-thumb img{width:100%;height:100%;object-fit:cover}.pending-loader{width:32px;height:32px;display:grid;place-items:center;border:1px solid #cfd4ca;border-radius:50%;background:#f7f8f5;color:#5f7059}.pending-loader :deep(svg){width:16px;height:16px;animation:spin .9s linear infinite}.task-cell{min-width:0;display:flex;flex-direction:column}.task-cell strong{font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.task-cell small{font-size:9px;color:var(--ns-ink-faint);margin-top:4px}.model-cell{display:block;max-width:195px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;word-break:normal}.time-cell{white-space:nowrap;color:var(--ns-ink-soft);font-size:10px}.preview-media{display:block;max-width:100%;max-height:62vh;margin:auto;object-fit:contain}.preview-meta{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-top:14px;padding-top:13px;border-top:1px solid var(--ns-line)}.preview-meta>div{min-width:0;display:flex;flex-direction:column}.preview-meta strong{font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.preview-meta span{margin-top:5px;color:var(--ns-ink-faint);font-size:9px}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:900px){.history-stats{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:650px){.history-stats{grid-template-columns:1fr}.history-stat{min-height:88px}.toolbar{align-items:stretch;gap:10px;flex-direction:column}.toolbar .arco-input-wrapper{width:100%!important}.preview-meta{align-items:stretch;flex-direction:column}}@media(prefers-reduced-motion:reduce){.pending-loader :deep(svg){animation:none}}
</style>

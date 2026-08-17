<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCloseCircle, IconCopy, IconImage, IconLoading, IconRefresh, IconSave as IconDownload, IconSearch, IconVideoCamera } from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../../services/api'
import MediaPreview from '../../components/MediaPreview.vue'

const rows = ref<any[]>([])
const loading = ref(false)
const query = ref('')
const status = ref('all')
const kind = ref('all')
const provider = ref('all')
const total = ref(0)
const page = ref(1)
const pageSize = 20
const preview = ref<any>(null)
const stats = ref<any>({ total: 0, success: 0, failed: 0 })
const todayStats = ref<any>({ total: 0, success: 0, failed: 0 })
let pollTimer: number | undefined
let requestSerial = 0
const providers = computed(() => [...new Set(rows.value.map((x) => x.provider).filter(Boolean))])
const filtered = computed(() => rows.value.filter((x) =>
  (!query.value || `${x.id} ${x.user_name} ${x.prompt}`.toLowerCase().includes(query.value.toLowerCase())),
))
const pagination = computed(() => ({ current: page.value, pageSize, total: total.value, showTotal: true }))

function mediaUrl(row: any, thumbnail = false) {
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
function failureReason(value: any) {
  const reason = String(value || '').trim()
  if (!reason) return '上游未返回具体原因'
  if (/duplicated key not allowed|duplicate key/i.test(reason)) return '并发扣费事务冲突'
  if (/insufficient|余额不足|额度不足/i.test(reason)) return '可用额度不足'
  if (/timeout|deadline exceeded|超时/i.test(reason)) return '上游服务响应超时'
  if (/no available|没有可用|account.*unavailable/i.test(reason)) return '暂无可用账号'
  return reason
}
async function copyPrompt(row: any) {
  const prompt = String(row.prompt || '').trim()
  if (!prompt) return Message.info('该请求没有提示词')
  await navigator.clipboard.writeText(prompt)
  Message.success('提示词已复制')
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
  const params = new URLSearchParams({ scope: 'all', limit: String(pageSize), offset: String((page.value - 1) * pageSize) })
  if (kind.value !== 'all') params.set('kind', kind.value)
  if (status.value !== 'all') params.set('status', status.value)
  if (provider.value !== 'all') params.set('provider', provider.value)
  try {
    const [response, dashboard] = await Promise.all([
      api(`/logs?${params.toString()}`),
      api('/dashboard'),
    ])
    if (serial !== requestSerial) return
    if (response.ok) {
      rows.value = response.data?.data || []
      total.value = response.data?.total || rows.value.length
      stats.value = response.data?.stats || stats.value
    }
    if (dashboard.ok) todayStats.value = dashboard.data?.today || todayStats.value
  } finally {
    if (serial === requestSerial) {
      if (!silent) loading.value = false
      schedulePoll()
    }
  }
}
function changePage(value: number) {
  page.value = value
  load()
}
function openPreview(row: any) {
  if (row?.status !== 'success' || !row?.file) return Message.info('该记录暂时没有可预览的作品')
  preview.value = row
}
function exportCSV() {
  const fields = ['id', 'kind', 'user_name', 'model', 'provider', 'status', 'cost', 'elapsed_ms', 'created_at', 'prompt']
  const csv = [fields.join(','), ...filtered.value.map((row) => fields.map((field) => `"${String(row[field] ?? '').replaceAll('"', '""')}"`).join(','))].join('\n')
  const link = document.createElement('a')
  link.href = URL.createObjectURL(new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' }))
  link.download = `northstar-logs-${Date.now()}.csv`
  link.click()
  URL.revokeObjectURL(link.href)
}

onMounted(() => load())
onUnmounted(() => { requestSerial += 1; if (pollTimer) window.clearTimeout(pollTimer) })
watch([kind, status, provider], () => {
  page.value = 1
  load()
})
const columns = [
  { title: '作品', slotName: 'asset', width: 82 },
  { title: '类型', slotName: 'kind', width: 90 },
  { title: '请求', slotName: 'request', width: 280 },
  { title: '调用用户', slotName: 'user', width: 155 },
  { title: '模型', slotName: 'model', width: 190 },
  { title: 'Provider', dataIndex: 'provider', width: 110 },
  { title: '状态', slotName: 'status', width: 130 },
  { title: '耗时', slotName: 'elapsed', width: 85 },
  { title: '额度', dataIndex: 'cost', width: 75 },
  { title: '时间', slotName: 'time', width: 175 },
]
</script>

<template>
  <div class="logs-page">
    <div class="section-heading"><div><h2>生成日志</h2><p>追踪请求、Provider 调度、计费和退款结果。</p></div><a-space><a-button :loading="loading" @click="load()"><IconRefresh />刷新</a-button><a-button :disabled="!filtered.length" @click="exportCSV"><IconDownload />导出</a-button></a-space></div>
    <div class="stats-strip" aria-label="生成任务统计">
      <div class="stat-item total"><span>总任务数</span><strong>{{ Number(stats.total || 0).toLocaleString() }}</strong><small class="stat-split"><b class="success-text">成功 {{ Number(stats.success || 0).toLocaleString() }}</b><b class="failed-text">失败 {{ Number(stats.failed || 0).toLocaleString() }}</b></small></div>
      <div class="stat-item today"><span>今日任务数</span><strong>{{ Number(todayStats.total || 0).toLocaleString() }}</strong><small class="stat-split"><b class="success-text">成功 {{ Number(todayStats.success || 0).toLocaleString() }}</b><b class="failed-text">失败 {{ Number(todayStats.failed || 0).toLocaleString() }}</b></small></div>
      <div class="stat-item success"><span>成功任务</span><strong>{{ Number(stats.success || 0).toLocaleString() }}</strong><small>全量已完成</small></div>
      <div class="stat-item failed"><span>失败任务</span><strong>{{ Number(stats.failed || 0).toLocaleString() }}</strong><small>已自动退款</small></div>
    </div>
    <div class="toolbar"><a-input v-model="query" placeholder="搜索事件 ID、用户或提示词"><template #prefix><IconSearch /></template></a-input><a-select v-model="kind"><a-option value="all">全部类型</a-option><a-option value="image">图片</a-option><a-option value="video">视频</a-option></a-select><a-select v-model="status"><a-option value="all">全部状态</a-option><a-option value="success">成功</a-option><a-option value="failed">失败</a-option><a-option value="pending">进行中</a-option></a-select><a-select v-model="provider"><a-option value="all">全部 Provider</a-option><a-option v-for="item in providers" :key="item" :value="item">{{ item }}</a-option></a-select></div>
    <div class="table-shell"><a-table :columns="columns" :data="filtered" :loading="loading" :pagination="pagination" :scroll="{ x: 1320 }" row-key="id" @page-change="changePage">
      <template #asset="{ record }"><button class="asset-thumb" :class="record.status" :disabled="record.status !== 'success' || !record.file" :title="record.status === 'success' && record.file ? '预览作品' : record.status === 'failed' ? failureReason(record.error) : '正在生成'" @click="openPreview(record)"><img v-if="record.status === 'success' && record.file && !record._thumb_error" :src="mediaUrl(record, true)" :alt="record.prompt" @error="record._thumb_error = true" /><span v-else-if="isPending(record)" class="pending-loader"><IconLoading /></span><IconCloseCircle v-else-if="record.status === 'failed'" /><IconVideoCamera v-else-if="record.kind === 'video'" /><IconImage v-else /></button></template>
      <template #kind="{ record }"><span class="kind-cell" :class="record.kind"><IconVideoCamera v-if="record.kind === 'video'" /><IconImage v-else />{{ record.kind === 'video' ? '视频' : '图片' }}</span></template>
      <template #request="{ record }"><button class="request" title="点击复制提示词" @click="copyPrompt(record)"><strong>{{ record.prompt || '无提示词' }}</strong><small>{{ record.id }}<IconCopy /></small></button></template>
      <template #user="{ record }"><div class="user-cell" :title="`${record.user_name || '匿名用户'} · ${record.user_id || '未绑定用户'}`"><strong>{{ record.user_name || '匿名用户' }}</strong><small>{{ record.user_id || '未绑定用户' }}</small></div></template>
      <template #model="{ record }"><span class="model-cell" :title="record.model">{{ record.model || '--' }}</span></template>
      <template #status="{ record }"><div class="status-cell"><a-tag :color="record.status === 'success' ? 'green' : record.status === 'failed' ? 'red' : 'orange'">{{ record.status === 'success' ? '成功 · 已扣费' : record.status === 'failed' ? '失败 · 已退款' : '进行中' }}</a-tag><small v-if="record.status === 'failed'" :title="String(record.error || '')">{{ failureReason(record.error) }}</small></div></template>
      <template #elapsed="{ record }">{{ record.elapsed_ms ? `${(record.elapsed_ms / 1000).toFixed(1)}s` : '--' }}</template>
      <template #time="{ record }"><span class="time-cell">{{ formatTime(record.created_at || record.ts) }}</span></template>
    </a-table></div>
    <MediaPreview :visible="Boolean(preview)" :src="preview ? mediaUrl(preview) : ''" :kind="preview?.kind" :filename="preview?.file?.split('/').pop()" downloadable @close="preview = null" />
  </div>
</template>

<style scoped>
.logs-page{max-width:1320px}.stats-strip{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin:0 0 16px}.stat-item{min-width:0;padding:14px 16px;position:relative;overflow:hidden;border:1px solid var(--ns-line);border-radius:8px;background:#f0f1ed}.stat-item::after{content:'';width:54px;height:8px;position:absolute;right:-12px;top:12px;background:currentColor;opacity:.12;transform:rotate(-36deg)}.stat-item>span,.stat-item small{display:block;color:var(--ns-ink-faint);font-size:10px}.stat-item strong{display:block;margin:6px 0 6px;color:var(--ns-ink);font-size:23px;line-height:1}.stat-item small{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.stat-item.total{background:#e9ece8;border-color:#d2d8d1}.stat-item.today{background:#f5efda;border-color:#e7d9a8}.stat-item.success{color:#3e7149;background:#e5f1e5;border-color:#c6dec9}.stat-item.failed{color:#985b4d;background:#f7e8e3;border-color:#ebcbc2}.stat-item.success strong,.stat-item.failed strong{color:currentColor}.stat-split{display:flex!important;align-items:center;gap:12px}.stat-split b{font-size:9px;font-weight:650}.success-text{color:#4d7a54}.failed-text{color:#a35e50}.toolbar{display:grid;grid-template-columns:minmax(260px,1fr) 130px 150px 170px;gap:10px;margin-bottom:14px}.table-shell{border:1px solid var(--ns-line);border-radius:8px;overflow:hidden;background:#fff}.asset-thumb{width:54px;height:54px;padding:0;position:relative;display:grid;place-items:center;overflow:hidden;border:1px solid var(--ns-line);border-radius:6px;background:#edf0ea;color:#899087;cursor:pointer}.asset-thumb:disabled{cursor:default}.asset-thumb img{width:100%;height:100%;object-fit:cover}.asset-thumb.failed{background:#f3eeeb;color:#895f52}.pending-loader{width:32px;height:32px;display:grid;place-items:center;border:1px solid #cfd4ca;border-radius:50%;background:#f7f8f5;color:#5f7059}.pending-loader :deep(svg){width:16px;height:16px;animation:spin .9s linear infinite}.kind-cell{height:28px;padding:0 9px;display:inline-flex;align-items:center;gap:6px;border-radius:999px;background:#edf1ea;color:#5c6b56;font-size:10px;font-weight:600;white-space:nowrap}.kind-cell.video{background:#eeeae4;color:#6d6255}.kind-cell :deep(svg){width:13px;height:13px}.request{width:100%;padding:0;border:0;background:transparent;display:flex;flex-direction:column;min-width:0;text-align:left;cursor:copy}.request strong{font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.request small{display:flex;align-items:center;gap:7px;margin-top:4px;color:var(--ns-ink-faint);font:9px ui-monospace}.request small svg{width:12px;height:12px}.user-cell{min-width:0;display:flex;flex-direction:column;gap:4px}.user-cell strong,.user-cell small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.user-cell strong{font-size:11px;color:var(--ns-ink)}.user-cell small{font:9px ui-monospace;color:var(--ns-ink-faint)}.model-cell{display:block;max-width:175px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.status-cell{max-width:145px;display:flex;flex-direction:column;align-items:flex-start;gap:5px}.status-cell small{display:block;width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#8a6256;font-size:9px;line-height:1.35}.time-cell{white-space:nowrap;color:var(--ns-ink-soft);font-size:10px}.preview-media{display:block;max-width:100%;max-height:62vh;margin:auto;object-fit:contain}.preview-prompt{margin:12px 0 0;color:var(--ns-ink-soft);font-size:11px;line-height:1.6}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:900px){.toolbar{grid-template-columns:minmax(220px,1fr) repeat(3,120px)}}@media(max-width:760px){.stats-strip{grid-template-columns:repeat(2,minmax(0,1fr))}.toolbar{grid-template-columns:1fr 1fr}}@media(max-width:650px){.toolbar>.arco-select:last-child{display:none}}@media(prefers-reduced-motion:reduce){.pending-loader :deep(svg){animation:none}}
</style>

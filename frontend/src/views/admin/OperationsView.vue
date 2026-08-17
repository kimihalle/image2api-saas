<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCheckCircle, IconCloud, IconExclamationCircle, IconRefresh, IconSafe, IconSettings, IconThunderbolt } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'

const data = ref<any>({ metrics: {}, alerts: [], reconciliations: [], backups: [] })
const deadJobs = ref<any[]>([])
const loading = ref(false)
const reconciling = ref(false)
const backingUp = ref(false)
const issueOpen = ref(false)
const issues = ref<any[]>([])
const settingsOpen = ref(false)
const reliability = ref<any>({ alert_webhook_url: '', backup_enabled: true, backup_retention_days: 14 })
let stream: EventSource | null = null
let reloadTimer: number | undefined

const m = computed(() => data.value.metrics || {})
const queueTotal = computed(() => Number(m.value.queue_queued || 0) + Number(m.value.queue_processing || 0) + Number(m.value.queue_retrying || 0))
const failureRate = computed(() => {
  const total = Number(m.value.success_15m || 0) + Number(m.value.failed_15m || 0)
  return total ? `${((Number(m.value.failed_15m || 0) / total) * 100).toFixed(1)}%` : '0%'
})

function time(value: any) {
  if (!value) return '--'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
function size(value: any) {
  const bytes = Number(value || 0)
  if (!bytes) return '--'
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`
  return `${(bytes / 1024 ** 2).toFixed(2)} MB`
}
function statusText(value: string) {
  return ({ completed: '已完成', failed: '失败', running: '执行中', open: '待处理', resolved: '已解决', repaired: '已修复', unresolved: '待核对', dead_letter: '重试耗尽' } as any)[value] || value || '--'
}
function statusClass(value: string) {
  if (['completed', 'resolved', 'repaired'].includes(value)) return 'ok'
  if (['failed', 'open', 'unresolved', 'dead_letter'].includes(value)) return 'danger'
  return 'pending'
}

async function load(silent = false) {
  if (!silent) loading.value = true
  const [snapshot, dead, settings] = await Promise.all([api('/operations'), api('/generation/dead-letter?limit=50'), api('/operations/settings')])
  if (snapshot.ok) data.value = snapshot.data || data.value
  else if (!silent) Message.error(snapshot.data?.detail || '系统保障数据加载失败')
  if (dead.ok) deadJobs.value = dead.data?.data || []
  if (settings.ok) reliability.value = settings.data || reliability.value
  loading.value = false
}
async function saveSettings() {
  const response = await api('/operations/settings', { method: 'PUT', body: JSON.stringify(reliability.value) })
  if (!response.ok) return Message.error(response.data?.detail || '保障配置保存失败')
  settingsOpen.value = false
  Message.success('保障配置已保存')
}
function scheduleReload() {
  if (reloadTimer) window.clearTimeout(reloadTimer)
  reloadTimer = window.setTimeout(() => load(true), 350)
}
async function resolveAlert(id: string) {
  const response = await api(`/operations/alerts/${id}/resolve`, { method: 'POST' })
  if (!response.ok) return Message.error(response.data?.detail || '告警处理失败')
  Message.success('告警已标记解决')
  await load(true)
}
async function reconcile() {
  reconciling.value = true
  const response = await api('/operations/reconcile', { method: 'POST' })
  reconciling.value = false
  if (!response.ok) return Message.error(response.data?.detail || '自动对账失败')
  Message.success(`对账完成，自动修复 ${response.data?.data?.auto_repaired || 0} 条`)
  await load(true)
}
async function backup() {
  backingUp.value = true
  const response = await api('/operations/backup', { method: 'POST' })
  backingUp.value = false
  if (!response.ok) return Message.error(response.data?.detail || '备份失败')
  Message.success('数据库备份已完成')
  await load(true)
}
async function showIssues(run: any) {
  const response = await api(`/operations/reconciliations/${run.id}/issues`)
  if (!response.ok) return Message.error(response.data?.detail || '差异读取失败')
  issues.value = response.data?.data || []
  issueOpen.value = true
}
async function retryDead(job: any) {
  const response = await api(`/generation/dead-letter/${job.id}/retry`, { method: 'POST' })
  if (!response.ok) return Message.error(response.data?.detail || '任务重试失败')
  Message.success('任务已重新进入队列')
  await load(true)
}

onMounted(() => {
  load()
  stream = new EventSource('/admin/api/generation/events')
  stream.addEventListener('generation', scheduleReload)
  stream.onerror = () => { /* EventSource reconnects; manual refresh remains available. */ }
})
onUnmounted(() => {
  stream?.close()
  if (reloadTimer) window.clearTimeout(reloadTimer)
})
</script>

<template>
  <div class="operations-page">
    <header class="page-head">
      <div><span>RELIABILITY CENTER</span><h2>系统保障</h2><p>队列、账号池、财务一致性与数据备份的统一运行视图。</p></div>
      <div class="head-actions"><button class="round-action" aria-label="保障配置" @click="settingsOpen = true"><IconSettings /></button><button class="round-action" aria-label="刷新" @click="load()"><IconRefresh /></button></div>
    </header>

    <a-spin :loading="loading" class="page-spin">
      <section class="metric-strip">
        <div><span>运行队列</span><strong>{{ queueTotal }}</strong><small>排队 {{ m.queue_queued || 0 }} · 执行 {{ m.queue_processing || 0 }} · 重试 {{ m.queue_retrying || 0 }}</small></div>
        <div><span>账号池健康</span><strong>{{ m.accounts_active || 0 }}<i>/ {{ m.accounts_total || 0 }}</i></strong><small>冷却 {{ m.accounts_cooling || 0 }} · 停用 {{ m.accounts_dead || 0 }}</small></div>
        <div><span>近 15 分钟失败率</span><strong :class="{ warn: Number(m.failed_15m || 0) > 0 }">{{ failureRate }}</strong><small>成功 {{ m.success_15m || 0 }} · 失败 {{ m.failed_15m || 0 }}</small></div>
        <div><span>待处理风险</span><strong :class="{ warn: Number(m.open_alerts || 0) > 0 }">{{ m.open_alerts || 0 }}</strong><small>死信 {{ m.queue_dead || 0 }} · 预扣 {{ m.reserved_credits || 0 }}</small></div>
      </section>

      <section class="section-block">
        <div class="section-head"><div><IconExclamationCircle /><span><strong>实时告警</strong><small>系统每分钟自动评估，恢复后自动关闭</small></span></div></div>
        <div v-if="data.alerts?.length" class="alert-list">
          <article v-for="item in data.alerts" :key="item.id" :class="['alert-row', item.severity]">
            <span class="severity-dot"></span><div><strong>{{ item.title }}</strong><p>{{ item.message }}</p></div>
            <span class="meta">{{ item.category }} · {{ time(item.last_seen_at) }}</span>
            <span :class="['status', statusClass(item.status)]">{{ statusText(item.status) }}</span>
            <button v-if="item.status === 'open'" class="text-action" @click="resolveAlert(item.id)"><IconCheckCircle />确认解决</button>
          </article>
        </div>
        <div v-else class="empty-line"><IconSafe />当前没有告警，核心链路运行正常</div>
      </section>

      <div class="split-layout">
        <section class="section-block">
          <div class="section-head"><div><IconThunderbolt /><span><strong>自动对账</strong><small>核对生成结果、预扣、结算与退款</small></span></div><a-button type="primary" shape="round" :loading="reconciling" @click="reconcile">立即对账</a-button></div>
          <div class="table-wrap"><table><thead><tr><th>执行时间</th><th>检查</th><th>差异</th><th>自动修复</th><th>待核对</th><th>状态</th></tr></thead><tbody>
            <tr v-for="item in data.reconciliations" :key="item.id" @click="showIssues(item)"><td>{{ time(item.started_at) }}</td><td>{{ item.checked }}</td><td>{{ item.issues }}</td><td class="success-text">{{ item.auto_repaired }}</td><td class="danger-text">{{ item.unresolved }}</td><td><span :class="['status', statusClass(item.status)]">{{ statusText(item.status) }}</span></td></tr>
            <tr v-if="!data.reconciliations?.length"><td colspan="6" class="empty-cell">暂无对账记录</td></tr>
          </tbody></table></div>
        </section>

        <section class="section-block">
          <div class="section-head"><div><IconCloud /><span><strong>备份恢复</strong><small>PostgreSQL 完整备份与 SHA-256 校验</small></span></div><a-button type="primary" shape="round" :loading="backingUp" @click="backup">立即备份</a-button></div>
          <div class="table-wrap"><table><thead><tr><th>执行时间</th><th>体积</th><th>存储位置</th><th>状态</th></tr></thead><tbody>
            <tr v-for="item in data.backups" :key="item.id"><td>{{ time(item.started_at) }}</td><td>{{ size(item.size_bytes) }}</td><td class="key-cell">{{ item.storage_key || item.error || '--' }}</td><td><span :class="['status', statusClass(item.status)]">{{ statusText(item.status) }}</span></td></tr>
            <tr v-if="!data.backups?.length"><td colspan="4" class="empty-cell">暂无备份记录</td></tr>
          </tbody></table></div>
        </section>
      </div>

      <section class="section-block">
        <div class="section-head"><div><IconExclamationCircle /><span><strong>死信任务</strong><small>仅显示超过最大重试次数的任务，人工确认后可重新入队</small></span></div><span class="count-chip">{{ deadJobs.length }}</span></div>
        <div class="table-wrap"><table><thead><tr><th>任务</th><th>类型 / 模型</th><th>尝试次数</th><th>最后错误</th><th>时间</th><th>操作</th></tr></thead><tbody>
          <tr v-for="item in deadJobs" :key="item.id"><td class="mono">{{ item.id }}</td><td>{{ item.kind }} · {{ item.model }}</td><td>{{ item.attempts }} / {{ item.max_attempts }}</td><td class="error-cell">{{ item.last_error || item.error || '--' }}</td><td>{{ time(item.created_at) }}</td><td><button class="text-action" @click="retryDead(item)"><IconRefresh />重新入队</button></td></tr>
          <tr v-if="!deadJobs.length"><td colspan="6" class="empty-cell">没有死信任务</td></tr>
        </tbody></table></div>
      </section>
    </a-spin>

    <a-modal v-model:visible="issueOpen" title="对账差异明细" :width="920" :footer="false">
      <div class="table-wrap modal-table"><table><thead><tr><th>类型</th><th>用户 / 引用</th><th>预期</th><th>实际</th><th>处理结果</th></tr></thead><tbody>
        <tr v-for="item in issues" :key="item.id"><td><strong>{{ item.kind }}</strong><small class="detail">{{ item.detail }}</small></td><td>{{ item.user_id || '--' }}<small class="detail mono">{{ item.reference }}</small></td><td>{{ item.expected }}</td><td>{{ item.actual }}</td><td><span :class="['status', statusClass(item.status)]">{{ statusText(item.status) }}</span></td></tr>
        <tr v-if="!issues.length"><td colspan="5" class="empty-cell">该批次没有差异</td></tr>
      </tbody></table></div>
    </a-modal>
    <a-modal v-model:visible="settingsOpen" title="系统保障配置" :width="620" @ok="saveSettings">
      <div class="settings-form"><label><span>告警 Webhook</span><a-input v-model="reliability.alert_webhook_url" placeholder="https://example.com/ops/alerts" /><small>关键告警会推送到该地址，同类告警 30 分钟内去重。</small></label><label class="inline-setting"><span><strong>每日自动备份</strong><small>每天 03:00 执行 PostgreSQL 完整备份</small></span><a-switch v-model="reliability.backup_enabled" size="small" /></label><label><span>备份保留天数</span><a-input-number v-model="reliability.backup_retention_days" :min="1" :max="365" /><small>仅自动清理对象存储中的过期备份。</small></label></div>
    </a-modal>
  </div>
</template>

<style scoped>
.operations-page{max-width:1420px;margin:0 auto}.page-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:22px}.page-head span{color:#8a7628;font-size:9px;font-weight:750}.page-head h2{margin:6px 0 5px;font-size:25px}.page-head p{margin:0;color:var(--ns-ink-soft);font-size:11px}.head-actions{display:flex;gap:8px}.round-action{width:38px;height:38px;display:grid;place-items:center;border:1px solid var(--ns-line);border-radius:50%;background:#fff;color:var(--ns-ink-soft);cursor:pointer}.page-spin{display:block}.metric-strip{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));border:1px solid var(--ns-line);border-radius:8px;background:#fff;overflow:hidden}.metric-strip>div{min-width:0;padding:20px 22px;border-right:1px solid var(--ns-line)}.metric-strip>div:last-child{border:0}.metric-strip span,.metric-strip small{display:block;color:var(--ns-ink-faint);font-size:9px}.metric-strip strong{display:block;margin:7px 0 6px;font-size:25px}.metric-strip strong i{font-size:12px;font-style:normal;color:var(--ns-ink-faint)}.metric-strip strong.warn,.danger-text{color:#b04c42}.success-text{color:#617a50}.section-block{margin-top:18px;border:1px solid var(--ns-line);border-radius:8px;background:#fff;overflow:hidden}.section-head{min-height:66px;padding:0 18px;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid var(--ns-line)}.section-head>div{display:flex;align-items:center;gap:10px}.section-head>div>svg{color:#78866f}.section-head span{display:flex;flex-direction:column}.section-head strong{font-size:12px}.section-head small{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.alert-list{display:flex;flex-direction:column}.alert-row{min-height:66px;display:grid;grid-template-columns:8px minmax(240px,1fr) auto auto auto;align-items:center;gap:14px;padding:10px 18px;border-bottom:1px solid #edf0ea}.alert-row:last-child{border:0}.severity-dot{width:7px;height:7px;border-radius:50%;background:#c89b35}.alert-row.critical .severity-dot{background:#b64b42}.alert-row strong{font-size:11px}.alert-row p{margin:4px 0 0;color:var(--ns-ink-soft);font-size:10px}.meta{color:var(--ns-ink-faint);font-size:9px}.status{display:inline-flex!important;width:max-content;padding:4px 8px;border-radius:999px;font-size:9px!important;font-weight:650}.status.ok{background:#e8efe4;color:#557046}.status.danger{background:#f7e7e4;color:#a1453d}.status.pending{background:#f4edda;color:#927124}.text-action{display:inline-flex;align-items:center;gap:5px;border:0;background:transparent;color:#65765c;font-size:10px;font-weight:650;cursor:pointer;white-space:nowrap}.empty-line{height:74px;display:flex;align-items:center;justify-content:center;gap:8px;color:var(--ns-ink-faint);font-size:10px}.split-layout{display:grid;grid-template-columns:1fr 1fr;gap:18px}.table-wrap{width:100%;overflow:auto}.table-wrap table{width:100%;border-collapse:collapse;font-size:10px}.table-wrap th{height:38px;padding:0 14px;background:#f6f7f3;color:var(--ns-ink-faint);font-size:9px;text-align:left;white-space:nowrap}.table-wrap td{height:48px;padding:8px 14px;border-top:1px solid #edf0ea;color:var(--ns-ink-soft)}.table-wrap tbody tr{cursor:default}.split-layout .table-wrap tbody tr{cursor:pointer}.key-cell,.error-cell{max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.mono{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:9px}.count-chip{min-width:28px;height:24px;display:grid!important;place-items:center;border-radius:999px;background:#edf0e9;color:#65705f;font-size:9px!important}.empty-cell{text-align:center!important;color:var(--ns-ink-faint)!important}.modal-table{max-height:560px}.detail{display:block;margin-top:4px;color:var(--ns-ink-faint);font-size:8px}.settings-form{display:flex;flex-direction:column;gap:18px}.settings-form label{display:flex;flex-direction:column;gap:7px}.settings-form label>span{font-size:10px;font-weight:650}.settings-form label>small,.settings-form .inline-setting span small{color:var(--ns-ink-faint);font-size:9px;font-weight:400}.settings-form .inline-setting{flex-direction:row;align-items:center;justify-content:space-between}.inline-setting>span{display:flex;flex-direction:column;gap:4px}
@media(max-width:1100px){.metric-strip{grid-template-columns:1fr 1fr}.metric-strip>div:nth-child(2){border-right:0}.metric-strip>div:nth-child(-n+2){border-bottom:1px solid var(--ns-line)}.split-layout{grid-template-columns:1fr}}@media(max-width:700px){.metric-strip{grid-template-columns:1fr}.metric-strip>div{border-right:0;border-bottom:1px solid var(--ns-line)!important}.alert-row{grid-template-columns:8px 1fr}.alert-row .meta,.alert-row .status,.alert-row .text-action{grid-column:2}.page-head p{display:none}}
</style>

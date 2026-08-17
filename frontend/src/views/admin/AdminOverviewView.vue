<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { IconCheckCircle, IconClockCircle, IconExclamationCircle, IconRefresh } from '@arco-design/web-vue/es/icon'
import TrendChart from '../../components/TrendChart.vue'
import { api } from '../../services/api'

const router = useRouter()
const data = ref<any>({})
const logs = ref<any[]>([])
const accounts = ref<any[]>([])
const loading = ref(false)
const chartHours = ref(24)

async function load() {
  loading.value = true
  const [dashboard, logResponse, accountResponse] = await Promise.all([
    api('/dashboard'),
    api('/logs?scope=all&limit=20'),
    api('/accounts'),
  ])
  if (dashboard.ok) data.value = dashboard.data || {}
  if (logResponse.ok) logs.value = logResponse.data?.data || []
  if (accountResponse.ok) accounts.value = accountResponse.data?.data || []
  loading.value = false
}

const lifetime = computed(() => data.value.lifetime || {})
const today = computed(() => data.value.today || {})
const successRate = computed(() => {
  const total = Number(lifetime.value.total || 0)
  return total ? `${(Number(lifetime.value.success || 0) / total * 100).toFixed(1)}%` : '0%'
})
const metrics = computed(() => [
  { label: '今日生成任务', value: Number(today.value.total || 0).toLocaleString(), detail: `图像 ${Number(today.value.image || 0)} · 视频 ${Number(today.value.video || 0)}` },
  { label: '今日活跃用户', value: Number(data.value.today_dau || 0).toLocaleString(), detail: `近 24 小时 ${Number(data.value.dau || 0)} 人` },
  { label: '累计生成任务', value: Number(lifetime.value.total || 0).toLocaleString(), detail: `API 请求 ${Number(lifetime.value.api || 0).toLocaleString()}` },
  { label: '全站成功率', value: successRate.value, detail: `失败 ${Number(lifetime.value.failed || 0).toLocaleString()} 次` },
])
const providerGroups = computed(() => {
  const map = new Map<string, any[]>()
  accounts.value.forEach((item) => map.set(item.pool || 'unknown', [...(map.get(item.pool || 'unknown') || []), item]))
  return [...map.entries()].map(([name, items]) => ({ name, total: items.length, active: items.filter((item) => item.status === 'active').length }))
})
const activeProviders = computed(() => accounts.value.filter((item) => item.status === 'active').length)
const failures = computed(() => logs.value.filter((item) => item.status === 'failed').length)
const unhealthy = computed(() => accounts.value.length - activeProviders.value)
const dateLabel = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', weekday: 'short' }).format(new Date())

function statusText(status: string) {
  if (status === 'success') return '成功 · 已扣费'
  if (status === 'failed') return '失败 · 已退款'
  return '处理中'
}

function shortTime(value: string) {
  if (!value) return '暂无时间'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

onMounted(load)
</script>

<template>
  <div class="admin-overview">
    <div class="admin-intro">
      <div><span>OPERATIONS OVERVIEW</span><h2>运营概览</h2><p>聚合生成质量、用户活跃、计费结果与上游资源状态。</p></div>
      <div class="intro-actions"><span>{{ dateLabel }}</span><a-button :loading="loading" @click="load"><IconRefresh />刷新数据</a-button></div>
    </div>

    <section class="metrics" aria-label="核心运营指标">
      <article v-for="metric in metrics" :key="metric.label">
        <span>{{ metric.label }}</span><strong>{{ metric.value }}</strong><small>{{ metric.detail }}</small>
      </article>
    </section>

    <section class="chart-panel">
      <div class="panel-head">
        <div><h3>生成任务趋势</h3><p>按小时统计图像与视频任务，数据来自真实生成日志。</p></div>
        <div class="range-control" aria-label="趋势时间范围">
          <button v-for="option in [{ value: 24, label: '24 小时' }, { value: 12, label: '12 小时' }, { value: 6, label: '6 小时' }]" :key="option.value" :class="{ active: chartHours === option.value }" @click="chartHours = option.value">{{ option.label }}</button>
        </div>
      </div>
      <TrendChart :data="data.hourly || []" :hours="chartHours" />
    </section>

    <div class="operations-grid">
      <section class="recent-panel">
        <div class="panel-head"><div><h3>最近生成记录</h3><p>请求执行、扣费和失败退款结果。</p></div><button class="plain-command" @click="router.push('/admin/logs')">查看全部日志</button></div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>请求</th><th>模型与 Provider</th><th>计费状态</th><th>耗时</th><th>额度</th><th>时间</th></tr></thead>
            <tbody>
              <tr v-for="row in logs.slice(0, 7)" :key="row.id">
                <td><div class="request-cell"><strong>{{ row.prompt || '无提示词' }}</strong><small>{{ row.user_name || row.user_id || '匿名用户' }} · {{ row.id }}</small></div></td>
                <td><div class="request-cell"><strong>{{ row.model || '未记录模型' }}</strong><small>{{ row.provider || '未记录 Provider' }}</small></div></td>
                <td><span class="status" :class="row.status"><IconCheckCircle v-if="row.status === 'success'" /><IconExclamationCircle v-else-if="row.status === 'failed'" /><IconClockCircle v-else />{{ statusText(row.status) }}</span></td>
                <td>{{ row.elapsed_ms ? `${(Number(row.elapsed_ms) / 1000).toFixed(1)}s` : '暂无' }}</td>
                <td>{{ Number(row.cost || 0).toLocaleString() }}</td>
                <td>{{ shortTime(row.created_at || row.ts) }}</td>
              </tr>
            </tbody>
          </table>
          <div v-if="!logs.length && !loading" class="empty">暂无生成记录</div>
        </div>
      </section>

      <aside class="health-panel">
        <div class="panel-head"><div><h3>Provider 健康</h3><p>当前账号池的实时调度状态。</p></div></div>
        <div class="health-summary"><strong>{{ activeProviders }} / {{ accounts.length }}</strong><span>账号可调度</span></div>
        <div class="provider-list">
          <button v-for="group in providerGroups" :key="group.name" @click="router.push('/admin/providers')"><span><i :class="group.active === group.total ? 'ok' : group.active ? 'warn' : 'bad'"></i>{{ group.name }}</span><strong>{{ group.active }} / {{ group.total }}</strong></button>
          <div v-if="!providerGroups.length && !loading" class="empty">暂无 Provider 账号</div>
        </div>
        <div v-if="unhealthy || failures" class="attention"><IconExclamationCircle /><div><strong>需要关注</strong><span v-if="unhealthy">{{ unhealthy }} 个账号当前不可调度</span><span v-if="failures">最近记录中有 {{ failures }} 次失败退款</span></div></div>
        <button class="manage-command" @click="router.push('/admin/providers')">管理 Provider 账号</button>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.admin-overview{display:flex;flex-direction:column;gap:20px}.admin-intro{display:flex;align-items:flex-end;justify-content:space-between;gap:24px}.admin-intro>div:first-child>span{font-size:9px;letter-spacing:.13em;color:#8a7628;font-weight:750}.admin-intro h2{font-size:27px;margin:6px 0 5px}.admin-intro p{margin:0;color:var(--ns-ink-soft);font-size:12px}.intro-actions{display:flex;align-items:center;gap:12px}.intro-actions>span{font:11px ui-monospace;color:var(--ns-ink-faint)}.metrics{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}.metrics article{min-height:132px;padding:18px 19px;background:linear-gradient(180deg,#fff,#fafaf8);border:1px solid var(--ns-line);border-radius:8px;box-shadow:0 1px 2px rgba(31,36,33,.04);display:flex;flex-direction:column}.metrics span{font-size:11px;color:var(--ns-ink-soft)}.metrics strong{margin:13px 0 16px;font-size:28px;line-height:1;font-variant-numeric:tabular-nums}.metrics small{margin-top:auto;color:var(--ns-ink-faint);font-size:10px}.chart-panel,.recent-panel,.health-panel{background:#fff;border:1px solid var(--ns-line);border-radius:8px;box-shadow:var(--ns-shadow)}.chart-panel{padding:20px 20px 14px}.panel-head{display:flex;align-items:flex-start;justify-content:space-between;gap:18px}.panel-head h3{margin:0 0 5px;font-size:14px}.panel-head p{margin:0;color:var(--ns-ink-faint);font-size:10px;line-height:1.5}.range-control{display:flex;padding:3px;border:1px solid var(--ns-line);border-radius:7px;background:#fafaf8}.range-control button{height:30px;padding:0 12px;border:0;border-radius:5px;background:transparent;color:var(--ns-ink-soft);font-size:10px;cursor:pointer}.range-control button.active{background:#202521;color:#fff;box-shadow:0 1px 3px rgba(31,36,33,.16)}.operations-grid{display:grid;grid-template-columns:minmax(0,1.8fr) minmax(260px,.62fr);gap:16px;align-items:start}.recent-panel{overflow:hidden}.recent-panel>.panel-head{padding:19px 20px 16px}.plain-command{border:0;background:transparent;color:var(--ns-accent-strong);font-size:10px;cursor:pointer}.table-wrap{overflow-x:auto}table{width:100%;border-collapse:collapse;font-size:10px}th{padding:10px 14px;border-top:1px solid var(--ns-line);border-bottom:1px solid var(--ns-line);background:#fafaf8;color:var(--ns-ink-faint);font-weight:600;text-align:left;white-space:nowrap}td{padding:12px 14px;border-bottom:1px solid #ededE8;color:var(--ns-ink-soft);white-space:nowrap}tbody tr:last-child td{border-bottom:0}tbody tr:hover{background:#fbfbf9}.request-cell{max-width:230px;display:flex;flex-direction:column}.request-cell strong{color:var(--ns-ink);font-size:10px;overflow:hidden;text-overflow:ellipsis}.request-cell small{margin-top:4px;color:var(--ns-ink-faint);font:9px ui-monospace;overflow:hidden;text-overflow:ellipsis}.status{display:inline-flex;align-items:center;gap:5px;font-size:9px}.status :deep(svg){width:13px;height:13px}.status.success{color:#587052}.status.failed{color:#a54b3f}.status.pending{color:#9a732a}.health-panel{padding:19px}.health-summary{margin:20px 0 12px;padding-bottom:17px;border-bottom:1px solid var(--ns-line);display:flex;align-items:baseline;gap:9px}.health-summary strong{font-size:25px}.health-summary span{color:var(--ns-ink-faint);font-size:10px}.provider-list{display:flex;flex-direction:column}.provider-list button{height:42px;padding:0;border:0;border-bottom:1px solid #ededE8;background:transparent;display:flex;align-items:center;justify-content:space-between;cursor:pointer}.provider-list button span{display:flex;align-items:center;gap:8px;font-size:10px}.provider-list button strong{font-size:10px}.provider-list i{width:7px;height:7px;border-radius:50%}.ok{background:#65735d}.warn{background:#b17a35}.bad{background:#a54b3f}.attention{margin-top:15px;padding:12px;background:#fbf7ef;border:1px solid #e8dcc8;border-radius:7px;color:#8d6328;display:flex;gap:9px}.attention>div{display:flex;flex-direction:column;gap:3px}.attention strong{font-size:10px}.attention span{font-size:9px}.manage-command{width:100%;height:36px;margin-top:15px;border:1px solid var(--ns-line);border-radius:6px;background:#fff;color:var(--ns-ink);font-size:10px;cursor:pointer}.manage-command:hover{background:#f4f5f1}.empty{padding:26px 12px;text-align:center;color:var(--ns-ink-faint);font-size:10px}
@media(max-width:1100px){.metrics{grid-template-columns:repeat(2,1fr)}.operations-grid{grid-template-columns:1fr}.health-panel{display:grid;grid-template-columns:1fr 1fr;gap:0 24px}.health-panel>.panel-head{grid-column:1/-1}.manage-command{align-self:end}}@media(max-width:680px){.admin-intro{align-items:flex-start;flex-direction:column}.intro-actions{width:100%;justify-content:space-between}.metrics{grid-template-columns:1fr 1fr;gap:8px}.metrics article{min-height:115px;padding:15px}.metrics strong{font-size:23px}.chart-panel{padding:17px 14px 8px}.panel-head{align-items:flex-start;flex-direction:column}.range-control{width:100%}.range-control button{flex:1;padding:0 6px}.operations-grid{min-width:0}.health-panel{display:block}.recent-panel>.panel-head{padding:16px}.table-wrap{max-width:calc(100vw - 34px)}}
</style>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconDelete, IconPlayCircle, IconPlus, IconRefresh } from '@arco-design/web-vue/es/icon'
import GenerationTestModal from '../../components/GenerationTestModal.vue'
import { api } from '../../services/api'

const rows = ref<any[]>([])
const active = ref('all')
const loading = ref(false)
const importOpen = ref(false)
const saving = ref(false)
const selectedKeys = ref<string[]>([])
const testingAccount = ref<any>(null)
const stats = ref<any>({ total: 0, dead_total: 0 })
const importResult = ref('')
const page = ref(1)
const pageSize = 20
const form = reactive({ provider: 'chatgpt', name: '', value: '', weight: 0 })
const endpoints: Record<string, string> = { chatgpt: 'import-chatgpt-token', adobe: 'import-adobe-cookie', runway: 'import-runway-token', leonardo: 'import-leonardo-cookie', krea: 'import-krea-cookie', imagine: 'import-imagine-token', grok: 'import-grok-token' }
let pollTimer: number | undefined
let pollAttempts = 0

const filtered = computed(() => active.value === 'all' ? rows.value : rows.value.filter((item) => active.value === 'dead' ? (item.dead || item.status === 'dead' || item.status === 'disabled') : item.status === active.value))
const deadCount = computed(() => Number(stats.value.dead_total || rows.value.filter((item) => item.dead || item.status === 'dead').length))
const pagination = computed(() => ({ current: page.value, pageSize, total: filtered.value.length, showTotal: true }))

async function load() {
  loading.value = true
  const response = await api('/accounts?limit=0')
  rows.value = response.ok ? (response.data?.data || []) : []
  stats.value = response.data?.stats || { total: rows.value.length, dead_total: 0 }
  selectedKeys.value = selectedKeys.value.filter((id) => rows.value.some((item) => item.id === id))
  page.value = Math.min(page.value, Math.max(1, Math.ceil(filtered.value.length / pageSize)))
  loading.value = false
  schedulePendingPoll()
}

function schedulePendingPoll() {
  if (pollTimer) window.clearTimeout(pollTimer)
  if (!rows.value.some((item) => item.pending || item.status === 'pending') || pollAttempts >= 60) return
  pollTimer = window.setTimeout(async () => { pollAttempts += 1; await load() }, 2000)
}

async function syncOne(row: any, quiet = false) {
  const response = await api(`/accounts/${row.pool}/${row.id}/quota`)
  if (!response.ok) { if (!quiet) Message.error(response.data?.detail || '同步失败'); return }
  Object.assign(row, response.data?.data || response.data)
  if (!quiet) { await load(); Message.success('账号资料与额度已同步') }
}

async function syncAll() {
  loading.value = true
  const queue = [...rows.value]
  const workers = Array.from({ length: Math.min(5, queue.length) }, async () => {
    while (queue.length) {
      const row = queue.shift()
      if (row) await syncOne(row, true)
    }
  })
  await Promise.all(workers)
  await load()
  Message.success('账号资料与额度已同步')
}

async function importAccounts() {
  const credentials = form.value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
  if (!credentials.length) return Message.warning('请按一行一个凭证填写')
  saving.value = true
  importResult.value = ''
  pollAttempts = 0
  let success = 0
  const errors: string[] = []
  for (let index = 0; index < credentials.length; index += 1) {
    const name = form.name.trim() ? `${form.name.trim()}${credentials.length > 1 ? `-${index + 1}` : ''}` : ''
    const response = await api(`/tokens/${endpoints[form.provider]}`, { method: 'POST', body: JSON.stringify({ value: credentials[index], name }) })
    if (!response.ok) { errors.push(`第 ${index + 1} 行：${response.data?.detail || '导入失败'}`); continue }
    success += 1
    if (Number(form.weight) && response.data?.id) await api(`/tokens/${form.provider}/${response.data.id}`, { method: 'PATCH', body: JSON.stringify({ weight: Number(form.weight) }) })
  }
  saving.value = false
  importResult.value = `已受理 ${success} 个，失败 ${errors.length} 个${errors.length ? `；${errors.slice(0, 3).join('；')}` : '；账号资料将在后台自动回填'}`
  await load()
  if (!errors.length) {
    Message.success(`已受理 ${success} 个账号，正在识别资料与额度`)
    importOpen.value = false
    Object.assign(form, { provider: 'chatgpt', name: '', value: '', weight: 0 })
  }
}

function deleteAccount(row: any) {
  Modal.warning({ title: '删除 Provider 账号', content: `确认删除 ${row.account_email || row.email || row.id}？删除后无法恢复。`, hideCancel: false, okText: '确认删除', cancelText: '取消', onOk: async () => { const response = await api(`/tokens/${row.pool}/${row.id}`, { method: 'DELETE' }); if (!response.ok) return Message.error(response.data?.detail || '删除失败'); await load(); Message.success('账号已删除') } })
}

function deleteSelected() {
  if (!selectedKeys.value.length) return
  Modal.warning({ title: '批量删除账号', content: `确认删除选中的 ${selectedKeys.value.length} 个账号？`, hideCancel: false, okText: '确认删除', cancelText: '取消', onOk: async () => { const response = await api('/tokens/delete-bulk', { method: 'POST', body: JSON.stringify({ ids: selectedKeys.value }) }); if (!response.ok) return Message.error(response.data?.detail || '批量删除失败'); selectedKeys.value = []; await load(); Message.success(`已删除 ${response.data?.deleted || 0} 个账号`) } })
}

async function deleteDead() {
  const response = await api('/accounts?dead=1&limit=0')
  const ids = (response.data?.data || []).map((item: any) => item.id)
  if (!ids.length) return Message.info('当前没有失效账号')
  selectedKeys.value = ids
  deleteSelected()
}

function hasNumber(value: any) { return value !== null && value !== undefined && Number.isFinite(Number(value)) }
function quotaPercent(row: any) {
  if (!hasNumber(row.remaining) || !hasNumber(row.total) || Number(row.total) <= 0) return 0
  return Math.max(0, Math.min(1, Number(row.remaining) / Number(row.total)))
}
function statusLabel(row: any) {
  if (row.pending || row.status === 'pending') return '识别中'
  if (row.dead || row.status === 'disabled' || row.status === 'dead') return '凭证失效'
  if (row.status === 'quota') return '额度不足'
  return '正常'
}
function statusColor(row: any) {
  if (row.dead || row.status === 'disabled' || row.status === 'dead') return 'red'
  if (row.pending || row.status === 'pending' || row.status === 'quota') return 'orange'
  return 'green'
}

onMounted(load)
watch(active, () => { page.value = 1 })
onBeforeUnmount(() => { if (pollTimer) window.clearTimeout(pollTimer) })
const columns: any[] = [{ title: '账号', slotName: 'account', width: 290 }, { title: 'Provider', dataIndex: 'pool', width: 105 }, { title: '状态', slotName: 'status', width: 105 }, { title: '可用积分', slotName: 'quota', width: 190 }, { title: '成功任务', dataIndex: 'success_total', width: 100 }, { title: '失败', dataIndex: 'fails', width: 80 }, { title: '权重', dataIndex: 'weight', width: 80 }, { title: '操作', slotName: 'action', width: 128, fixed: 'right' }]
</script>

<template>
  <div>
    <div class="section-heading"><div><h2>Provider 账号</h2><p>批量维护上游凭证、账号资料、积分、权重与调用健康度。</p></div><a-space><a-button :loading="loading" @click="syncAll"><IconRefresh />同步账号与额度</a-button><a-button type="primary" @click="importOpen = true"><IconPlus />批量导入</a-button></a-space></div>
    <div class="provider-summary"><button v-for="item in [{ k: 'all', n: '全部账号', v: rows.length }, { k: 'active', n: '可调度', v: rows.filter(row => row.status === 'active' && !row.dead).length }, { k: 'quota', n: '额度不足', v: rows.filter(row => row.status === 'quota').length }, { k: 'dead', n: '凭证失效', v: deadCount }]" :key="item.k" :class="{ active: active === item.k }" @click="active = item.k"><span>{{ item.n }}</span><strong>{{ item.v }}</strong></button></div>
    <div class="batch-bar"><span>已选择 {{ selectedKeys.length }} 个账号</span><div><a-button v-if="selectedKeys.length" status="danger" @click="deleteSelected"><IconDelete />删除选中</a-button><a-button :disabled="!deadCount" @click="deleteDead"><IconDelete />清理失效账号<span v-if="deadCount">（{{ deadCount }}）</span></a-button></div></div>
    <a-table v-model:selected-keys="selectedKeys" :row-selection="{ type: 'checkbox', showCheckedAll: true }" row-key="id" :columns="columns" :data="filtered" :loading="loading" :pagination="pagination" :scroll="{ x: 1130 }" @page-change="page = $event">
      <template #account="{ record }"><div class="account"><span>{{ record.pool?.slice(0, 1).toUpperCase() }}</span><div><strong>{{ record.account_display_name || record.display_name || record.account_email || record.email || '资料识别中' }}</strong><small v-if="record.account_email || record.email">{{ record.account_email || record.email }}</small><small>{{ record.id }}</small></div></div></template>
      <template #status="{ record }"><a-tag :color="statusColor(record)">{{ statusLabel(record) }}</a-tag></template>
      <template #quota="{ record }"><div v-if="record.pool === 'grok'" class="quota-value"><strong>图 {{ record.image_remaining ?? '-' }} · 视频 {{ record.video_remaining ?? '-' }}</strong></div><div v-else-if="hasNumber(record.remaining)" class="quota-value"><strong>{{ Number(record.remaining).toLocaleString() }} 积分</strong><a-progress v-if="hasNumber(record.total) && Number(record.total) > 0" :percent="quotaPercent(record)" :show-text="false" size="small" color="#65735d" /><small v-if="hasNumber(record.total)">总额 {{ Number(record.total).toLocaleString() }}</small><small v-else>已完成额度同步</small></div><div v-else-if="record.pending || record.status === 'pending'" class="quota-value pending"><strong>正在识别</strong><small>后台校验凭证与账号资料</small></div><div v-else class="quota-value unknown" :title="record.quota_error || '上游未返回额度数据'"><strong>暂不可查</strong><small>{{ record.pool === 'adobe' ? '请配置可用代理后同步' : '上游未返回额度' }}</small></div></template>
      <template #action="{ record }"><div class="row-actions"><a-tooltip content="测试生图或视频"><button @click="testingAccount = record"><IconPlayCircle /></button></a-tooltip><a-tooltip content="同步账号与额度"><button @click="syncOne(record)"><IconRefresh /></button></a-tooltip><a-tooltip content="删除账号"><button class="danger" @click="deleteAccount(record)"><IconDelete /></button></a-tooltip></div></template>
    </a-table>
    <a-modal v-model:visible="importOpen" title="批量导入 Provider 账号" :ok-loading="saving" ok-text="开始导入" @ok="importAccounts"><a-form :model="form" layout="vertical"><div class="import-grid"><a-form-item label="Provider"><a-select v-model="form.provider"><a-option v-for="(_, key) in endpoints" :key="key" :value="key">{{ key }}</a-option></a-select></a-form-item><a-form-item label="账号名称前缀"><a-input v-model="form.name" placeholder="可选，例如 adobe-prod" /></a-form-item><a-form-item label="权重"><a-input-number v-model="form.weight" /></a-form-item></div><a-form-item label="凭证列表（一行一个）"><a-textarea v-model="form.value" :auto-size="{ minRows: 9, maxRows: 16 }" placeholder="每行粘贴一个 Token 或 Cookie&#10;第二个凭证继续换行粘贴" /></a-form-item><p class="import-tip">导入后系统会异步识别邮箱、昵称和积分，当前页面将自动刷新检测状态。</p><p v-if="importResult" class="import-result">{{ importResult }}</p></a-form></a-modal>
    <GenerationTestModal v-if="testingAccount" :account="testingAccount" @close="testingAccount = null" />
  </div>
</template>

<style scoped>
.provider-summary{display:grid;grid-template-columns:repeat(4,1fr);margin-bottom:14px;border:1px solid var(--ns-line);border-radius:7px;overflow:hidden}.provider-summary button{padding:16px 18px;border:0;border-right:1px solid var(--ns-line);background:#fff;text-align:left;display:flex;align-items:center;justify-content:space-between;cursor:pointer}.provider-summary button:last-child{border:0}.provider-summary button.active{background:var(--ns-accent-soft)}.provider-summary span{font-size:11px;color:var(--ns-ink-soft)}.provider-summary strong{font-size:18px}.batch-bar{min-height:45px;margin-bottom:10px;padding:6px 10px;border:1px solid var(--ns-line);border-radius:7px;background:#fafaf8;display:flex;align-items:center;justify-content:space-between;gap:12px}.batch-bar>span{font-size:10px;color:var(--ns-ink-faint)}.batch-bar>div{display:flex;gap:8px}.account{display:flex;align-items:center;gap:10px}.account>span{width:31px;height:31px;display:grid;place-items:center;background:#e5e7e1;border-radius:5px;font-weight:700}.account>div{display:flex;flex-direction:column;min-width:0}.account strong{font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.account small{font-size:9px;color:var(--ns-ink-faint);margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.quota-value{display:flex;flex-direction:column;align-items:flex-start;gap:3px}.quota-value strong{font-size:11px}.quota-value small{font-size:9px;color:var(--ns-ink-faint)}.quota-value .arco-progress{width:112px;margin-top:2px}.quota-value.unknown strong{color:var(--ns-warn)}.quota-value.pending strong{color:var(--ns-ink-soft)}.row-actions{display:flex;gap:3px}.row-actions button{width:30px;height:30px;display:grid;place-items:center;border:0;border-radius:5px;background:transparent;color:var(--ns-ink-soft);cursor:pointer}.row-actions button:hover{background:#eef0eb;color:var(--ns-ink)}.row-actions button.danger{color:var(--ns-danger)}.import-grid{display:grid;grid-template-columns:1.2fr 1.5fr .7fr;gap:12px}.import-tip,.import-result{font-size:10px;color:var(--ns-ink-faint)}.import-result{color:var(--ns-accent-strong)}@media(max-width:700px){.provider-summary{grid-template-columns:repeat(2,1fr)}.provider-summary button:nth-child(2){border-right:0}.provider-summary button:nth-child(-n+2){border-bottom:1px solid var(--ns-line)}.section-heading{align-items:flex-start;flex-direction:column}.batch-bar{align-items:flex-start;flex-direction:column}.import-grid{grid-template-columns:1fr}}
</style>

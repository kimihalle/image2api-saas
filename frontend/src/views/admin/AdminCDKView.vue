<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconCopy, IconDelete, IconGift, IconPlus, IconSearch } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'

const rows = ref<any[]>([])
const stats = ref<any>({ total: 0, active: 0, redeemed: 0, active_amount: 0, redeemed_amount: 0 })
const total = ref(0)
const loading = ref(false)
const selectedKeys = ref<string[]>([])
const page = ref(1)
const pageSize = 20
const query = ref('')
const status = ref('')
const type = ref('')
const createOpen = ref(false)
const saving = ref(false)
const created = ref<any[]>([])
const form = reactive({ amount: 100, count: 1, type: 'normal', note: '' })
let searchTimer: number | undefined

const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

async function load() {
  loading.value = true
  const params = new URLSearchParams({ limit: String(pageSize), offset: String((page.value - 1) * pageSize) })
  if (query.value.trim()) params.set('q', query.value.trim())
  if (status.value) params.set('status', status.value)
  if (type.value) params.set('type', type.value)
  const response = await api(`/cdks?${params}`)
  loading.value = false
  if (!response.ok) return Message.error(response.data?.detail || '兑换码加载失败')
  rows.value = response.data?.data || []
  stats.value = response.data?.stats || stats.value
  total.value = Number(response.data?.total || 0)
  selectedKeys.value = selectedKeys.value.filter((key) => rows.value.some((item) => item.code === key))
}

function filterChanged() { page.value = 1; load() }
watch(query, () => { if (searchTimer) window.clearTimeout(searchTimer); searchTimer = window.setTimeout(filterChanged, 300) })

async function createBatch() {
  if (form.amount <= 0) return Message.warning('额度必须大于 0')
  if (form.count < 1 || form.count > 500) return Message.warning('单次生成数量为 1 至 500 个')
  saving.value = true
  const response = await api('/cdks', { method: 'POST', body: JSON.stringify(form) })
  saving.value = false
  if (!response.ok) return Message.error(response.data?.detail || '生成失败')
  created.value = response.data?.created || []
  await load()
  Message.success(`已生成 ${created.value.length} 个兑换码`)
}

async function copyText(value: string, success = '已复制兑换码') {
  try { await navigator.clipboard.writeText(value); Message.success(success) } catch { Message.error('复制失败，请检查浏览器权限') }
}

function copyAll() { copyText(created.value.map((item) => item.code).join('\n'), `已复制 ${created.value.length} 个兑换码`) }

function deleteOne(row: any) {
  Modal.warning({
    title: '删除兑换码', content: `确认删除 ${row.code}？此操作不可撤销。`, okText: '确认删除', hideCancel: false,
    async onOk() {
      const response = await api(`/cdks/${encodeURIComponent(row.code)}`, { method: 'DELETE' })
      if (!response.ok) { Message.error(response.data?.detail || '删除失败'); return false }
      await load(); Message.success('兑换码已删除'); return true
    },
  })
}

function deleteSelected() {
  if (!selectedKeys.value.length) return Message.warning('请先选择兑换码')
  Modal.warning({
    title: '批量删除', content: `确认删除选中的 ${selectedKeys.value.length} 个兑换码？`, okText: '确认删除', hideCancel: false,
    async onOk() {
      const response = await api('/cdks/delete-bulk', { method: 'POST', body: JSON.stringify({ codes: selectedKeys.value }) })
      if (!response.ok) { Message.error(response.data?.detail || '批量删除失败'); return false }
      selectedKeys.value = []; await load(); Message.success(`已删除 ${response.data?.deleted || 0} 个兑换码`); return true
    },
  })
}

function formatTime(value: any) {
  if (!value) return '-'
  const date = new Date(Number(value) * 1000)
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString('zh-CN', { hour12: false })
}

onMounted(load)
const columns: any[] = [
  { title: '兑换码', slotName: 'code', width: 245 },
  { title: '额度', slotName: 'amount', width: 100 },
  { title: '类型', slotName: 'type', width: 100 },
  { title: '状态', slotName: 'status', width: 100 },
  { title: '兑换用户', slotName: 'user', width: 150 },
  { title: '备注', dataIndex: 'note', ellipsis: true, tooltip: true },
  { title: '创建时间', slotName: 'created', width: 170 },
  { title: '操作', slotName: 'actions', width: 112, fixed: 'right' },
]
</script>

<template>
  <div>
    <div class="section-heading"><div><h2>兑换码管理</h2><p>生成额度兑换码，跟踪发放、兑换和核销情况。</p></div><a-button type="primary" @click="created = []; createOpen = true"><IconPlus />生成兑换码</a-button></div>
    <section class="stat-strip"><div><span>兑换码总数</span><strong>{{ Number(stats.total || 0).toLocaleString() }}</strong></div><div><span>可用数量</span><strong>{{ Number(stats.active || 0).toLocaleString() }}</strong></div><div><span>已兑换数量</span><strong>{{ Number(stats.redeemed || 0).toLocaleString() }}</strong></div><div><span>待兑换额度</span><strong>{{ Number(stats.active_amount || 0).toLocaleString() }}</strong></div><div><span>已发放额度</span><strong>{{ Number(stats.redeemed_amount || 0).toLocaleString() }}</strong></div></section>
    <div class="toolbar"><a-input v-model="query" placeholder="搜索兑换码" allow-clear><template #prefix><IconSearch /></template></a-input><a-select v-model="status" @change="filterChanged"><a-option value="">全部状态</a-option><a-option value="active">未使用</a-option><a-option value="used">已使用</a-option></a-select><a-select v-model="type" @change="filterChanged"><a-option value="">全部类型</a-option><a-option value="normal">普通码</a-option><a-option value="marketing">营销码</a-option></a-select><a-button v-if="selectedKeys.length" status="danger" @click="deleteSelected"><IconDelete />删除选中（{{ selectedKeys.length }}）</a-button><span v-else>共 {{ total }} 条</span></div>
    <a-table v-model:selected-keys="selectedKeys" :columns="columns" :data="rows" :loading="loading" :pagination="false" row-key="code" :row-selection="{ type: 'checkbox', showCheckedAll: true }" :scroll="{ x: 1180 }">
      <template #empty><a-empty description="暂无兑换码" /></template>
      <template #code="{ record }"><div class="code-cell"><code>{{ record.code }}</code><a-button type="text" size="mini" aria-label="复制兑换码" @click="copyText(record.code)"><IconCopy /></a-button></div></template>
      <template #amount="{ record }"><strong class="amount">{{ Number(record.amount || 0).toLocaleString() }}</strong></template>
      <template #type="{ record }"><a-tag>{{ record.type === 'marketing' ? '营销码' : '普通码' }}</a-tag></template>
      <template #status="{ record }"><a-tag :color="record.status === 'active' ? 'green' : 'gray'">{{ record.status === 'active' ? '未使用' : '已兑换' }}</a-tag></template>
      <template #user="{ record }"><span class="muted">{{ record.redeemed_by_name || '-' }}</span></template>
      <template #created="{ record }"><span class="muted">{{ formatTime(record.created_at) }}</span></template>
      <template #actions="{ record }"><div class="action-row"><a-tooltip content="复制"><a-button type="text" aria-label="复制" @click="copyText(record.code)"><IconCopy /></a-button></a-tooltip><a-tooltip content="删除"><a-button type="text" status="danger" aria-label="删除" @click="deleteOne(record)"><IconDelete /></a-button></a-tooltip></div></template>
    </a-table>
    <nav v-if="pages > 1" class="page-list" aria-label="兑换码分页"><button v-for="item in pages" :key="item" :class="{ active: item === page }" @click="page = item; load()">{{ item }}</button></nav>

    <a-modal v-model:visible="createOpen" :width="600" title="生成兑换码" :footer="false">
      <a-form :model="form" layout="vertical"><div class="form-grid"><a-form-item label="单码额度"><a-input-number v-model="form.amount" :min="1" :precision="0" /></a-form-item><a-form-item label="生成数量"><a-input-number v-model="form.count" :min="1" :max="500" :precision="0" /></a-form-item><a-form-item label="兑换码类型"><a-select v-model="form.type"><a-option value="normal">普通码</a-option><a-option value="marketing">营销码</a-option></a-select></a-form-item><a-form-item label="批次备注"><a-input v-model="form.note" placeholder="可选，用于记录发放场景" /></a-form-item></div><p class="type-note">营销码同一批次每位用户仅可兑换一次；普通码不限制同批次兑换次数。</p><div class="modal-actions"><a-button @click="createOpen = false">关闭</a-button><a-button type="primary" :loading="saving" @click="createBatch"><IconGift />确认生成</a-button></div></a-form>
      <section v-if="created.length" class="created-block"><div><strong>本批已生成 {{ created.length }} 个</strong><a-button type="text" @click="copyAll"><IconCopy />复制全部</a-button></div><pre>{{ created.map((item) => item.code).join('\n') }}</pre></section>
    </a-modal>
  </div>
</template>

<style scoped>
.action-row{display:flex;align-items:center;flex-wrap:nowrap;white-space:nowrap}.action-row :deep(.arco-btn){flex:0 0 auto}
.stat-strip{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));border:1px solid var(--ns-line);border-radius:8px;background:#fff;margin-bottom:18px}.stat-strip>div{display:flex;flex-direction:column;padding:18px 20px}.stat-strip>div+div{border-left:1px solid var(--ns-line)}.stat-strip span{font-size:10px;color:var(--ns-ink-faint)}.stat-strip strong{margin-top:8px;font-size:21px}.toolbar{display:grid;grid-template-columns:minmax(220px,340px) 140px 140px auto;align-items:center;gap:10px;margin-bottom:14px}.toolbar>span{text-align:right;color:var(--ns-ink-faint);font-size:11px}.code-cell{display:flex;align-items:center;gap:5px}.code-cell code{font-size:11px;font-weight:650;color:var(--ns-ink)}.amount{font-size:12px}.muted{color:var(--ns-ink-soft);font-size:10px}.page-list{display:flex;justify-content:flex-end;gap:6px;margin-top:16px}.page-list button{width:32px;height:32px;border:1px solid var(--ns-line);border-radius:6px;background:#fff;color:var(--ns-ink-soft);cursor:pointer}.page-list button.active{background:#252b26;border-color:#252b26;color:#fff}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 12px}.type-note{margin:-2px 0 18px;color:var(--ns-ink-faint);font-size:10px}.modal-actions{display:flex;justify-content:flex-end;gap:8px}.created-block{margin-top:20px;padding-top:18px;border-top:1px solid var(--ns-line)}.created-block>div{display:flex;align-items:center;justify-content:space-between}.created-block pre{max-height:220px;overflow:auto;margin:9px 0 0;padding:12px;border-radius:7px;background:#272c28;color:#f2f3ef;font-size:11px;line-height:1.8}@media(max-width:900px){.stat-strip{grid-template-columns:repeat(2,1fr)}.stat-strip>div+div{border-left:0}.stat-strip>div:nth-child(even){border-left:1px solid var(--ns-line)}.stat-strip>div:nth-child(n+3){border-top:1px solid var(--ns-line)}.stat-strip>div:last-child{grid-column:1/-1}.toolbar{grid-template-columns:1fr 130px 130px}.toolbar>span,.toolbar>.arco-btn{grid-column:1/-1;justify-self:end}}@media(max-width:560px){.toolbar{grid-template-columns:1fr 1fr}.toolbar>.arco-input-wrapper{grid-column:1/-1}.form-grid{grid-template-columns:1fr}.stat-strip>div{padding:14px}.stat-strip strong{font-size:18px}}
</style>

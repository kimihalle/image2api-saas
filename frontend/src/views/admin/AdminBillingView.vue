<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { IconSave as IconDownload, IconSearch } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'
const rows = ref<any[]>([])
const ledger = ref<any[]>([])
const query = ref('')
const status = ref('all')
const orderPage = ref(1)
const ledgerPage = ref(1)
const orderTotal = ref(0)
const ledgerTotal = ref(0)
const orderLoading = ref(false)
const ledgerLoading = ref(false)
const exporting = ref(false)
const orderStats = ref<any>({ paid_count: 0, paid_amount: 0, paid_points: 0 })
const ledgerStats = ref<any>({ captured_count: 0, captured_amount: 0, refunded_count: 0, refunded_amount: 0 })
const pageSize = 20
let filterTimer: number | undefined

const orderPagination = computed(() => ({ current: orderPage.value, pageSize, total: orderTotal.value, showTotal: true }))
const ledgerPagination = computed(() => ({ current: ledgerPage.value, pageSize, total: ledgerTotal.value, showTotal: true }))

function orderParams(limit = pageSize, offset = (orderPage.value - 1) * pageSize) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  if (status.value !== 'all') params.set('status', status.value)
  if (query.value.trim()) params.set('q', query.value.trim())
  return params
}

async function loadOrders() {
  orderLoading.value = true
  try {
    const response = await api(`/pay/admin/orders?${orderParams().toString()}`)
    if (!response.ok) return
    rows.value = response.data?.data || []
    orderTotal.value = Number(response.data?.total || 0)
    orderStats.value = response.data?.stats || orderStats.value
  } finally {
    orderLoading.value = false
  }
}

async function loadLedger() {
  ledgerLoading.value = true
  try {
    const offset = (ledgerPage.value - 1) * pageSize
    const response = await api(`/billing/ledger/admin?limit=${pageSize}&offset=${offset}`)
    if (!response.ok) return
    ledger.value = response.data?.data || []
    ledgerTotal.value = Number(response.data?.total || 0)
    ledgerStats.value = response.data?.stats || ledgerStats.value
  } finally {
    ledgerLoading.value = false
  }
}

function changeOrderPage(page: number) { orderPage.value = page; loadOrders() }
function changeLedgerPage(page: number) { ledgerPage.value = page; loadLedger() }

async function exportCSV() {
  exporting.value = true
  try {
    const response = await api(`/pay/admin/orders?${orderParams(Math.max(orderTotal.value, pageSize), 0).toString()}`)
    if (!response.ok) return
    const exportRows = response.data?.data || []
    const fields = ['id', 'user_name', 'source', 'amount', 'points', 'pay_type', 'status', 'created_at']
    const csv = [fields.join(','), ...exportRows.map((row: any) => fields.map((field) => `"${String(row[field] ?? '').replaceAll('"', '""')}"`).join(','))].join('\n')
    const link = document.createElement('a')
    link.href = URL.createObjectURL(new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' }))
    link.download = `northstar-orders-${Date.now()}.csv`
    link.click()
    URL.revokeObjectURL(link.href)
  } finally {
    exporting.value = false
  }
}

watch([query, status], () => {
  orderPage.value = 1
  if (filterTimer) window.clearTimeout(filterTimer)
  filterTimer = window.setTimeout(loadOrders, 250)
})
onMounted(() => { loadOrders(); loadLedger() })
onBeforeUnmount(() => { if (filterTimer) window.clearTimeout(filterTimer) })

const columns = [{ title: '订单/交易', slotName: 'order', width: 230 }, { title: '用户', dataIndex: 'user_name' }, { title: '来源', slotName: 'source' }, { title: '金额', slotName: 'amount' }, { title: '额度变动', slotName: 'points' }, { title: '状态', slotName: 'status' }, { title: '时间', dataIndex: 'created_at' }]
const ledgerColumns = [{ title: '流水号', dataIndex: 'id' }, { title: '用户 ID', dataIndex: 'user_id' }, { title: '关联任务', dataIndex: 'event_id' }, { title: '用途', dataIndex: 'reason' }, { title: '额度', dataIndex: 'amount' }, { title: '状态', slotName: 'ledgerStatus' }, { title: '余额', dataIndex: 'balance_after' }]
</script>
<template><div><div class="section-heading"><div><h2>订单与账本</h2><p>支付、管理员调账、生成预留、确认扣费和退款。</p></div><a-button :disabled="!orderTotal" :loading="exporting" @click="exportCSV"><IconDownload />导出账单</a-button></div><section class="billing-metrics"><div><span>已支付收入</span><strong>¥ {{ Number(orderStats.paid_amount || 0).toFixed(2) }}</strong><small>{{ Number(orderStats.paid_count || 0).toLocaleString() }} 笔订单</small></div><div><span>充值额度</span><strong>{{ Number(orderStats.paid_points || 0).toLocaleString() }}</strong><small>真实到账记录</small></div><div><span>已确认扣费</span><strong>{{ Number(ledgerStats.captured_amount || 0).toLocaleString() }}</strong><small>{{ Number(ledgerStats.captured_count || 0).toLocaleString() }} 笔流水</small></div><div><span>失败退款</span><strong>{{ Number(ledgerStats.refunded_amount || 0).toLocaleString() }}</strong><small>{{ Number(ledgerStats.refunded_count || 0).toLocaleString() }} 笔流水</small></div></section><div class="toolbar"><a-input v-model="query" placeholder="搜索订单号或用户" allow-clear><template #prefix><IconSearch /></template></a-input><a-select v-model="status"><a-option value="all">全部状态</a-option><a-option value="paid">已支付</a-option><a-option value="pending">待支付</a-option><a-option value="cancelled">已取消</a-option></a-select></div><a-tabs default-active-key="ledger"><a-tab-pane key="ledger" title="生成账本"><a-table :columns="ledgerColumns" :data="ledger" :loading="ledgerLoading" :pagination="ledgerPagination" row-key="id" :scroll="{ x: 980 }" @page-change="changeLedgerPage"><template #ledgerStatus="{ record }"><a-tag :color="record.status === 'captured' ? 'green' : record.status === 'refunded' ? 'orange' : 'gray'">{{ record.status === 'captured' ? '已确认' : record.status === 'refunded' ? '已退款' : '预扣中' }}</a-tag></template></a-table></a-tab-pane><a-tab-pane key="orders" title="充值订单"><a-table :columns="columns" :data="rows" :loading="orderLoading" :pagination="orderPagination" row-key="id" :scroll="{ x: 1050 }" @page-change="changeOrderPage"><template #order="{ record }"><div class="order"><strong>{{ record.id }}</strong><small>{{ record.pay_type }}</small></div></template><template #source="{ record }"><a-tag>{{ record.source || 'epay' }}</a-tag></template><template #amount="{ record }">¥ {{ Number(record.amount).toFixed(2) }}</template><template #points="{ record }"><strong class="positive">+{{ record.points }}</strong></template><template #status="{ record }"><a-tag :color="record.status === 'paid' ? 'green' : 'orange'">{{ record.status === 'paid' ? '已完成' : record.status === 'cancelled' ? '已取消' : '待支付' }}</a-tag></template></a-table></a-tab-pane></a-tabs></div></template>
<style scoped>.billing-metrics{display:grid;grid-template-columns:repeat(4,1fr);border:1px solid var(--ns-line);background:#fff;border-radius:var(--ns-radius);margin-bottom:18px}.billing-metrics>div{padding:18px 20px;border-right:1px solid var(--ns-line);display:flex;flex-direction:column}.billing-metrics>div:last-child{border:0}.billing-metrics span{font-size:10px;color:var(--ns-ink-soft)}.billing-metrics strong{font-size:20px;margin:8px 0 5px}.billing-metrics small{font-size:9px;color:var(--ns-accent)}.toolbar{display:grid;grid-template-columns:minmax(250px,360px) 150px;gap:10px;margin-bottom:14px}.order{display:flex;flex-direction:column}.order strong{font:11px ui-monospace}.order small{font-size:9px;color:var(--ns-ink-faint);margin-top:3px}.positive{color:var(--ns-accent-strong)}@media(max-width:800px){.billing-metrics{grid-template-columns:repeat(2,1fr)}.billing-metrics>div:nth-child(2){border-right:0}.billing-metrics>div:nth-child(-n+2){border-bottom:1px solid var(--ns-line)}.toolbar{grid-template-columns:1fr 140px}}
</style>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconAlipayCircle, IconCheckCircle, IconCheckCircleFill, IconGift, IconHistory, IconQrcode, IconRefresh, IconThunderbolt, IconWechatpay } from '@arco-design/web-vue/es/icon'
import QRCode from 'qrcode'
import { api } from '../../services/api'
import { useAuthStore } from '../../stores/auth'
import { useSiteStore } from '../../stores/site'

const auth = useAuthStore()
const site = useSiteStore()
const orders = ref<any[]>([])
const ledger = ref<any[]>([])
const orderTotal = ref(0)
const ledgerTotal = ref(0)
const orderPage = ref(1)
const ledgerPage = ref(1)
const orderLoading = ref(false)
const ledgerLoading = ref(false)
const pageSize = 20
const payConfig = ref<any>({ enabled: false, methods: [], min_amount: 0, points_ratio: 0 })
const selectedPackage = ref('')
const rechargeMode = ref<'package' | 'custom'>('package')
const rechargeOpen = ref(false)
const redeemOpen = ref(false)
const busy = ref(false)
const redeeming = ref(false)
const form = reactive({ amount: 20, method: '' })
const redeemCode = ref('')
const cashierOpen = ref(false)
const currentOrder = ref<any>(null)
const qrDataURL = ref('')
const remainingSeconds = ref(0)
let pollTimer: number | undefined
let countdownTimer: number | undefined

const captured = computed(() => ledger.value.filter((item) => item.status === 'captured').reduce((sum, item) => sum + Number(item.amount || 0), 0))
const refunded = computed(() => ledger.value.filter((item) => item.status === 'refunded').reduce((sum, item) => sum + Number(item.amount || 0), 0))
const availablePackages = computed(() => (payConfig.value.packages || []).filter((item: any) => item.enabled !== false))
const selectedPoints = computed(() => Number(availablePackages.value.find((item: any) => item.id === selectedPackage.value)?.points || 0))
const effectiveMinAmount = computed(() => Math.max(0.01, Number(payConfig.value.min_amount || 0)))
const customPoints = computed(() => Math.round(Number(form.amount || 0) * Number(payConfig.value.points_ratio || 0)))
const rechargePoints = computed(() => rechargeMode.value === 'package' ? selectedPoints.value : customPoints.value)
const quickAmounts = computed(() => [...new Set([effectiveMinAmount.value, 20, 50, 100, 200].filter((value) => value >= effectiveMinAmount.value))].slice(0, 5))
const selectedPackageItem = computed(() => availablePackages.value.find((item: any) => item.id === selectedPackage.value) || null)
const bestPackageID = computed(() => {
  const sorted = [...availablePackages.value].sort((a: any, b: any) => (Number(b.points || 0) / Math.max(Number(b.amount || 0), 0.01)) - (Number(a.points || 0) / Math.max(Number(a.amount || 0), 0.01)))
  return sorted[0]?.id || ''
})
const selectedMethodName = computed(() => form.method === 'wxpay' ? '微信支付' : form.method === 'alipay' ? '支付宝' : form.method || '未选择')
const countdownText = computed(() => `${String(Math.floor(remainingSeconds.value / 60)).padStart(2, '0')}:${String(remainingSeconds.value % 60).padStart(2, '0')}`)
const purchaseURL = computed(() => String(site.contact.shop || '').trim())
const orderPagination = computed(() => ({ current: orderPage.value, pageSize, total: orderTotal.value, showTotal: true }))
const ledgerPagination = computed(() => ({ current: ledgerPage.value, pageSize, total: ledgerTotal.value, showTotal: true }))

async function load() {
  const [config] = await Promise.all([api('/pay/config'), loadOrders(), loadLedger()])
  if (config.ok) {
    payConfig.value = config.data
    form.method = config.data?.methods?.[0] || ''
    const firstPackage = config.data?.packages?.find((item: any) => item.enabled !== false)
    selectedPackage.value = firstPackage?.id || ''
    if (firstPackage) form.amount = Number(firstPackage.amount || 0)
  }
}

async function loadOrders() {
  orderLoading.value = true
  try {
    const offset = (orderPage.value - 1) * pageSize
    const response = await api(`/pay/orders?limit=${pageSize}&offset=${offset}`)
    if (!response.ok) return
    orders.value = response.data?.data || []
    orderTotal.value = Number(response.data?.total || 0)
  } finally {
    orderLoading.value = false
  }
}

async function loadLedger() {
  ledgerLoading.value = true
  try {
    const offset = (ledgerPage.value - 1) * pageSize
    const response = await api(`/billing/ledger?limit=${pageSize}&offset=${offset}`)
    if (!response.ok) return
    ledger.value = response.data?.data || []
    ledgerTotal.value = Number(response.data?.total || 0)
  } finally {
    ledgerLoading.value = false
  }
}

function changeOrderPage(value: number) {
  orderPage.value = value
  loadOrders()
}

function changeLedgerPage(value: number) {
  ledgerPage.value = value
  loadLedger()
}

function openRecharge() {
  if (!payConfig.value.enabled) return Message.warning('在线充值暂未开放，可使用兑换码充值')
  if (rechargeMode.value === 'package' && !selectedPackage.value && availablePackages.value.length) choosePackage(availablePackages.value[0])
  rechargeOpen.value = true
}

function choosePackage(item: any) {
  rechargeMode.value = 'package'
  selectedPackage.value = item.id
  form.amount = Number(item.amount || 0)
}

function setRechargeMode(mode: 'package' | 'custom') {
  rechargeMode.value = mode
  if (mode === 'package') {
    const item = availablePackages.value.find((value: any) => value.id === selectedPackage.value) || availablePackages.value[0]
    if (item) choosePackage(item)
    return
  }
  selectedPackage.value = ''
  form.amount = Math.max(effectiveMinAmount.value, Number(form.amount || 0))
}

async function recharge() {
  if (rechargeMode.value === 'package' && !selectedPackage.value) return Message.warning('请选择充值套餐')
  if (!form.method) return Message.warning('请选择支付方式')
  if (!form.amount || form.amount < effectiveMinAmount.value) return Message.warning(`最低充值金额为 ¥${effectiveMinAmount.value.toFixed(2)}`)
  busy.value = true
  const response = await api('/pay/recharge', { method: 'POST', body: JSON.stringify({ amount: Number(form.amount), method: form.method, package_id: rechargeMode.value === 'package' ? selectedPackage.value : '' }) })
  busy.value = false
  if (!response.ok) return Message.error(response.data?.detail || '创建充值订单失败')
  rechargeOpen.value = false
  const order = response.data
  Message.success(`订单 ${order.id || ''} 已创建`)
  await openCashier(order)
  await load()
  watchOrder(order.id)
}

async function openCashier(order: any) {
  currentOrder.value = order
  cashierOpen.value = true
  qrDataURL.value = ''
  if (order?.pay_info_type === 'qrcode' && order?.pay_info) {
    try { qrDataURL.value = await QRCode.toDataURL(String(order.pay_info), { width: 260, margin: 1, color: { dark: '#202521', light: '#ffffff' } }) } catch { Message.error('支付二维码生成失败') }
  }
  startCountdown(order)
}

function startCountdown(order: any) {
  if (countdownTimer) window.clearInterval(countdownTimer)
  const serverNow = Number(order?.server_now || Math.floor(Date.now() / 1000))
  const expiresAt = Number(order?.expires_at || serverNow)
  const localStarted = Date.now()
  const update = () => { remainingSeconds.value = Math.max(0, expiresAt - serverNow - Math.floor((Date.now() - localStarted) / 1000)) }
  update()
  countdownTimer = window.setInterval(update, 1000)
}

function openPayPage() {
  const url = String(currentOrder.value?.pay_info || '')
  if (!url) return Message.warning('订单没有可用的支付地址')
  window.open(url, '_blank', 'noopener,noreferrer')
}

function openCDKShop() {
  if (!purchaseURL.value) return Message.info('暂未配置兑换码购买地址')
  try {
    const target = new URL(purchaseURL.value)
    if (!['http:', 'https:'].includes(target.protocol)) throw new Error('unsupported protocol')
    window.open(target.toString(), '_blank', 'noopener,noreferrer')
  } catch {
    Message.error('兑换码购买地址配置有误，请联系客服')
  }
}

async function continueOrder(order: any) {
  busy.value = true
  const response = await api(`/pay/orders/${order.id}/continue`, { method: 'POST' })
  busy.value = false
  if (!response.ok) return Message.error(response.data?.detail || '恢复支付失败')
  await openCashier(response.data)
  watchOrder(response.data?.id)
}

async function redeem() {
  const code = redeemCode.value.trim().toUpperCase()
  if (!code) return Message.warning('请输入兑换码')
  redeeming.value = true
  const response = await api('/auth/redeem-cdk', { method: 'POST', body: JSON.stringify({ code }) })
  redeeming.value = false
  if (!response.ok) return Message.error(response.data?.detail || '兑换失败')
  if (auth.user) auth.user.credits = Number(response.data?.credits || 0)
  redeemOpen.value = false
  redeemCode.value = ''
  await load()
  Message.success(`兑换成功，已到账 ${Number(response.data?.amount || 0).toLocaleString()} 额度`)
}

async function watchOrder(id: string) {
  if (!id) return
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = window.setInterval(async () => {
    const response = await api(`/pay/orders/${id}`)
    if (!response.ok) return
    const item = response.data
    const index = orders.value.findIndex((order) => order.id === id)
    if (index >= 0) orders.value[index] = item
    if (['paid', 'failed', 'expired', 'cancelled'].includes(item?.status)) {
      if (pollTimer) window.clearInterval(pollTimer)
      if (item?.status === 'paid') {
        currentOrder.value = item
        await auth.refreshUser()
        Message.success('支付成功，额度已到账')
      }
      await load()
    }
  }, 4000)
}

function formatTime(value: any) {
  if (!value) return '-'
  if (typeof value === 'number') return new Date(value * 1000).toLocaleString('zh-CN', { hour12: false })
  return String(value).replace('T', ' ').slice(0, 19)
}

onMounted(load)
onUnmounted(() => { if (pollTimer) window.clearInterval(pollTimer); if (countdownTimer) window.clearInterval(countdownTimer) })

const columns = [
  { title: '订单号', dataIndex: 'id', width: 220 }, { title: '金额', slotName: 'amount', width: 100 },
  { title: '到账额度', dataIndex: 'points', width: 110 }, { title: '方式', dataIndex: 'pay_type', width: 110 },
  { title: '状态', slotName: 'status', width: 100 }, { title: '时间', slotName: 'created', width: 180 }, { title: '操作', slotName: 'orderActions', width: 110 },
]
const ledgerColumns = [
  { title: '流水号', dataIndex: 'id', width: 220 }, { title: '关联任务', dataIndex: 'event_id', width: 200 },
  { title: '用途', dataIndex: 'reason' }, { title: '额度', slotName: 'ledgerAmount', width: 110 },
  { title: '结果', slotName: 'ledgerStatus', width: 120 }, { title: '余额', dataIndex: 'balance_after', width: 110 },
]
</script>

<template>
  <div>
    <div class="section-heading"><div><h2>账单与额度</h2><p>充值订单、兑换码入账、生成扣费和失败退款统一记录。</p></div><div class="heading-actions"><a-button @click="redeemOpen = true"><IconGift />兑换码充值</a-button><a-button type="primary" :disabled="!payConfig.enabled" @click="openRecharge"><IconThunderbolt />在线充值</a-button></div></div>
    <section class="balance-strip"><div><span>当前可用额度</span><strong><IconThunderbolt />{{ Number(auth.user?.credits || 0).toLocaleString() }}</strong><small>生成任务提交时预留额度，成功确认扣费，失败自动退回</small></div><div class="balance-actions"><div class="balance-meta"><span><small>已确认扣费</small><strong>{{ captured }}</strong></span><span><small>失败已退款</small><strong>{{ refunded }}</strong></span></div></div></section>
    <section class="ledger-flow"><div><b>01</b><IconHistory /><span><strong>预留额度</strong><small>提交生成任务时</small></span></div><div><b>02</b><IconCheckCircle /><span><strong>确认扣费</strong><small>Provider 返回成功</small></span></div><div><b>03</b><IconGift /><span><strong>失败退款</strong><small>失败或超时自动处理</small></span></div></section>
    <a-tabs default-active-key="ledger">
      <a-tab-pane key="ledger" title="额度流水"><a-table :columns="ledgerColumns" :data="ledger" :loading="ledgerLoading" :pagination="ledgerPagination" :scroll="{ x: 980 }" row-key="id" @page-change="changeLedgerPage"><template #empty><a-empty description="暂无额度流水" /></template><template #ledgerAmount="{ record }"><span :class="record.status === 'refunded' ? 'credit' : 'debit'">{{ record.status === 'refunded' ? '+' : '-' }}{{ Number(record.amount || 0).toLocaleString() }}</span></template><template #ledgerStatus="{ record }"><a-tag :color="record.status === 'captured' ? 'green' : record.status === 'refunded' ? 'orange' : 'gray'">{{ record.status === 'captured' ? '已确认扣费' : record.status === 'refunded' ? '已退款' : record.status === 'reserved' ? '预扣处理中' : '状态异常' }}</a-tag></template></a-table></a-tab-pane>
      <a-tab-pane key="orders" title="充值订单"><a-table :columns="columns" :data="orders" :loading="orderLoading" :pagination="orderPagination" :scroll="{ x: 1030 }" row-key="id" @page-change="changeOrderPage"><template #empty><a-empty description="暂无充值订单" /></template><template #amount="{ record }">¥ {{ Number(record.amount).toFixed(2) }}</template><template #status="{ record }"><a-tag :color="record.status === 'paid' ? 'green' : record.status === 'failed' ? 'red' : 'orange'">{{ record.status === 'paid' ? '已支付' : record.status === 'failed' ? '失败' : ['expired', 'cancelled'].includes(record.status) ? '已过期' : '待支付' }}</a-tag></template><template #created="{ record }">{{ formatTime(record.created_at) }}</template><template #orderActions="{ record }"><a-button v-if="record.status !== 'paid'" type="text" size="mini" :loading="busy" @click="continueOrder(record)"><IconRefresh />继续支付</a-button><span v-else class="muted">已到账</span></template></a-table></a-tab-pane>
    </a-tabs>

  <a-modal v-model:visible="redeemOpen" title="兑换码充值" modal-class="user-dialog">
    <div class="redeem-intro"><span><IconGift /></span><div><strong>兑换额度</strong><small>兑换成功后额度将立即计入当前账户</small></div></div>
    <a-form :model="{ code: redeemCode }" layout="vertical"><a-form-item label="兑换码"><a-input v-model="redeemCode" size="large" placeholder="XXXX-XXXX-XXXX-XXXX" allow-clear @press-enter="redeem" /></a-form-item></a-form>
    <p class="redeem-note">兑换码不区分大小写，已使用或失效的兑换码无法重复兑换。</p>
    <template #footer><div class="redeem-footer"><a-button @click="redeemOpen = false">取消</a-button><a-button v-if="purchaseURL" @click="openCDKShop"><IconGift />购买兑换码</a-button><a-button type="primary" :loading="redeeming" @click="redeem">确认兑换</a-button></div></template>
  </a-modal>
  <a-modal v-model:visible="rechargeOpen" modal-class="user-dialog" :modal-style="{ width: 'min(820px, calc(100vw - 32px))', maxHeight: 'calc(100vh - 32px)' }" :body-style="{ maxHeight: 'calc(100vh - 112px)', overflowY: 'auto' }" :footer="false" title="在线充值">
      <div class="recharge-dialog">
        <div class="recharge-overview"><div><span class="recharge-kicker">账户充值</span><h3>{{ rechargeMode === 'package' ? '选择适合你的额度套餐' : '按需要充值任意金额' }}</h3><p>支付成功后自动到账，可用于图片与视频生成。</p></div><div class="current-balance"><span>当前余额</span><strong><IconThunderbolt />{{ Number(auth.user?.credits || 0).toLocaleString() }}</strong></div></div>
        <section class="package-section">
          <div class="recharge-label"><strong>{{ rechargeMode === 'package' ? '充值套餐' : '充值金额' }}</strong><div class="recharge-modes"><button :class="{ active: rechargeMode === 'package' }" @click="setRechargeMode('package')">套餐充值</button><button :class="{ active: rechargeMode === 'custom' }" @click="setRechargeMode('custom')">自定义金额</button></div></div>
          <div v-if="rechargeMode === 'package'" class="package-picker"><button v-for="(item, index) in availablePackages" :key="item.id" type="button" class="package-option" :class="[`package-tone-${index % 3}`, { selected: selectedPackage === item.id }]" :data-tier="String(index + 1).padStart(2, '0')" @click="choosePackage(item)"><span class="package-head"><b>{{ item.name }}</b><em v-if="item.id === bestPackageID">性价比优选</em><IconCheckCircleFill v-if="selectedPackage === item.id" /></span><strong class="package-price"><small>¥</small>{{ Number(item.amount).toFixed(2) }}</strong><span class="package-credit"><IconThunderbolt />{{ Number(item.points).toLocaleString() }} 额度</span><small class="package-rate">每元约 {{ Math.round(Number(item.points || 0) / Math.max(Number(item.amount || 0), 0.01)).toLocaleString() }} 额度</small></button></div>
          <div v-else class="custom-amount-panel"><div class="amount-editor"><span class="currency-mark">¥</span><a-input-number v-model="form.amount" :min="effectiveMinAmount" :precision="2" :hide-button="true" placeholder="输入充值金额" /><small>最低充值 ¥{{ effectiveMinAmount.toFixed(2) }} · 每 1 元兑换 {{ Number(payConfig.points_ratio || 0).toLocaleString() }} 额度</small></div><div class="credit-preview"><span><IconThunderbolt />预计到账</span><div><strong>{{ customPoints.toLocaleString() }}</strong><small>额度</small></div></div><div class="quick-amounts"><span>快捷金额</span><button v-for="value in quickAmounts" :key="value" :class="{ active: Number(form.amount) === value }" @click="form.amount = value">¥{{ Number(value).toFixed(value % 1 ? 2 : 0) }}</button></div></div>
        </section>
        <div class="recharge-bottom"><section class="method-section"><div class="recharge-label"><strong>支付方式</strong><span>由支付平台安全处理</span></div><div class="method-picker"><button v-for="method in payConfig.methods || []" :key="method" type="button" :class="{ selected: form.method === method }" @click="form.method = method"><IconWechatpay v-if="method === 'wxpay'" /><IconAlipayCircle v-else-if="method === 'alipay'" /><IconQrcode v-else /><span>{{ method === 'wxpay' ? '微信支付' : method === 'alipay' ? '支付宝' : method }}</span><IconCheckCircleFill class="method-check" /></button></div></section><aside class="order-summary"><div><span>{{ rechargeMode === 'package' ? '已选套餐' : '充值类型' }}</span><strong>{{ rechargeMode === 'package' ? (selectedPackageItem?.name || '请选择套餐') : '自定义充值' }}</strong></div><div><span>到账额度</span><strong>{{ rechargePoints.toLocaleString() }}</strong></div><div><span>支付方式</span><strong>{{ selectedMethodName }}</strong></div><div class="summary-total"><span>应付金额</span><strong>¥ {{ Number(form.amount || 0).toFixed(2) }}</strong></div></aside></div>
        <div class="recharge-actions"><p><IconCheckCircle />支付结果将自动确认，无需手动提交凭证</p><div><a-button @click="rechargeOpen = false">取消</a-button><a-button type="primary" :loading="busy" :disabled="!form.method || form.amount < effectiveMinAmount || (rechargeMode === 'package' && !selectedPackage)" @click="recharge"><IconThunderbolt />确认支付 ¥ {{ Number(form.amount || 0).toFixed(2) }}</a-button></div></div>
      </div>
    </a-modal>
  <a-modal v-model:visible="cashierOpen" :width="440" title="支付订单" :footer="false" modal-class="user-dialog" @cancel="cashierOpen = false"><div v-if="currentOrder" class="cashier"><div class="cashier-amount"><span>应付金额</span><strong>¥ {{ Number(currentOrder.amount || 0).toFixed(2) }}</strong><small>到账 {{ Number(currentOrder.points || 0).toLocaleString() }} 额度</small></div><div v-if="currentOrder.status === 'paid'" class="cashier-success"><IconCheckCircle /><strong>支付成功，额度已到账</strong></div><template v-else><img v-if="qrDataURL" :src="qrDataURL" class="pay-qrcode" alt="支付二维码" /><a-button v-if="currentOrder.pay_info_type === 'jump'" type="primary" long @click="openPayPage"><IconThunderbolt />打开支付页面</a-button><div class="cashier-meta"><span>订单号</span><code>{{ currentOrder.id }}</code><span>剩余时间</span><strong>{{ countdownText }}</strong></div><p>支付完成后页面会自动确认并更新账户额度，请勿重复创建订单。</p></template></div></a-modal>
  </div>
</template>

<style scoped>
.heading-actions{display:flex;gap:8px}.balance-strip{display:flex;justify-content:space-between;gap:30px;padding:26px 28px;border:1px solid var(--ns-line);border-radius:8px;background:#fff}.balance-strip>div:first-child{display:flex;flex-direction:column}.balance-strip span,.balance-strip small{color:var(--ns-ink-soft);font-size:11px}.balance-strip>div:first-child>strong{display:flex;align-items:center;gap:8px;font-size:38px;letter-spacing:0;margin:7px 0}.balance-strip>div:first-child>strong :deep(svg){width:23px;color:#c9a62d}.balance-actions{display:flex;align-items:center}.balance-meta{display:flex;align-items:center}.balance-meta>span{min-width:116px;padding-left:24px;border-left:1px solid var(--ns-line);display:flex;flex-direction:column}.balance-meta strong{color:var(--ns-ink);font-size:18px;margin-top:7px}.ledger-flow{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));padding:24px 0;margin:18px 0 32px;border-block:1px solid var(--ns-line)}.ledger-flow>div{display:flex;align-items:center;justify-content:center;gap:10px;padding:0 24px}.ledger-flow>div+div{border-left:1px solid var(--ns-line)}.ledger-flow b{font:600 9px ui-monospace;color:var(--ns-ink-faint)}.ledger-flow>div>span{display:flex;flex-direction:column}.ledger-flow strong{font-size:12px}.ledger-flow small{font-size:10px;color:var(--ns-ink-faint);margin-top:3px}.ledger-flow :deep(svg){color:var(--ns-accent)}.pay-note,.redeem-note{font-size:10px;color:var(--ns-ink-soft)}.credit{color:#4f7047}.debit{color:var(--ns-ink)}.redeem-intro{display:flex;align-items:center;gap:11px;margin-bottom:18px;padding:12px;border:1px solid var(--ns-line);border-radius:7px;background:#fafaf7}.redeem-intro>span{width:36px;height:36px;display:grid;place-items:center;border-radius:50%;background:#f2e9bd;color:#9d7d10}.redeem-intro>div{display:flex;flex-direction:column}.redeem-intro strong{font-size:12px}.redeem-intro small{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}@media(max-width:980px){.balance-strip{flex-direction:column}.balance-actions{justify-content:flex-start}.balance-meta>span{padding:0 18px 0 0;border:0}}@media(max-width:700px){.section-heading{align-items:flex-start}.heading-actions{flex-direction:column}.ledger-flow{grid-template-columns:1fr}.ledger-flow>div{justify-content:flex-start;min-height:58px}.ledger-flow>div+div{border-left:0;border-top:1px solid var(--ns-line)}}
.recharge-dialog{color:var(--ns-ink)}.recharge-overview{display:flex;align-items:flex-start;justify-content:space-between;gap:28px;padding:3px 0 22px;border-bottom:1px solid var(--ns-line)}.recharge-kicker{display:block;margin-bottom:6px;color:#64705f;font-size:10px;font-weight:700}.recharge-overview h3{margin:0;font-size:20px;line-height:1.35;letter-spacing:0}.recharge-overview p{margin:7px 0 0;color:var(--ns-ink-faint);font-size:11px}.current-balance{min-width:142px;padding-left:22px;border-left:1px solid var(--ns-line);display:flex;flex-direction:column;align-items:flex-end}.current-balance span{color:var(--ns-ink-faint);font-size:10px}.current-balance strong{display:flex;align-items:center;gap:6px;margin-top:6px;font-size:19px}.current-balance :deep(svg){color:#b89421}.package-section{padding:22px 0 24px}.recharge-label{display:flex;align-items:center;justify-content:space-between;margin-bottom:11px}.recharge-label strong{font-size:12px}.recharge-label span{color:var(--ns-ink-faint);font-size:9px}.package-picker{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.package-option{min-height:154px;padding:17px;border:1px solid var(--ns-line);border-radius:8px;background:#fff;display:flex;flex-direction:column;align-items:flex-start;text-align:left;cursor:pointer;transition:border-color .18s ease,background-color .18s ease,box-shadow .18s ease}.package-option:hover{border-color:#9ca697}.package-option.selected{border-color:#65735f;background:#f4f6f1;box-shadow:0 0 0 1px #65735f}.package-head{width:100%;min-height:22px;display:flex;align-items:center;gap:6px}.package-head b{font-size:12px}.package-head em{margin-left:auto;padding:3px 6px;border-radius:4px;background:#e4eadf;color:#53644d;font-size:8px;font-style:normal;white-space:nowrap}.package-head>svg{margin-left:auto;color:#5d7056}.package-price{margin-top:12px;font-size:26px;line-height:1}.package-price small{margin-right:4px;color:var(--ns-ink);font-size:13px}.package-credit{display:flex;align-items:center;gap:5px;margin-top:14px;color:#485742;font-size:11px;font-weight:650}.package-credit :deep(svg){color:#bd9926}.package-rate{margin-top:7px;color:var(--ns-ink-faint);font-size:9px}.recharge-bottom{display:grid;grid-template-columns:minmax(0,1fr) 260px;border-top:1px solid var(--ns-line);border-bottom:1px solid var(--ns-line)}.method-section{padding:21px 24px 21px 0}.method-picker{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:9px}.method-picker button{height:50px;padding:0 13px;border:1px solid var(--ns-line);border-radius:7px;background:#fff;display:flex;align-items:center;gap:9px;color:var(--ns-ink-soft);cursor:pointer}.method-picker button:hover{border-color:#a5aca1}.method-picker button.selected{border-color:#667461;background:#f5f7f3;color:var(--ns-ink)}.method-picker button>svg:first-child{width:20px;height:20px;color:#5e6f59}.method-check{margin-left:auto;color:#60705b;opacity:0}.method-picker button.selected .method-check{opacity:1}.order-summary{padding:21px 0 21px 24px;border-left:1px solid var(--ns-line);display:flex;flex-direction:column;gap:10px}.order-summary>div{display:flex;justify-content:space-between;gap:14px;color:var(--ns-ink-faint);font-size:10px}.order-summary>div strong{max-width:145px;color:var(--ns-ink);font-size:10px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.order-summary .summary-total{align-items:flex-end;margin-top:auto;padding-top:12px;border-top:1px solid var(--ns-line)}.order-summary .summary-total strong{font-size:19px}.recharge-actions{padding-top:18px;display:flex;align-items:center;justify-content:space-between;gap:20px}.recharge-actions p{margin:0;display:flex;align-items:center;gap:6px;color:var(--ns-ink-faint);font-size:9px}.recharge-actions p :deep(svg){color:#63735d}.recharge-actions>div{display:flex;gap:8px}.muted{color:var(--ns-ink-faint);font-size:10px}.cashier{display:flex;flex-direction:column;align-items:center}.cashier-amount{width:100%;padding-bottom:18px;margin-bottom:18px;border-bottom:1px solid var(--ns-line);display:flex;flex-direction:column;align-items:center}.cashier-amount span,.cashier-amount small{color:var(--ns-ink-faint);font-size:10px}.cashier-amount strong{margin:5px 0;font-size:30px}.pay-qrcode{width:220px;height:220px;margin:0 auto 18px;border:8px solid #fff}.cashier-meta{width:100%;display:grid;grid-template-columns:auto minmax(0,1fr);gap:8px 14px;padding:14px 0;margin-top:14px;border-block:1px solid var(--ns-line);font-size:10px}.cashier-meta span{color:var(--ns-ink-faint)}.cashier-meta code,.cashier-meta strong{text-align:right;overflow:hidden;text-overflow:ellipsis}.cashier>p{margin:12px 0 0;color:var(--ns-ink-faint);font-size:9px;line-height:1.6;text-align:center}.cashier-success{min-height:130px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;color:#55734f}.cashier-success :deep(svg){width:38px;height:38px}@media(max-width:720px){.recharge-overview{align-items:stretch;flex-direction:column}.current-balance{padding:0;border:0;align-items:flex-start}.package-picker{margin-inline:-2px;padding:2px 2px 8px;display:grid;grid-auto-flow:column;grid-auto-columns:minmax(210px,78%);grid-template-columns:none;overflow-x:auto;scroll-snap-type:x proximity}.package-option{scroll-snap-align:start}.recharge-bottom{grid-template-columns:1fr}.method-section{padding-right:0}.order-summary{padding-left:0;border-left:0;border-top:1px solid var(--ns-line)}.recharge-actions{align-items:stretch;flex-direction:column}.recharge-actions>div{justify-content:flex-end}}@media(max-width:480px){.method-picker{grid-template-columns:1fr}}

/* 套餐自身就是选项容器，用不同材质色和几何编号建立层级。 */
.package-option{min-height:170px;padding:19px;position:relative;isolation:isolate;overflow:hidden;border-color:transparent;box-shadow:inset 0 0 0 1px rgba(44,52,45,.08);transition:transform .18s ease,box-shadow .18s ease,border-color .18s ease}.package-option::before{content:attr(data-tier);position:absolute;z-index:-1;right:9px;bottom:-20px;font:700 76px/1 ui-monospace;color:currentColor;opacity:.075}.package-option::after{content:'';width:88px;height:88px;position:absolute;z-index:-1;right:-48px;top:-48px;background:#fff;opacity:.28;transform:rotate(34deg)}.package-option:hover{border-color:transparent;transform:translateY(-2px);box-shadow:0 8px 18px rgba(43,50,43,.12),inset 0 0 0 1px rgba(44,52,45,.12)}.package-option.selected{border-color:#35473a;box-shadow:0 0 0 2px #35473a,0 10px 22px rgba(43,50,43,.14)}.package-option>*{position:relative;z-index:1}.package-tone-0,.package-option.package-tone-0.selected{color:#55451e;background:#f3e3aa}.package-tone-0 .package-credit,.package-tone-0 .package-price small{color:#55451e}.package-tone-0 .package-credit :deep(svg){color:#9b7310}.package-tone-1,.package-option.package-tone-1.selected{color:#304c3d;background:#cfe2d3}.package-tone-1 .package-credit,.package-tone-1 .package-price small{color:#304c3d}.package-tone-1 .package-credit :deep(svg){color:#4f765c}.package-tone-2,.package-option.package-tone-2.selected{color:#edf1e9;background:#314139}.package-tone-2 .package-price small,.package-tone-2 .package-credit,.package-tone-2 .package-rate{color:#edf1e9}.package-tone-2 .package-credit :deep(svg){color:#e4bd49}.package-tone-2 .package-head em{background:#e6cb71;color:#3d361d}.package-tone-2 .package-head>svg{color:#e6cb71}.package-head em{background:rgba(255,255,255,.52);color:inherit}.package-rate{opacity:.72}.method-picker button:first-child{background:#e7f0e5;border-color:#cfddcb;color:#3d6142}.method-picker button:nth-child(2){background:#e8f0ef;border-color:#cedddb;color:#3f6261}.method-picker button.selected{border-color:#506650;box-shadow:inset 0 0 0 1px #506650}.order-summary{margin-block:0;background:#f3f0e5}.order-summary>div,.order-summary .summary-total{padding-inline:16px}.order-summary>div:first-child{padding-top:2px}
</style>
<style scoped>
.recharge-modes{padding:3px;display:flex;gap:2px;border-radius:999px;background:#e8ebe5}.recharge-modes button{height:27px;padding:0 12px;border:0;border-radius:999px;background:transparent;color:var(--ns-ink-soft);font-size:9px;cursor:pointer}.recharge-modes button.active{background:#35453b;color:#fff;box-shadow:0 2px 7px rgba(40,55,45,.16)}.custom-amount-panel{display:grid;grid-template-columns:minmax(0,1.35fr) minmax(210px,.65fr);overflow:hidden;border:1px solid #d9ddd6;border-radius:8px;background:#f8f9f6}.amount-editor{min-width:0;padding:24px 26px;display:grid;grid-template-columns:auto minmax(0,1fr);align-items:center;column-gap:12px}.currency-mark{color:#35453b;font-size:27px;font-weight:750}.amount-editor :deep(.arco-input-wrapper){height:52px;padding-inline:4px;border:0;border-bottom:2px solid #46554b;border-radius:0;background:transparent;box-shadow:none}.amount-editor :deep(.arco-input-wrapper:hover),.amount-editor :deep(.arco-input-wrapper-focus){border-color:#b18e22;box-shadow:none}.amount-editor :deep(input){font-size:27px;font-weight:750;color:#29332d}.amount-editor>small{grid-column:1/-1;margin-top:10px;color:#788079;font-size:9px}.credit-preview{padding:22px 24px;border-left:1px solid #d5ddd2;background:#e7eee5;display:flex;flex-direction:column;justify-content:center}.credit-preview>span{display:flex;align-items:center;gap:6px;color:#5b6c5a;font-size:9px;font-weight:650}.credit-preview>span :deep(svg){color:#b48d18}.credit-preview>div{display:flex;align-items:baseline;gap:7px;margin-top:8px}.credit-preview strong{font-size:27px;line-height:1;color:#314336}.credit-preview small{color:#687668;font-size:9px}.quick-amounts{grid-column:1/-1;padding:12px 14px;border-top:1px solid #d9ddd6;background:#fff;display:flex;align-items:center;gap:7px}.quick-amounts>span{margin-right:auto;color:#7b827b;font-size:9px}.quick-amounts button{min-width:62px;height:32px;padding:0 12px;border:1px solid #d9ddd6;border-radius:6px;background:#f7f8f5;color:#4c574f;font-size:10px;font-weight:650;cursor:pointer}.quick-amounts button:hover{border-color:#9ba69d}.quick-amounts button.active{border-color:#cfb451;background:#eee0a7;color:#4d421d;box-shadow:inset 0 0 0 1px rgba(145,116,27,.12)}@media(max-width:720px){.recharge-overview{gap:12px;padding-bottom:15px}.current-balance{flex-direction:row;align-items:center;justify-content:space-between}.current-balance strong{margin-top:0}.package-section{padding:15px 0 17px}.recharge-modes button{padding:0 9px}.custom-amount-panel{grid-template-columns:1fr}.amount-editor{padding:18px}.amount-editor :deep(.arco-input-wrapper){height:45px}.credit-preview{padding:15px 18px;border-top:1px solid #d5ddd2;border-left:0;flex-direction:row;align-items:center;justify-content:space-between}.credit-preview>div{margin-top:0}.credit-preview strong{font-size:24px}.quick-amounts{padding:9px;overflow-x:auto}.quick-amounts>span{display:none}.quick-amounts button{flex:1 0 58px;height:30px;padding-inline:9px}.method-section{padding-block:16px}.method-picker button{height:45px;padding-inline:10px;gap:6px}.order-summary{padding:14px 0;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;background:#f3f0e5}.order-summary>div{padding-inline:13px;display:flex;flex-direction:column;gap:4px}.order-summary>div strong{max-width:none}.order-summary .summary-total{grid-column:1/-1;padding:11px 13px 0;flex-direction:row;align-items:flex-end}.recharge-actions{position:sticky;bottom:-1px;z-index:2;margin:0 -20px -20px;padding:13px 20px;background:#fff;border-top:1px solid var(--ns-line);box-shadow:0 -8px 20px rgba(35,43,37,.05)}.recharge-actions p{display:none}.recharge-actions>div,.recharge-actions :deep(.arco-btn-primary){width:100%}.recharge-actions>div>.arco-btn:first-child{display:none}}@media(max-width:450px){.recharge-label{align-items:flex-start;gap:9px;flex-direction:column}.method-picker{grid-template-columns:repeat(2,minmax(0,1fr))}.method-picker button>span{white-space:nowrap}.quick-amounts{padding:9px}}
.redeem-footer{display:flex;align-items:center;justify-content:flex-end;gap:8px}@media(max-width:480px){.redeem-footer{display:grid;grid-template-columns:1fr 1fr}.redeem-footer :deep(.arco-btn){width:100%;margin:0}.redeem-footer :deep(.arco-btn-primary){grid-column:1/-1;grid-row:1}}
</style>

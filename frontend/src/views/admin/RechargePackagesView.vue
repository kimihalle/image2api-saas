<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconDelete, IconPlus, IconSave } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'

type PackageRow = { id: string; name: string; amount: number; points: number; enabled: boolean; sort: number }
const loading = ref(false)
const saving = ref(false)
const pay = reactive({ enabled: false, pid: '', key: '', api_base: '', methods: ['wxpay', 'alipay'] as string[], min_amount: 1, points_ratio: 100, packages: [] as PackageRow[] })

function addPackage() {
  pay.packages.push({ id: `package-${Date.now()}`, name: '新套餐', amount: 10, points: 1000, enabled: true, sort: pay.packages.length + 1 })
}
function removePackage(index: number) { pay.packages.splice(index, 1) }
async function load() {
  loading.value = true
  const response = await api('/settings/pay')
  loading.value = false
  if (!response.ok) return Message.error(response.data?.detail || '支付设置加载失败')
  Object.assign(pay, response.data, { packages: response.data?.packages || [] })
}
async function save() {
  saving.value = true
  const response = await api('/settings/pay', { method: 'PUT', body: JSON.stringify({ ...pay, packages: pay.packages.map((item, index) => ({ ...item, amount: Number(item.amount) || 0, points: Number(item.points) || 0, sort: index + 1 })) }) })
  saving.value = false
  if (!response.ok) return Message.error(response.data?.detail || '套餐保存失败')
  Message.success('充值套餐已保存并同步到前台')
}
onMounted(load)
</script>

<template>
  <div class="packages-page">
    <div class="section-heading"><div><span class="eyebrow">RECHARGE CATALOG</span><h2>充值套餐</h2><p>管理用户在线充值时看到的额度套餐、价格和上下架状态。</p></div><a-space><a-button @click="addPackage"><IconPlus />新增套餐</a-button><a-button type="primary" :loading="saving" @click="save"><IconSave />保存套餐</a-button></a-space></div>
    <a-alert v-if="!pay.enabled" type="warning" title="在线充值当前未开启" description="套餐可以先配置保存；开启支付设置中的在线充值后，前台才会展示并允许下单。" class="pay-alert" />
    <a-spin :loading="loading" class="package-list">
      <div class="list-head"><span>套餐 ID</span><span>名称</span><span>金额（元）</span><span>到账额度</span><span>前台显示</span><span></span></div>
      <div v-for="(item, index) in pay.packages" :key="item.id || index" class="package-row"><a-input v-model="item.id" placeholder="unique-id" /><a-input v-model="item.name" placeholder="例如创作者包" /><a-input-number v-model="item.amount" :min="0.01" :precision="2" /><a-input-number v-model="item.points" :min="1" :precision="0" /><a-switch v-model="item.enabled" size="small" /><a-button type="text" status="danger" shape="circle" aria-label="删除套餐" @click="removePackage(index)"><IconDelete /></a-button></div>
      <a-empty v-if="!pay.packages.length" description="暂无套餐，点击新增套餐开始配置" />
    </a-spin>
    <div class="catalog-note"><strong>运营提示</strong><p>订单创建时服务端会重新读取套餐配置并校验金额和到账额度；修改套餐不会影响已支付订单。停用套餐只会阻止新订单，历史账单仍然保留。</p></div>
  </div>
</template>

<style scoped>
.packages-page{max-width:1120px}.eyebrow{display:block;margin-bottom:6px;color:#8a7628;font-size:9px;font-weight:750;letter-spacing:.13em}.pay-alert{margin-bottom:18px}.package-list{display:block;border-top:1px solid var(--ns-line);border-bottom:1px solid var(--ns-line);background:#fff}.list-head,.package-row{display:grid;grid-template-columns:1.1fr 1.3fr 140px 140px 72px 38px;align-items:center;gap:10px;padding:13px 16px}.list-head{color:var(--ns-ink-faint);font-size:10px;background:#f7f8f4}.list-head span:nth-child(5){text-align:center}.package-row{border-top:1px solid var(--ns-line)}.package-row :deep(.arco-input-number){width:100%}.package-row :deep(.arco-switch){width:28px;justify-self:center}.package-row :deep(.arco-btn-shape-circle){width:30px;min-width:30px;height:30px;padding:0;justify-self:center}.catalog-note{margin-top:22px;padding:16px 18px;border-left:3px solid #d8bb45;background:#faf9f3}.catalog-note strong{font-size:11px}.catalog-note p{margin:6px 0 0;color:var(--ns-ink-soft);font-size:10px;line-height:1.7}@media(max-width:760px){.section-heading{align-items:flex-start}.list-head{display:none}.package-row{grid-template-columns:1fr 1fr}.package-row :deep(.arco-switch){justify-self:start}.package-row :deep(.arco-btn){justify-self:end}}
</style>

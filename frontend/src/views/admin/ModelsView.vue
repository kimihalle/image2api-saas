<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconDelete, IconEdit, IconImage, IconPlayCircle, IconPlus, IconSearch,
  IconSettings, IconVideoCamera,
} from '@arco-design/web-vue/es/icon'
import GenerationTestModal from '../../components/GenerationTestModal.vue'
import { api } from '../../services/api'

type PriceRow = { key: string; price: number | undefined; agent: number | undefined }

const rows = ref<any[]>([])
const catalog = ref<any[]>([])
const loading = ref(false)
const saving = ref(false)
const query = ref('')
const provider = ref('all')
const activeTab = ref('managed')
const editorOpen = ref(false)
const editingId = ref('')
const selectedEntry = ref<any>(null)
const priceRows = ref<PriceRow[]>([])
const durationRows = ref<PriceRow[]>([])
const testingModel = ref<any>(null)
const form = reactive({ alias: '', weight: 0, enabled: true, free_allowed: false })
const managedPage = ref(1)
const catalogPage = ref(1)
const pageSize = 20

const providers = computed(() => [...new Set([...rows.value, ...catalog.value].map((item) => item.provider).filter(Boolean))])
const filteredRows = computed(() => rows.value.filter(matchesFilter))
const filteredCatalog = computed(() => catalog.value.filter(matchesFilter))
const pagedCatalog = computed(() => filteredCatalog.value.slice((catalogPage.value - 1) * pageSize, catalogPage.value * pageSize))
const managedPagination = computed(() => ({ current: managedPage.value, pageSize, total: filteredRows.value.length, showTotal: true }))
const isVideo = computed(() => selectedEntry.value?.type === 'video')

function matchesFilter(item: any) {
  const text = `${item.id || ''} ${item.alias || ''} ${item.name || ''} ${item.description || ''}`.toLowerCase()
  return (provider.value === 'all' || item.provider === provider.value) && (!query.value || text.includes(query.value.toLowerCase()))
}

function numberOrUndefined(value: any) {
  if (value === '' || value == null || !Number.isFinite(Number(value))) return undefined
  return Number(value)
}

function mapPrices(items: PriceRow[]) {
  return items.reduce<Record<string, number>>((result, item) => {
    const value = numberOrUndefined(item.price)
    if (value !== undefined) result[item.key] = value
    return result
  }, {})
}

function mapAgentPrices(items: PriceRow[]) {
  return items.reduce<Record<string, number>>((result, item) => {
    const value = numberOrUndefined(item.agent)
    if (value !== undefined) result[item.key] = value
    return result
  }, {})
}

async function load() {
  loading.value = true
  const [managedResponse, catalogResponse] = await Promise.all([api('/managed-models'), api('/catalog')])
  rows.value = managedResponse.ok ? (managedResponse.data?.data || managedResponse.data || []) : []
  catalog.value = catalogResponse.ok ? (catalogResponse.data?.data || catalogResponse.data || []) : []
  managedPage.value = Math.min(managedPage.value, Math.max(1, Math.ceil(filteredRows.value.length / pageSize)))
  catalogPage.value = Math.min(catalogPage.value, Math.max(1, Math.ceil(filteredCatalog.value.length / pageSize)))
  loading.value = false
}

function openEditor(entry: any, managed?: any) {
  selectedEntry.value = { ...entry, ...(managed || {}), id: managed?.id || entry.id }
  editingId.value = managed?.id || ''
  Object.assign(form, { alias: managed?.alias || '', weight: Number(managed?.weight || 0), enabled: managed?.enabled !== false, free_allowed: managed?.free_allowed === true })
  const resolutions = entry.resolutions?.length ? entry.resolutions : (managed?.resolutions || Object.keys(managed?.prices || {}))
  priceRows.value = resolutions.map((key: string) => ({
    key,
    price: numberOrUndefined(managed?.prices?.[key]),
    agent: numberOrUndefined(managed?.prices_agent?.[key]),
  }))
  const durationKeys = managed?.duration_prices?.per_second != null
    ? ['per_second']
    : (entry.durations?.length ? entry.durations : (managed?.durations || Object.keys(managed?.duration_prices || {})))
  durationRows.value = durationKeys.map((key: string) => ({
    key,
    price: numberOrUndefined(managed?.duration_prices?.[key]),
    agent: numberOrUndefined(managed?.duration_prices_agent?.[key]),
  }))
  editorOpen.value = true
}

function edit(row: any) {
  const entry = catalog.value.find((item) => item.id === row.id) || row
  openEditor(entry, row)
}

async function save() {
  const entry = selectedEntry.value
  const prices = mapPrices(priceRows.value)
  if (!Object.keys(prices).length) return Message.warning(isVideo.value ? '至少填写一个分辨率价格' : '至少填写一个画质价格')
  const durationPrices = mapPrices(durationRows.value)
  if (isVideo.value && !Object.keys(durationPrices).length) return Message.warning('至少填写一个时长价格')
  const payload: any = {
    id: entry.id,
    name: entry.name || entry.description || entry.id,
    alias: form.alias.trim(),
    type: entry.type,
    provider: entry.provider,
    enabled: form.enabled,
    free_allowed: form.free_allowed,
    ratios: entry.ratios || [],
    resolutions: entry.resolutions || [],
    prices,
    prices_agent: mapAgentPrices(priceRows.value),
    image_to_image: !!entry.image_to_image,
    max_reference_images: Number(entry.max_reference_images || 0),
    reference_mode: entry.reference_mode || 'none',
    weight: Number(form.weight || 0),
  }
  if (isVideo.value) {
    payload.durations = durationRows.value[0]?.key === 'per_second' ? (entry.durations || []) : durationRows.value.map((item) => item.key)
    payload.duration_prices = durationPrices
    payload.duration_prices_agent = mapAgentPrices(durationRows.value)
  }
  saving.value = true
  const response = await api(editingId.value ? `/managed-models/${encodeURIComponent(editingId.value)}` : '/managed-models', {
    method: editingId.value ? 'PATCH' : 'POST',
    body: JSON.stringify(payload),
  })
  saving.value = false
  if (!response.ok) return Message.error(response.data?.detail || '保存失败')
  editorOpen.value = false
  await load()
  Message.success(editingId.value ? '模型配置已更新' : '模型已加入服务目录')
}

async function toggle(row: any, value: boolean) {
  const response = await api(`/managed-models/${encodeURIComponent(row.id)}`, { method: 'PATCH', body: JSON.stringify({ enabled: value }) })
  if (!response.ok) return Message.error(response.data?.detail || '更新失败')
  row.enabled = value
  Message.success(value ? '模型已启用' : '模型已停用')
}

function remove(row: any) {
  Modal.warning({
    title: '删除模型',
    content: `确认从服务目录删除 ${row.alias || row.id}？删除后 API 将无法调用该模型。`,
    okText: '确认删除',
    hideCancel: false,
    async onOk() {
      const response = await api(`/managed-models/${encodeURIComponent(row.id)}`, { method: 'DELETE' })
      if (!response.ok) { Message.error(response.data?.detail || '删除失败'); return false }
      await load()
      Message.success('模型已删除')
      return true
    },
  })
}

function capabilityText(item: any) {
  const parts = [item.ratios?.join(' / '), item.resolutions?.join(' / ')]
  if (item.type === 'video') parts.push(item.durations?.join(' / '))
  if (Number(item.max_reference_images || 0) > 0) parts.push(`参考图 ${item.max_reference_images} 张`)
  return parts.filter(Boolean).join(' · ')
}

function priceText(item: any) {
  const base = Object.entries(item.prices || {}).map(([key, value]) => `${key} ${value}`).join(' / ')
  const duration = Object.entries(item.duration_prices || {}).map(([key, value]) => `${key} ${value}`).join(' / ')
  return [base, duration].filter(Boolean).join(' + ') || '未配置'
}

watch([query, provider], () => { managedPage.value = 1; catalogPage.value = 1 })
onMounted(load)

const columns: any[] = [
  { title: '模型', slotName: 'model', width: 280 },
  { title: 'Provider', dataIndex: 'provider', width: 110 },
  { title: '能力', slotName: 'capability', width: 330 },
  { title: '计费（积分）', slotName: 'price', width: 240 },
  { title: '状态', slotName: 'enabled', width: 90 },
  { title: '操作', slotName: 'actions', width: 150, fixed: 'right' },
]
</script>

<template>
  <div>
    <div class="section-heading"><div><h2>模型目录</h2><p>从系统内置模型库启用能力，并维护用户价格、代理价格与 API 别名。</p></div><a-button type="primary" @click="activeTab = 'catalog'"><IconPlus />添加内置模型</a-button></div>
    <div class="toolbar"><a-input v-model="query" placeholder="搜索模型名称或 ID" allow-clear><template #prefix><IconSearch /></template></a-input><a-select v-model="provider"><a-option value="all">全部 Provider</a-option><a-option v-for="item in providers" :key="item" :value="item">{{ item }}</a-option></a-select><span>{{ activeTab === 'managed' ? filteredRows.length : filteredCatalog.length }} 个模型</span></div>
    <a-tabs v-model:active-key="activeTab" class="model-tabs">
      <a-tab-pane key="managed" title="已上线模型">
        <a-table :columns="columns" :data="filteredRows" :loading="loading" :pagination="managedPagination" row-key="id" :scroll="{ x: 1200 }" @page-change="managedPage = $event">
          <template #empty><a-empty description="尚未上线模型，请从内置模型库添加" /></template>
          <template #model="{ record }"><div class="model-cell"><span :class="record.type"><IconVideoCamera v-if="record.type === 'video'" /><IconImage v-else /></span><div><strong>{{ record.alias || record.name || record.id }}</strong><code>{{ record.id }}</code></div></div></template>
          <template #capability="{ record }"><span class="capability">{{ capabilityText(record) }}</span></template>
          <template #price="{ record }"><span class="price">{{ priceText(record) }}</span></template>
          <template #enabled="{ record }"><a-switch :model-value="record.enabled" @change="toggle(record, $event as boolean)" /></template>
          <template #actions="{ record }"><div class="row-actions"><a-tooltip content="调用测试"><a-button type="text" aria-label="调用测试" @click="testingModel = record"><IconPlayCircle /></a-button></a-tooltip><a-tooltip content="编辑配置"><a-button type="text" aria-label="编辑配置" @click="edit(record)"><IconEdit /></a-button></a-tooltip><a-tooltip content="删除模型"><a-button type="text" status="danger" aria-label="删除模型" @click="remove(record)"><IconDelete /></a-button></a-tooltip></div></template>
        </a-table>
      </a-tab-pane>
      <a-tab-pane key="catalog" title="内置模型库">
        <div class="catalog-list"><article v-for="item in pagedCatalog" :key="item.id" class="catalog-row"><span class="catalog-icon" :class="item.type"><IconVideoCamera v-if="item.type === 'video'" /><IconImage v-else /></span><div class="catalog-main"><div><strong>{{ item.description || item.id }}</strong><code>{{ item.id }}</code></div><p>{{ capabilityText(item) }}</p></div><div class="catalog-provider"><span>{{ item.provider }}</span><small>{{ item.type === 'video' ? '视频生成' : item.image_to_image ? '文生图 / 图生图' : '图像生成' }}</small></div><a-tag v-if="item.added" color="green">已上线</a-tag><a-button v-else type="outline" @click="openEditor(item)"><IconSettings />配置上线</a-button></article></div>
        <a-empty v-if="!loading && !filteredCatalog.length" description="没有符合条件的内置模型" />
        <a-pagination v-if="filteredCatalog.length > pageSize" v-model:current="catalogPage" :total="filteredCatalog.length" :page-size="pageSize" show-total />
      </a-tab-pane>
    </a-tabs>

    <a-modal v-model:visible="editorOpen" :width="720" :title="editingId ? '编辑模型配置' : '配置模型上线'" :ok-loading="saving" ok-text="保存配置" @ok="save">
      <div v-if="selectedEntry" class="editor-summary"><span :class="selectedEntry.type"><IconVideoCamera v-if="isVideo" /><IconImage v-else /></span><div><strong>{{ selectedEntry.description || selectedEntry.name || selectedEntry.id }}</strong><code>{{ selectedEntry.id }} · {{ selectedEntry.provider }}</code></div></div>
      <a-form :model="form" layout="vertical">
        <div class="form-grid"><a-form-item label="API 别名"><a-input v-model="form.alias" placeholder="留空时使用模型 ID" /><span class="field-note">对外公开的模型 ID；修改后原始 ID 仍可兼容调用</span></a-form-item><a-form-item label="展示权重"><a-input-number v-model="form.weight" :min="0" :precision="0" /></a-form-item></div>
        <div class="capability-panel"><div><span>画面比例</span><p>{{ selectedEntry?.ratios?.join(' / ') || '未声明' }}</p></div><div><span>分辨率</span><p>{{ selectedEntry?.resolutions?.join(' / ') || '未声明' }}</p></div><div v-if="isVideo"><span>时长</span><p>{{ selectedEntry?.durations?.join(' / ') || '未声明' }}</p></div><div><span>参考图</span><p>{{ Number(selectedEntry?.max_reference_images || 0) ? `最多 ${selectedEntry.max_reference_images} 张` : '不支持' }}</p></div></div>
        <a-form-item :label="isVideo ? '分辨率价格（积分）' : '画质价格（积分）'"><div class="price-editor"><div class="price-head"><span>规格</span><span>用户价格</span><span>代理价格（可选）</span></div><div v-for="item in priceRows" :key="item.key" class="price-line"><strong>{{ item.key }}</strong><a-input-number v-model="item.price" :min="0" placeholder="必填" /><a-input-number v-model="item.agent" :min="0" placeholder="跟随用户价格" /></div></div></a-form-item>
        <a-form-item v-if="isVideo" label="时长价格（积分）"><div class="price-editor"><div class="price-head"><span>时长</span><span>用户价格</span><span>代理价格（可选）</span></div><div v-for="item in durationRows" :key="item.key" class="price-line"><strong>{{ item.key === 'per_second' ? '每秒' : item.key }}</strong><a-input-number v-model="item.price" :min="0" placeholder="必填" /><a-input-number v-model="item.agent" :min="0" placeholder="跟随用户价格" /></div></div><p class="formula">视频实付为分辨率价格加时长价格；按秒模型按实际秒数计算。</p></a-form-item>
        <div class="form-grid model-access-grid"><a-form-item label="启用状态"><a-switch v-model="form.enabled" /><span class="switch-note">关闭后用户端和 OpenAI 兼容接口均不可调用</span></a-form-item><a-form-item label="允许普号调度"><a-switch v-model="form.free_allowed" /><span class="switch-note">允许有剩余额度的 free Provider 账号参与正式生成</span></a-form-item></div>
      </a-form>
    </a-modal>
    <GenerationTestModal v-if="testingModel" :model="testingModel" @close="testingModel = null" />
  </div>
</template>

<style scoped>
.toolbar{display:grid;grid-template-columns:minmax(220px,360px) 170px 1fr;align-items:center;gap:10px;margin-bottom:8px}.toolbar>span{text-align:right;color:var(--ns-ink-faint);font-size:11px}.model-tabs :deep(.arco-tabs-nav-tab){justify-content:flex-start}.model-cell,.editor-summary{display:flex;align-items:center;gap:11px}.model-cell>span,.editor-summary>span,.catalog-icon{width:34px;height:34px;flex:0 0 34px;display:grid;place-items:center;border-radius:6px;background:#e9eee5;color:#566452}.model-cell>span.video,.editor-summary>span.video,.catalog-icon.video{background:#eeeae0;color:#746542}.model-cell>div,.editor-summary>div,.catalog-main>div{display:flex;flex-direction:column;min-width:0}.model-cell strong,.editor-summary strong,.catalog-main strong{font-size:12px}.model-cell code,.editor-summary code,.catalog-main code{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.capability,.price{font-size:10px;line-height:1.7;color:var(--ns-ink-soft)}.row-actions{display:flex;gap:2px}.catalog-list{border-top:1px solid var(--ns-line)}.catalog-row{display:grid;grid-template-columns:38px minmax(260px,1fr) 150px auto;align-items:center;gap:14px;padding:15px 8px;border-bottom:1px solid var(--ns-line)}.catalog-main p{margin:6px 0 0;color:var(--ns-ink-soft);font-size:10px}.catalog-provider{display:flex;flex-direction:column}.catalog-provider span{font-size:11px;font-weight:650}.catalog-provider small{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.editor-summary{margin-bottom:18px;padding:12px;border:1px solid var(--ns-line);border-radius:7px;background:#fafaf7}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.capability-panel{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:1px;margin-bottom:18px;background:var(--ns-line);border:1px solid var(--ns-line);border-radius:7px;overflow:hidden}.capability-panel>div{min-height:70px;padding:12px;background:#fafaf7}.capability-panel span{font-size:9px;color:var(--ns-ink-faint)}.capability-panel p{margin:6px 0 0;font-size:10px;line-height:1.5}.price-editor{width:100%;border:1px solid var(--ns-line);border-radius:7px;overflow:hidden}.price-head,.price-line{display:grid;grid-template-columns:100px 1fr 1fr;gap:10px;align-items:center;padding:8px 10px}.price-head{background:#f5f5f1;color:var(--ns-ink-faint);font-size:9px}.price-line+.price-line{border-top:1px solid var(--ns-line)}.price-line strong{font-size:11px}.formula,.switch-note{margin:7px 0 0;color:var(--ns-ink-faint);font-size:10px}.switch-note{margin-left:10px}@media(max-width:760px){.toolbar{grid-template-columns:1fr 140px}.toolbar>span{display:none}.catalog-row{grid-template-columns:38px 1fr auto}.catalog-provider{display:none}.capability-panel{grid-template-columns:1fr 1fr}.form-grid{grid-template-columns:1fr}}@media(max-width:520px){.catalog-row{grid-template-columns:34px 1fr}.catalog-row>.arco-btn,.catalog-row>.arco-tag{grid-column:2;justify-self:start}.price-head,.price-line{grid-template-columns:64px 1fr 1fr;gap:6px;padding-inline:7px}}
.field-note{display:block;margin-top:6px;color:var(--ns-ink-faint);font-size:9px;line-height:1.5}
</style>

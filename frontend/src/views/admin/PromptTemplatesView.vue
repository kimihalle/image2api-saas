<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconCloudDownload, IconDelete, IconEdit, IconEye, IconPlus, IconSearch,
  IconSettings, IconUpload,
} from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../../services/api'

const tab = ref('templates')
const rows = ref<any[]>([])
const categories = ref<any[]>([])
const batches = ref<any[]>([])
const sources = ref<any[]>([])
const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = 20
const query = ref('')
const typeFilter = ref('all')
const statusFilter = ref('all')
const categoryFilter = ref('all')
const editorOpen = ref(false)
const categoryOpen = ref(false)
const editingID = ref('')
const editingCategoryID = ref('')
const syncForm = reactive({ source: 'youmind', limit: 100 })
let searchTimer: number | undefined

const emptyTemplate = () => ({
  title: '', description: '', category_id: '', media_type: 'image', prompt: '', cover: '', status: 'draft', featured: false, weight: 0,
  reference_mode: 'none', min_references: 0, max_references: 0,
  tags_text: '', ratios_text: '1:1,4:3,3:4,16:9,9:16', resolutions_text: '1K,2K,4K', durations_text: '', models_text: '',
  variables: [] as any[],
})
const form = reactive(emptyTemplate())
const categoryForm = reactive({ name: '', description: '', icon: 'Sparkles', cover: '', weight: 0, enabled: true })

function split(value: string) { return [...new Set(String(value || '').split(/[,，\n]/).map((item) => item.trim()).filter(Boolean))] }
function list(value: unknown) { return Array.isArray(value) ? value.map(String) : [] }
function categoryName(id: string) { return categories.value.find((item) => item.id === id)?.name || '-' }
function coverFor(item: any) {
  if (item.cover) return imageUrl(item.cover)
  const source = String(item.id || item.title || '')
  const hash = [...source].reduce((sum, char) => sum + char.charCodeAt(0), 0)
  return imageUrl(`/inspiration/${String((hash % 30) + 1).padStart(2, '0')}.jpg`)
}
function statusText(value: string) { return value === 'published' ? '已上架' : value === 'disabled' ? '已停用' : '草稿' }
function statusColor(value: string) { return value === 'published' ? 'green' : value === 'disabled' ? 'red' : 'gray' }
function formatTime(value: any) { const date = new Date(value); return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false }) }

async function loadTemplates() {
  loading.value = true
  const params = new URLSearchParams({ limit: String(pageSize), offset: String((page.value - 1) * pageSize), status: statusFilter.value })
  if (query.value.trim()) params.set('q', query.value.trim())
  if (typeFilter.value !== 'all') params.set('media_type', typeFilter.value)
  if (categoryFilter.value !== 'all') params.set('category', categoryFilter.value)
  const response = await api(`/prompt-admin/templates?${params}`)
  loading.value = false
  if (!response.ok) return Message.error(response.data?.detail || '模板加载失败')
  rows.value = response.data?.data || []
  total.value = Number(response.data?.total || 0)
}
async function loadCategories() {
  const response = await api('/prompt-admin/categories')
  if (response.ok) categories.value = response.data?.data || []
}
async function loadImports() {
  const [batchResponse, sourceResponse] = await Promise.all([api('/prompt-admin/imports'), api('/prompt-admin/sources')])
  if (batchResponse.ok) batches.value = batchResponse.data?.data || []
  if (sourceResponse.ok) sources.value = sourceResponse.data?.data || []
}
async function loadAll() { await Promise.all([loadCategories(), loadImports()]); await loadTemplates() }
function filterChanged() { page.value = 1; loadTemplates() }
watch(query, () => { if (searchTimer) window.clearTimeout(searchTimer); searchTimer = window.setTimeout(filterChanged, 300) })

function openTemplate(item?: any) {
  editingID.value = item?.id || ''
  Object.assign(form, emptyTemplate())
  if (item) Object.assign(form, {
    ...item,
    variables: Array.isArray(item.variables) ? item.variables.map((variable: any) => ({ ...variable, options_text: list(variable.options).join(',') })) : [],
    tags_text: list(item.tags).join(','), ratios_text: list(item.ratios).join(','), resolutions_text: list(item.resolutions).join(','),
    durations_text: list(item.durations).join(','), models_text: list(item.compatible_models).join(','),
  })
  if (!form.category_id) form.category_id = categories.value[0]?.id || ''
  editorOpen.value = true
}
function addVariable() { form.variables.push({ name: '', label: '', type: 'text', default: '', placeholder: '', required: false, options_text: '' }) }
function removeVariable(index: number) { form.variables.splice(index, 1) }
async function saveTemplate() {
  if (!form.title.trim() || !form.prompt.trim() || !form.category_id) return Message.warning('标题、分类和提示词不能为空')
  const body = {
    title: form.title, description: form.description, category_id: form.category_id, media_type: form.media_type, prompt: form.prompt,
    cover: form.cover, status: form.status, featured: form.featured, weight: Number(form.weight || 0), reference_mode: form.reference_mode,
    min_references: Number(form.min_references || 0), max_references: Number(form.max_references || 0), tags: split(form.tags_text), ratios: split(form.ratios_text),
    resolutions: split(form.resolutions_text), durations: split(form.durations_text), compatible_models: split(form.models_text),
    variables: form.variables.map((item: any) => ({ name: item.name.trim(), label: item.label.trim() || item.name.trim(), type: item.type || 'text', default: item.default || '', placeholder: item.placeholder || '', required: !!item.required, options: split(item.options_text) })),
  }
  saving.value = true
  const response = await api(editingID.value ? `/prompt-admin/templates/${editingID.value}` : '/prompt-admin/templates', { method: editingID.value ? 'PUT' : 'POST', body: JSON.stringify(body) })
  saving.value = false
  if (!response.ok) return Message.error(response.data?.detail || '模板保存失败')
  editorOpen.value = false
  await Promise.all([loadTemplates(), loadCategories()])
  Message.success(editingID.value ? '模板已更新' : '模板已创建')
}
async function quickStatus(item: any) {
  const body = {
    ...item, status: item.status === 'published' ? 'disabled' : 'published', variables: item.variables || [], tags: item.tags || [], ratios: item.ratios || [], resolutions: item.resolutions || [], durations: item.durations || [], compatible_models: item.compatible_models || [],
  }
  const response = await api(`/prompt-admin/templates/${item.id}`, { method: 'PUT', body: JSON.stringify(body) })
  if (!response.ok) return Message.error(response.data?.detail || '状态更新失败')
  await Promise.all([loadTemplates(), loadCategories()])
  Message.success(body.status === 'published' ? '模板已上架' : '模板已停用')
}
function deleteTemplate(item: any) {
  Modal.warning({ title: '删除模板', content: `确认永久删除“${item.title}”？收藏和使用记录也会一并移除。`, okText: '确认删除', hideCancel: false, async onOk() {
    const response = await api(`/prompt-admin/templates/${item.id}`, { method: 'DELETE' })
    if (!response.ok) { Message.error(response.data?.detail || '删除失败'); return false }
    await Promise.all([loadTemplates(), loadCategories()]); Message.success('模板已删除'); return true
  } })
}

function openCategory(item?: any) {
  editingCategoryID.value = item?.id || ''
  Object.assign(categoryForm, { name: item?.name || '', description: item?.description || '', icon: item?.icon || 'Sparkles', cover: item?.cover || '', weight: Number(item?.weight || 0), enabled: item ? item.enabled !== false : true })
  categoryOpen.value = true
}
async function saveCategory() {
  if (!categoryForm.name.trim()) return Message.warning('请填写分类名称')
  saving.value = true
  const response = await api(editingCategoryID.value ? `/prompt-admin/categories/${editingCategoryID.value}` : '/prompt-admin/categories', { method: editingCategoryID.value ? 'PUT' : 'POST', body: JSON.stringify(categoryForm) })
  saving.value = false
  if (!response.ok) return Message.error(response.data?.detail || '分类保存失败')
  categoryOpen.value = false
  await loadCategories(); Message.success(editingCategoryID.value ? '分类已更新' : '分类已创建')
}
function deleteCategory(item: any) {
  Modal.warning({ title: '删除分类', content: `确认删除“${item.name}”？仅空分类可以删除。`, okText: '确认删除', hideCancel: false, async onOk() {
    const response = await api(`/prompt-admin/categories/${item.id}`, { method: 'DELETE' })
    if (!response.ok) { Message.error(response.data?.detail || '删除失败'); return false }
    await loadCategories(); Message.success('分类已删除'); return true
  } })
}
async function syncSource() {
  syncing.value = true
  const response = await api('/prompt-admin/sync', { method: 'POST', body: JSON.stringify(syncForm) })
  syncing.value = false
  await loadImports()
  if (!response.ok) return Message.error(response.data?.detail || '同步失败')
  await Promise.all([loadTemplates(), loadCategories()])
  Message.success(`同步完成：新增 ${response.data?.inserted || 0}，更新 ${response.data?.updated || 0}，跳过 ${response.data?.skipped || 0}`)
}

onMounted(loadAll)
const columns: any[] = [
  { title: '模板', slotName: 'template', width: 310 }, { title: '类型', slotName: 'type', width: 90 }, { title: '分类', slotName: 'category', width: 120 },
  { title: '状态', slotName: 'status', width: 100 }, { title: '使用 / 收藏', slotName: 'metrics', width: 130 }, { title: '排序', dataIndex: 'weight', width: 80 },
  { title: '更新时间', slotName: 'updated', width: 170 }, { title: '操作', slotName: 'actions', width: 150, fixed: 'right' },
]
</script>

<template>
  <div class="prompt-admin-page">
    <div class="section-heading"><div><span class="eyebrow">CONTENT OPERATIONS</span><h2>灵感模板</h2><p>维护前台灵感库、模板变量和外部内容审核流程。</p></div><a-button v-if="tab === 'templates'" type="primary" @click="openTemplate()"><IconPlus />新建模板</a-button><a-button v-else-if="tab === 'categories'" type="primary" @click="openCategory()"><IconPlus />新建分类</a-button></div>
    <a-tabs v-model:active-key="tab" class="admin-tabs"><a-tab-pane key="templates" title="模板管理" /><a-tab-pane key="categories" title="分类管理" /><a-tab-pane key="imports" title="内容同步" /></a-tabs>

    <section v-if="tab === 'templates'">
      <div class="toolbar"><a-input v-model="query" placeholder="搜索标题、描述或提示词" allow-clear><template #prefix><IconSearch /></template></a-input><a-select v-model="typeFilter" @change="filterChanged"><a-option value="all">全部类型</a-option><a-option value="image">图片</a-option><a-option value="video">视频</a-option></a-select><a-select v-model="categoryFilter" @change="filterChanged"><a-option value="all">全部分类</a-option><a-option v-for="item in categories" :key="item.id" :value="item.id">{{ item.name }}</a-option></a-select><a-select v-model="statusFilter" @change="filterChanged"><a-option value="all">全部状态</a-option><a-option value="published">已上架</a-option><a-option value="draft">草稿</a-option><a-option value="disabled">已停用</a-option></a-select><span>共 {{ total }} 条</span></div>
      <a-table :columns="columns" :data="rows" :loading="loading" :pagination="false" row-key="id" :scroll="{ x: 1170 }">
        <template #empty><a-empty description="暂无模板" /></template>
        <template #template="{ record }"><div class="template-cell"><span class="table-thumb"><img :src="coverFor(record)" alt="" /></span><div><strong>{{ record.title }}</strong><small>{{ record.description || '未填写模板说明' }}</small></div></div></template>
        <template #type="{ record }"><a-tag>{{ record.media_type === 'video' ? '视频' : '图片' }}</a-tag></template>
        <template #category="{ record }"><span class="muted">{{ categoryName(record.category_id) }}</span></template>
        <template #status="{ record }"><a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag></template>
        <template #metrics="{ record }"><span class="metric">{{ record.use_count }} / {{ record.favorite_count }}</span></template>
        <template #updated="{ record }"><span class="muted">{{ formatTime(record.updated_at) }}</span></template>
        <template #actions="{ record }"><div class="action-row"><a-tooltip :content="record.status === 'published' ? '停用' : '上架'"><a-button type="text" :status="record.status === 'published' ? 'warning' : 'success'" :aria-label="record.status === 'published' ? '停用' : '上架'" @click="quickStatus(record)"><IconEye /></a-button></a-tooltip><a-tooltip content="编辑"><a-button type="text" aria-label="编辑" @click="openTemplate(record)"><IconEdit /></a-button></a-tooltip><a-tooltip content="删除"><a-button type="text" status="danger" aria-label="删除" @click="deleteTemplate(record)"><IconDelete /></a-button></a-tooltip></div></template>
      </a-table>
      <a-pagination v-if="total > pageSize" v-model:current="page" :total="total" :page-size="pageSize" show-total @change="loadTemplates" />
    </section>

    <section v-else-if="tab === 'categories'" class="category-table">
      <div class="category-head"><span>分类</span><span>说明</span><span>模板数</span><span>排序</span><span>前台显示</span><span>操作</span></div>
      <div v-for="item in categories" :key="item.id" class="category-row"><div><span class="category-icon"><IconSettings /></span><strong>{{ item.name }}</strong></div><span>{{ item.description || '-' }}</span><strong>{{ item.template_count }}</strong><span>{{ item.weight }}</span><a-tag :color="item.enabled ? 'green' : 'gray'">{{ item.enabled ? '显示' : '隐藏' }}</a-tag><div class="action-row"><a-button type="text" aria-label="编辑分类" @click="openCategory(item)"><IconEdit /></a-button><a-button type="text" status="danger" aria-label="删除分类" @click="deleteCategory(item)"><IconDelete /></a-button></div></div>
    </section>

    <section v-else class="import-layout">
      <div class="sync-panel"><span class="sync-mark"><IconCloudDownload /></span><div><h3>从预设 GitHub 内容源同步</h3><p>同步内容只进入草稿，不会自动发布。系统按来源 ID 和内容摘要去重，外部示例图片不会直接用于前台商业展示。</p></div><div class="sync-fields"><a-select v-model="syncForm.source"><a-option v-for="item in sources" :key="item.id" :value="item.id">{{ item.name }}</a-option></a-select><a-input-number v-model="syncForm.limit" :min="10" :max="500" :step="10" /><a-button type="primary" :loading="syncing" @click="syncSource"><IconCloudDownload />开始同步</a-button></div></div>
      <div class="history-title"><strong>同步记录</strong><span>最近 20 次</span></div>
      <div class="import-table"><div class="import-head"><span>来源</span><span>状态</span><span>请求 / 获取</span><span>新增 / 更新 / 跳过</span><span>开始时间</span><span>结果</span></div><div v-for="item in batches" :key="item.id" class="import-row"><strong>{{ item.source }}</strong><a-tag :color="item.status === 'completed' ? 'green' : item.status === 'failed' ? 'red' : 'orange'">{{ item.status === 'completed' ? '完成' : item.status === 'failed' ? '失败' : '同步中' }}</a-tag><span>{{ item.requested }} / {{ item.fetched }}</span><span>{{ item.inserted }} / {{ item.updated }} / {{ item.skipped }}</span><span>{{ formatTime(item.started_at) }}</span><span :title="item.error">{{ item.error || '正常' }}</span></div><a-empty v-if="!batches.length" description="暂无同步记录" /></div>
    </section>

    <a-drawer v-model:visible="editorOpen" :width="720" :title="editingID ? '编辑模板' : '新建模板'" :footer="false" unmount-on-close>
      <a-form :model="form" layout="vertical" class="template-form"><div class="form-grid two"><a-form-item label="模板标题" required><a-input v-model="form.title" /></a-form-item><a-form-item label="所属分类" required><a-select v-model="form.category_id"><a-option v-for="item in categories" :key="item.id" :value="item.id">{{ item.name }}</a-option></a-select></a-form-item><a-form-item label="模板类型"><a-radio-group v-model="form.media_type" type="button"><a-radio value="image">图片</a-radio><a-radio value="video">视频</a-radio></a-radio-group></a-form-item><a-form-item label="状态"><a-select v-model="form.status"><a-option value="draft">草稿</a-option><a-option value="published">已上架</a-option><a-option value="disabled">已停用</a-option></a-select></a-form-item></div><a-form-item label="模板说明"><a-textarea v-model="form.description" :auto-size="{ minRows: 2, maxRows: 4 }" /></a-form-item><a-form-item label="提示词" required extra="变量使用 {{variable_name}} 格式，并在下方建立同名变量。"><a-textarea v-model="form.prompt" :auto-size="{ minRows: 7, maxRows: 14 }" /></a-form-item>
        <div class="field-title"><div><strong>模板变量</strong><span>用户使用模板时填写的内容</span></div><a-button size="small" @click="addVariable"><IconPlus />添加变量</a-button></div><div v-for="(variable, index) in form.variables" :key="index" class="variable-row"><a-input v-model="variable.name" placeholder="变量名" /><a-input v-model="variable.label" placeholder="显示名称" /><a-select v-model="variable.type"><a-option value="text">文本</a-option><a-option value="select">选项</a-option></a-select><a-input v-model="variable.default" placeholder="默认值" /><a-input v-if="variable.type === 'select'" v-model="variable.options_text" placeholder="选项，逗号分隔" /><a-input v-else v-model="variable.placeholder" placeholder="输入提示" /><a-checkbox v-model="variable.required">必填</a-checkbox><a-button type="text" status="danger" aria-label="删除变量" @click="removeVariable(index)"><IconDelete /></a-button></div>
        <div class="form-grid two section-fields"><a-form-item label="标签（逗号分隔）"><a-input v-model="form.tags_text" /></a-form-item><a-form-item label="封面地址"><a-input v-model="form.cover" placeholder="/showcase/example.jpg" /></a-form-item><a-form-item label="支持比例"><a-input v-model="form.ratios_text" /></a-form-item><a-form-item label="支持分辨率"><a-input v-model="form.resolutions_text" /></a-form-item><a-form-item v-if="form.media_type === 'video'" label="支持时长"><a-input v-model="form.durations_text" placeholder="5,10" /></a-form-item><a-form-item label="限定模型 ID"><a-input v-model="form.models_text" placeholder="留空表示不限定" /></a-form-item><a-form-item label="参考素材"><a-select v-model="form.reference_mode"><a-option value="none">不需要</a-option><a-option value="optional">可选</a-option><a-option value="required">必须上传</a-option></a-select></a-form-item><a-form-item label="最大参考图数"><a-input-number v-model="form.max_references" :min="0" :max="10" /></a-form-item><a-form-item label="排序权重"><a-input-number v-model="form.weight" :min="0" /></a-form-item><a-form-item label="精选推荐"><a-switch v-model="form.featured" /></a-form-item></div>
        <div class="drawer-actions"><a-button @click="editorOpen = false">取消</a-button><a-button type="primary" :loading="saving" @click="saveTemplate">保存模板</a-button></div>
      </a-form>
    </a-drawer>

    <a-modal v-model:visible="categoryOpen" :width="560" :title="editingCategoryID ? '编辑分类' : '新建分类'" :footer="false"><a-form :model="categoryForm" layout="vertical"><a-form-item label="分类名称" required><a-input v-model="categoryForm.name" /></a-form-item><a-form-item label="分类说明"><a-input v-model="categoryForm.description" /></a-form-item><div class="form-grid two"><a-form-item label="图标名称"><a-input v-model="categoryForm.icon" /></a-form-item><a-form-item label="排序权重"><a-input-number v-model="categoryForm.weight" :min="0" /></a-form-item><a-form-item label="前台显示"><a-switch v-model="categoryForm.enabled" /></a-form-item></div><div class="drawer-actions"><a-button @click="categoryOpen = false">取消</a-button><a-button type="primary" :loading="saving" @click="saveCategory">保存分类</a-button></div></a-form></a-modal>
  </div>
</template>

<style scoped>
.prompt-admin-page{max-width:1380px}.eyebrow{display:block;margin-bottom:6px;color:#927923;font-size:9px;font-weight:750;letter-spacing:.13em}.admin-tabs{margin-bottom:16px}.toolbar{display:grid;grid-template-columns:minmax(240px,1fr) 120px 150px 130px auto;align-items:center;gap:9px;margin-bottom:14px}.toolbar>span{text-align:right;color:var(--ns-ink-faint);font-size:10px}.template-cell{display:grid;grid-template-columns:46px minmax(0,1fr);align-items:center;gap:10px}.table-thumb{width:46px;height:38px;display:grid;place-items:center;overflow:hidden;border-radius:5px;background:#e9ebe6;color:#697169}.table-thumb img{width:100%;height:100%;object-fit:cover}.template-cell>div{min-width:0;display:flex;flex-direction:column}.template-cell strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:11px}.template-cell small{margin-top:4px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--ns-ink-faint);font-size:9px}.muted,.metric{color:var(--ns-ink-soft);font-size:10px}.metric{font-variant-numeric:tabular-nums}.action-row{display:flex;align-items:center;flex-wrap:nowrap}.pagination{display:flex;justify-content:flex-end;gap:6px;margin-top:16px}.pagination button{width:32px;height:32px;border:1px solid var(--ns-line);border-radius:6px;background:#fff;cursor:pointer}.pagination button.active{border-color:#29302a;background:#29302a;color:#fff}.category-table,.import-table{border-top:1px solid var(--ns-line);border-bottom:1px solid var(--ns-line);background:#fff}.category-head,.category-row{display:grid;grid-template-columns:1.2fr 2fr 100px 90px 100px 90px;align-items:center;gap:12px;padding:13px 16px}.category-head,.import-head{background:#f5f6f2;color:var(--ns-ink-faint);font-size:9px}.category-row{border-top:1px solid var(--ns-line);font-size:10px}.category-row>div:first-child{display:flex;align-items:center;gap:9px}.category-icon{width:30px;height:30px;display:grid;place-items:center;border-radius:50%;background:#e6e9e2;color:#5c6759}.sync-panel{display:grid;grid-template-columns:54px minmax(280px,1fr) minmax(300px,.9fr);align-items:center;gap:18px;padding:24px 26px;border:1px solid #dcd8bc;border-radius:8px;background:#faf9f2}.sync-mark{width:48px;height:48px;display:grid;place-items:center;border-radius:50%;background:#e7dfb5;color:#6f6128}.sync-panel h3{margin:0;font-size:14px}.sync-panel p{margin:7px 0 0;color:var(--ns-ink-soft);font-size:10px;line-height:1.7}.sync-fields{display:grid;grid-template-columns:1fr 100px auto;gap:8px}.history-title{display:flex;justify-content:space-between;margin:26px 0 10px}.history-title strong{font-size:12px}.history-title span{color:var(--ns-ink-faint);font-size:9px}.import-head,.import-row{display:grid;grid-template-columns:1.2fr 100px 120px 150px 170px 1fr;align-items:center;gap:12px;padding:13px 16px}.import-row{border-top:1px solid var(--ns-line);font-size:10px}.import-row>span:last-child{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--ns-ink-soft)}.form-grid.two{display:grid;grid-template-columns:1fr 1fr;gap:0 12px}.field-title{display:flex;align-items:center;justify-content:space-between;margin:6px 0 10px;padding-top:14px;border-top:1px solid var(--ns-line)}.field-title>div{display:flex;flex-direction:column}.field-title strong{font-size:11px}.field-title span{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.variable-row{display:grid;grid-template-columns:1fr 1fr 90px 1fr 1.3fr 65px 34px;align-items:center;gap:7px;margin-bottom:8px;padding:9px;border:1px solid var(--ns-line);border-radius:6px;background:#fafaf7}.section-fields{margin-top:20px;padding-top:18px;border-top:1px solid var(--ns-line)}.drawer-actions{display:flex;justify-content:flex-end;gap:8px;padding-top:10px}.template-form :deep(.arco-input-number){width:100%}
@media(max-width:1000px){.toolbar{grid-template-columns:1fr 1fr 1fr}.toolbar>.arco-input-wrapper{grid-column:1/-1}.toolbar>span{grid-column:1/-1}.sync-panel{grid-template-columns:48px 1fr}.sync-fields{grid-column:1/-1}.category-head{display:none}.category-row{grid-template-columns:1fr 1.5fr 70px 70px 90px 80px}}@media(max-width:680px){.form-grid.two{grid-template-columns:1fr}.variable-row{grid-template-columns:1fr 1fr}.variable-row>.arco-btn{justify-self:end}.sync-panel{grid-template-columns:1fr;padding:18px}.sync-fields{grid-template-columns:1fr}.category-row{grid-template-columns:1fr 1fr}.category-row>span:nth-of-type(1){grid-column:1/-1}.import-head{display:none}.import-row{grid-template-columns:1fr 1fr}.toolbar{grid-template-columns:1fr 1fr}}
</style>

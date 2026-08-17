<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCheck, IconDelete, IconEdit, IconEye, IconImage, IconPlus } from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../../services/api'

const items = ref<any[]>([])
const generated = ref<any[]>([])
const loading = ref(false)
const pickerLoading = ref(false)
const filter = ref('all')
const visible = ref(false)
const pickerOpen = ref(false)
const saving = ref(false)
const form = reactive({ id: '', kind: 'hero', title: '', subtitle: '', prompt: '', image: '', weight: 100, span: '' })
const kinds = [{ value: 'hero', label: 'Hero' }, { value: 'bento', label: '灵感' }, { value: 'work', label: '作品' }]

function source(value: string) { return imageUrl(value) }
function generatedSource(name: string, thumbnail = false) { return imageUrl(`/images/${String(name).replace(/^\/+/, '')}${thumbnail ? '.thumb.jpg' : ''}`) }
async function load() {
  loading.value = true
  const query = filter.value === 'all' ? '' : `&kind=${filter.value}`
  const response = await api(`/showcase/admin?limit=100&offset=0${query}`)
  items.value = response.data?.data || []
  loading.value = false
}
async function loadGenerated() {
  pickerLoading.value = true
  const response = await api('/images?kind=image&limit=200&offset=0')
  generated.value = response.ok ? (response.data?.data || []) : []
  pickerLoading.value = false
}
function openCreate(kind = 'hero') {
  Object.assign(form, { id: '', kind, title: '', subtitle: '', prompt: '', image: '', weight: kind === 'hero' ? 300 : 100, span: '' })
  visible.value = true
}
function openEdit(item: any) {
  Object.assign(form, { id: item.id, kind: item.kind, title: item.title || '', subtitle: item.subtitle || '', prompt: item.prompt || '', image: item.image || '', weight: item.weight || 0, span: item.span || '' })
  visible.value = true
}
async function openGeneratedPicker() {
  pickerOpen.value = true
  await loadGenerated()
}
function chooseGenerated(item: any) {
  form.image = `/images/${String(item.name).replace(/^\/+/, '')}`
  pickerOpen.value = false
}
async function save() {
  if (!form.image.trim()) return Message.warning('请先选择一张已生成图片')
  if (form.kind !== 'work' && !form.title.trim()) return Message.warning('请填写标题')
  saving.value = true
  const payload = { kind: form.kind, title: form.title.trim(), subtitle: form.subtitle.trim(), prompt: form.prompt.trim(), image: form.image.trim(), weight: Number(form.weight) || 0, span: form.span }
  const response = form.id
    ? await api(`/showcase/${form.id}`, { method: 'PATCH', body: JSON.stringify(payload) })
    : await api('/showcase', { method: 'POST', body: JSON.stringify(payload) })
  saving.value = false
  if (!response.ok) return Message.error(response.data?.detail || '保存失败')
  visible.value = false
  Message.success('首页内容已保存')
  load()
}
async function remove(item: any) {
  const response = await api(`/showcase/${item.id}`, { method: 'DELETE' })
  if (!response.ok) return Message.error(response.data?.detail || '删除失败')
  Message.success('展示项已删除')
  load()
}
function setFilter(value: string) { filter.value = value; load() }
onMounted(load)
</script>

<template>
  <div class="showcase-admin">
    <div class="section-heading"><div><h2>首页内容</h2><p>管理公共首页的 Hero、灵感和精选作品。</p></div><a-button type="primary" shape="round" @click="openCreate()"><IconPlus />新增内容</a-button></div>
    <div class="toolbar"><div class="filter-pills"><button v-for="item in [{value:'all',label:'全部'},...kinds]" :key="item.value" :class="{active:filter===item.value}" @click="setFilter(item.value)">{{ item.label }}</button></div><a-link href="/" target="_blank"><IconEye />预览公共首页</a-link></div>
    <a-spin :loading="loading" style="width:100%"><div v-if="items.length" class="showcase-grid"><article v-for="item in items" :key="item.id" class="showcase-card"><div class="showcase-image"><img :src="source(item.image)" :alt="item.title" /><span>{{ kinds.find(kind => kind.value === item.kind)?.label }}</span></div><div class="showcase-info"><div><strong>{{ item.title || '无标题内容' }}</strong><small>{{ item.subtitle || item.prompt || '未填写说明' }}</small></div><b>权重 {{ item.weight }}</b></div><div class="showcase-actions"><a-button size="small" long @click="openEdit(item)"><IconEdit />编辑</a-button><a-button size="small" status="danger" @click="remove(item)"><IconDelete /></a-button></div></article></div><div v-else class="empty"><IconImage /><strong>暂无首页展示内容</strong><p>从一张已生成作品开始配置公共首页。</p><a-button type="primary" @click="openCreate()">新增内容</a-button></div></a-spin>

    <a-modal v-model:visible="visible" :width="760" :title="form.id ? '编辑首页内容' : '新增首页内容'" :footer="false">
      <div class="editor-preview" :class="{ empty: !form.image }">
        <img v-if="form.image" :src="source(form.image)" alt="图片预览" />
        <div v-else><IconImage /><span>尚未选择图片</span></div>
        <button class="picker-command" @click="openGeneratedPicker"><IconImage />选择已生成图片</button>
        <div v-if="form.image" class="preview-caption"><span>{{ form.subtitle }}</span><strong>{{ form.title }}</strong></div>
      </div>
      <a-form :model="form" layout="vertical" class="editor-form">
        <div class="form-grid"><a-form-item label="内容类型"><a-select v-model="form.kind"><a-option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</a-option></a-select></a-form-item><a-form-item label="排序权重"><a-input-number v-model="form.weight" :min="0" style="width:100%" /></a-form-item></div>
        <div class="form-grid"><a-form-item label="标题"><a-input v-model="form.title" placeholder="作品标题" /></a-form-item><a-form-item label="副标题"><a-input v-model="form.subtitle" placeholder="例如 PRODUCT STUDY" /></a-form-item></div>
        <a-form-item v-if="form.kind !== 'work'" label="提示词"><a-textarea v-model="form.prompt" :auto-size="{minRows:3,maxRows:6}" /></a-form-item>
        <div class="modal-actions"><a-button @click="visible=false">取消</a-button><a-button type="primary" :loading="saving" @click="save">保存</a-button></div>
      </a-form>
    </a-modal>

    <a-modal v-model:visible="pickerOpen" :width="920" title="选择已生成图片" :footer="false">
      <a-spin :loading="pickerLoading" class="picker-loading">
        <div v-if="generated.length" class="generated-grid"><button v-for="item in generated" :key="item.name" :class="{ selected: form.image === `/images/${String(item.name).replace(/^\/+/, '')}` }" @click="chooseGenerated(item)"><img :src="generatedSource(item.name, true)" :alt="item.prompt || item.name" /><span><IconCheck v-if="form.image === `/images/${String(item.name).replace(/^\/+/, '')}`" />{{ item.prompt || '未记录提示词' }}</span></button></div>
        <div v-else-if="!pickerLoading" class="picker-empty"><IconImage /><strong>暂无可选择图片</strong><p>用户生成成功的图片会自动出现在这里。</p></div>
      </a-spin>
    </a-modal>
  </div>
</template>

<style scoped>
.toolbar{min-height:54px;margin-bottom:18px;padding:8px 10px;display:flex;align-items:center;justify-content:space-between;gap:18px;border-block:1px solid var(--ns-line)}.filter-pills{display:flex;align-items:center;gap:5px}.filter-pills button{height:31px;padding:0 13px;border:0;border-radius:999px;background:transparent;color:var(--ns-ink-soft);font-size:10px;cursor:pointer}.filter-pills button:hover{background:var(--ns-surface-muted)}.filter-pills button.active{background:#242a25;color:#fff}.showcase-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:14px}.showcase-card{min-width:0;border:1px solid var(--ns-line);border-radius:8px;background:#fff;overflow:hidden}.showcase-image{position:relative;aspect-ratio:4/3;background:var(--ns-surface-muted);overflow:hidden}.showcase-image img{width:100%;height:100%;object-fit:cover}.showcase-image>span{position:absolute;top:10px;left:10px;padding:5px 8px;border-radius:999px;background:rgba(29,34,30,.8);color:#fff;font-size:9px}.showcase-info{padding:13px 13px 10px;display:flex;justify-content:space-between;align-items:start;gap:10px}.showcase-info>div{min-width:0;display:flex;flex-direction:column}.showcase-info strong{font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.showcase-info small{margin-top:4px;color:var(--ns-ink-faint);font-size:9px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.showcase-info b{flex:0 0 auto;color:var(--ns-accent-strong);font-size:9px}.showcase-actions{padding:0 13px 13px;display:flex;gap:7px}.empty{min-height:370px;display:flex;flex-direction:column;align-items:center;justify-content:center;color:var(--ns-ink-faint)}.empty :deep(svg){width:29px;height:29px}.empty strong{margin-top:15px;color:var(--ns-ink);font-size:13px}.empty p{margin:7px 0 18px;font-size:10px}.editor-preview{height:172px;margin-bottom:18px;position:relative;overflow:hidden;border-radius:7px;background:#252b26}.editor-preview img{width:100%;height:100%;object-fit:cover}.editor-preview.empty{display:grid;place-items:center;background:var(--ns-surface-muted);border:1px dashed var(--ns-line-strong)}.editor-preview.empty>div{display:flex;flex-direction:column;align-items:center;gap:7px;color:var(--ns-ink-faint);font-size:10px}.picker-command{height:31px;padding:0 11px;position:absolute;top:11px;right:11px;display:flex;align-items:center;gap:6px;border:1px solid rgba(255,255,255,.45);border-radius:6px;background:rgba(27,31,28,.82);color:#fff;font-size:10px;cursor:pointer;backdrop-filter:blur(8px)}.preview-caption{position:absolute;left:11px;right:11px;bottom:11px;padding:9px 10px;background:rgba(29,34,30,.78);color:#fff;display:flex;justify-content:space-between;align-items:center}.preview-caption span{color:#c3cac4;font-size:9px}.preview-caption strong{font-size:11px}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.modal-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:6px}.picker-loading{width:100%;min-height:260px}.generated-grid{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:10px}.generated-grid button{aspect-ratio:1;padding:0;position:relative;overflow:hidden;border:2px solid transparent;border-radius:7px;background:#edf0ea;cursor:pointer}.generated-grid button.selected{border-color:#64735e}.generated-grid img{width:100%;height:100%;object-fit:cover}.generated-grid span{position:absolute;inset:auto 5px 5px;padding:5px 6px;display:flex;align-items:center;gap:4px;border-radius:5px;background:rgba(26,30,27,.72);color:#fff;font-size:8px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.picker-empty{min-height:260px;display:flex;flex-direction:column;align-items:center;justify-content:center;color:var(--ns-ink-faint)}.picker-empty strong{margin-top:10px;color:var(--ns-ink-soft)}.picker-empty p{margin:5px 0 0;font-size:10px}@media(max-width:1150px){.showcase-grid{grid-template-columns:repeat(3,1fr)}}@media(max-width:760px){.showcase-grid{grid-template-columns:repeat(2,1fr)}.toolbar{align-items:start;flex-direction:column}.form-grid{grid-template-columns:1fr}.generated-grid{grid-template-columns:repeat(3,1fr)}}@media(max-width:480px){.showcase-grid{grid-template-columns:1fr}.generated-grid{grid-template-columns:repeat(2,1fr)}}
.editor-preview.empty{min-height:0}
</style>

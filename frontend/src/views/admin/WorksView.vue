<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconDelete, IconDownload, IconImage, IconRefresh, IconVideoCamera } from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../../services/api'
import MediaPreview from '../../components/MediaPreview.vue'

const rows = ref<any[]>([])
const stats = ref<any>({ total: 0, image: 0, video: 0, size_bytes: 0 })
const loading = ref(false)
const kind = ref('')
const selected = ref<string[]>([])
const preview = ref<any>(null)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

function mediaUrl(name: string, thumbnail = false) { return imageUrl(`/images/${String(name).replace(/^\/+/, '')}${thumbnail ? '.thumb.jpg' : ''}`) }
function formatSize(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`
}
function formatTime(value: number) { return value ? new Date(value * 1000).toLocaleString('zh-CN', { hour12: false }) : '--' }
async function downloadWork(row: any) {
  const url = mediaUrl(row.name)
  const filename = String(row.name || '').split('/').pop() || (row.kind === 'video' ? 'video.mp4' : 'image.png')
  try {
    const response = await fetch(url)
    if (!response.ok) throw new Error('download failed')
    const objectUrl = URL.createObjectURL(await response.blob())
    const anchor = document.createElement('a')
    anchor.href = objectUrl
    anchor.download = filename
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
  } catch {
    Message.error('文件下载失败，请稍后重试')
  }
}
async function load() {
  loading.value = true
  const query = new URLSearchParams({ limit: String(pageSize), offset: String((page.value - 1) * pageSize) })
  if (kind.value) query.set('kind', kind.value)
  const response = await api(`/images?${query}`)
  loading.value = false
  if (!response.ok) return Message.error(response.data?.detail || '作品加载失败')
  rows.value = response.data?.data || []
  total.value = Number(response.data?.total || 0)
  stats.value = response.data?.stats || stats.value
  selected.value = selected.value.filter((name) => rows.value.some((row) => row.name === name))
}
function setKind(value: string) { kind.value = value; page.value = 1; selected.value = []; load() }
function toggleAll(checked: boolean | Array<string | number | boolean>) { selected.value = checked === true ? rows.value.map((row) => row.name) : [] }
async function removeNames(names: string[]) {
  if (!names.length) return
  Modal.warning({
    title: '确认删除作品',
    content: `将永久删除 ${names.length} 个文件及其缩略图，删除后无法恢复。`,
    hideCancel: false,
    okText: '确认删除',
    onOk: async () => {
      let removed = 0
      for (const name of names) {
        const response = await api(`/images?name=${encodeURIComponent(name)}`, { method: 'DELETE' })
        if (response.ok) removed += 1
      }
      selected.value = []
      preview.value = null
      await load()
      Message.success(`已删除 ${removed} 个作品`)
    },
  })
}
onMounted(load)
</script>

<template>
  <div class="works-page">
    <div class="section-heading"><div><span class="eyebrow">GENERATED ASSETS</span><h2>作品管理</h2><p>统一查看和清理用户生成的图像与视频文件。</p></div><a-button :loading="loading" @click="load"><template #icon><IconRefresh /></template>刷新</a-button></div>
    <section class="metrics"><div><span>全部作品</span><strong>{{ stats.total }}</strong></div><div><span>图像</span><strong>{{ stats.image }}</strong></div><div><span>视频</span><strong>{{ stats.video }}</strong></div><div><span>占用空间</span><strong>{{ formatSize(stats.size_bytes) }}</strong></div></section>
    <div class="toolbar"><div class="filters"><button :class="{ active: kind === '' }" @click="setKind('')">全部</button><button :class="{ active: kind === 'image' }" @click="setKind('image')"><IconImage />图像</button><button :class="{ active: kind === 'video' }" @click="setKind('video')"><IconVideoCamera />视频</button></div><div class="batch"><span v-if="selected.length">已选 {{ selected.length }} 项</span><a-button v-if="selected.length" status="danger" @click="removeNames(selected)"><template #icon><IconDelete /></template>批量删除</a-button></div></div>
    <div class="select-line"><a-checkbox :model-value="rows.length > 0 && selected.length === rows.length" @change="toggleAll">选择本页</a-checkbox><span>每页 {{ pageSize }} 个 · 共 {{ total }} 个文件</span></div>
    <a-spin :loading="loading" class="works-loading">
      <div v-if="rows.length" class="gallery-grid">
        <figure v-for="row in rows" :key="row.name" class="gallery-item">
          <button class="media-button" :aria-label="`预览${row.kind === 'video' ? '视频' : '图像'}`" @click="preview = row"><img v-if="!row._thumb_error" :src="mediaUrl(row.name, true)" :alt="row.prompt || row.name" loading="lazy" @error="row._thumb_error = true" /><span v-else class="media-fallback"><IconVideoCamera v-if="row.kind === 'video'" /><IconImage v-else /></span></button>
          <a-checkbox v-model="selected" :value="row.name" class="picker" @click.stop />
          <span class="kind"><IconVideoCamera v-if="row.kind === 'video'" /><IconImage v-else />{{ row.kind === 'video' ? '视频' : '图像' }}</span>
          <span class="item-actions">
            <a-tooltip content="下载"><button type="button" aria-label="下载作品" @click.stop="downloadWork(row)"><IconDownload /></button></a-tooltip>
            <a-tooltip content="删除"><button type="button" aria-label="删除作品" @click.stop="removeNames([row.name])"><IconDelete /></button></a-tooltip>
          </span>
        </figure>
      </div>
      <div v-else-if="!loading" class="empty"><IconImage /><strong>暂无生成作品</strong><span>用户完成生成后，文件会自动出现在这里。</span></div>
    </a-spin>
    <a-pagination v-if="pages > 1" v-model:current="page" :total="total" :page-size="pageSize" show-total @change="load" />
    <MediaPreview :visible="Boolean(preview)" :src="preview ? mediaUrl(preview.name) : ''" :kind="preview?.kind" :filename="preview?.name?.split('/').pop()" downloadable @close="preview = null" />
  </div>
</template>

<style scoped>
.works-page{max-width:1280px}.eyebrow{display:block;margin-bottom:6px;color:#8a7628;font-size:9px;font-weight:750;letter-spacing:.12em}.metrics{display:grid;grid-template-columns:repeat(4,1fr);border-block:1px solid var(--ns-line);margin-bottom:18px}.metrics>div{padding:17px 20px;border-right:1px solid var(--ns-line);display:flex;flex-direction:column}.metrics>div:last-child{border:0}.metrics span{font-size:9px;color:var(--ns-ink-faint)}.metrics strong{margin-top:5px;font-size:21px}.toolbar,.select-line{display:flex;align-items:center;justify-content:space-between;gap:14px}.toolbar{margin-bottom:12px}.filters{display:flex;padding:3px;background:#e8eae4;border-radius:7px}.filters button{height:32px;padding:0 14px;border:0;border-radius:5px;background:transparent;color:var(--ns-ink-soft);display:flex;align-items:center;gap:6px;cursor:pointer}.filters button.active{background:#fff;color:var(--ns-ink);box-shadow:0 1px 3px rgba(0,0,0,.08)}.batch{display:flex;align-items:center;gap:10px;font-size:10px;color:var(--ns-ink-faint)}.select-line{padding:10px 2px;border-top:1px solid var(--ns-line);font-size:10px;color:var(--ns-ink-faint)}.works-loading{width:100%;min-height:220px}.gallery-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px}.gallery-item{aspect-ratio:1;margin:0;position:relative;overflow:hidden;border-radius:7px;background:#e8ebe5}.media-button{width:100%;height:100%;padding:0;border:0;background:transparent;cursor:pointer}.media-button img{width:100%;height:100%;display:block;object-fit:cover;transition:transform .22s ease}.gallery-item:hover .media-button img{transform:scale(1.025)}.media-fallback{width:100%;height:100%;display:grid;place-items:center;color:#929991}.media-fallback svg{width:30px;height:30px}.picker{position:absolute;top:8px;left:8px;padding:5px;border-radius:5px;background:transparent}.picker :deep(.arco-checkbox-icon){border-color:rgba(255,255,255,.82);background:rgba(24,30,26,.18);box-shadow:0 1px 5px rgba(20,25,22,.2);backdrop-filter:blur(6px)}.picker :deep(.arco-checkbox-checked .arco-checkbox-icon){border-color:#273029;background:#273029}.kind{position:absolute;left:8px;bottom:8px;height:24px;padding:0 8px;border-radius:999px;background:rgba(28,32,29,.76);color:#fff;display:flex;align-items:center;gap:5px;font-size:8px}.item-actions{position:absolute;right:8px;top:8px;display:flex;gap:6px;opacity:0;transform:translateY(-2px);transition:opacity .16s ease,transform .16s ease}.item-actions button{width:28px;height:28px;padding:0;display:grid;place-items:center;border:1px solid rgba(255,255,255,.14);border-radius:50%;background:rgba(28,32,29,.76);color:#fff;cursor:pointer;backdrop-filter:blur(7px)}.item-actions button:hover{background:rgba(28,32,29,.94)}.gallery-item:hover .item-actions,.item-actions:focus-within{opacity:1;transform:none}.empty{min-height:320px;display:flex;align-items:center;justify-content:center;flex-direction:column;color:var(--ns-ink-faint)}.empty svg{width:30px;height:30px}.empty strong{margin-top:12px;color:var(--ns-ink-soft);font-size:12px}.empty span{margin-top:5px;font-size:10px}.preview-media{display:block;max-width:100%;max-height:60vh;margin:auto;object-fit:contain}.preview-meta{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-top:13px;padding-top:12px;border-top:1px solid var(--ns-line)}.preview-meta>div{min-width:0;display:flex;flex-direction:column}.preview-meta strong{font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.preview-meta span{margin-top:5px;color:var(--ns-ink-faint);font-size:9px}.works-page :deep(.arco-pagination){justify-content:flex-end;margin-top:18px}@media(max-width:1100px){.gallery-grid{grid-template-columns:repeat(4,1fr)}}@media(max-width:760px){.metrics{grid-template-columns:1fr 1fr}.metrics>div:nth-child(2){border-right:0}.gallery-grid{grid-template-columns:repeat(3,1fr)}.item-actions{opacity:1;transform:none}}@media(max-width:480px){.gallery-grid{grid-template-columns:repeat(2,1fr)}.toolbar{align-items:flex-start;flex-direction:column}.batch{width:100%;justify-content:space-between}.preview-meta{align-items:stretch;flex-direction:column}}
</style>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import {
  IconBookmark, IconClockCircle, IconFire, IconImage, IconStarFill,
  IconPlayCircle, IconSearch, IconStar, IconThunderbolt,
} from '@arco-design/web-vue/es/icon'
import { useRouter } from 'vue-router'
import { api, imageUrl } from '../../services/api'
import { useAuthStore } from '../../stores/auth'

type PromptVariable = {
  name: string
  label: string
  type: string
  placeholder?: string
  default?: string
  required?: boolean
  options?: string[]
}

const auth = useAuthStore()
const router = useRouter()
const rows = ref<any[]>([])
const categories = ref<any[]>([])
const favoriteIDs = ref(new Set<string>())
const selected = ref<any | null>(null)
const detailOpen = ref(false)
const loading = ref(false)
const using = ref(false)
const total = ref(0)
const offset = ref(0)
const pageSize = 24
const query = ref('')
const mediaType = ref('all')
const category = ref('all')
const sort = ref('popular')
const values = reactive<Record<string, string>>({})
let searchTimer: number | undefined

const hasMore = computed(() => rows.value.length < total.value)
const variables = computed<PromptVariable[]>(() => Array.isArray(selected.value?.variables) ? selected.value.variables : [])

function coverFor(item: any) {
  if (item.cover) return imageUrl(item.cover)
  const source = String(item.id || item.title || '')
  const hash = [...source].reduce((sum, char) => sum + char.charCodeAt(0), 0)
  return imageUrl(`/inspiration/${String((hash % 30) + 1).padStart(2, '0')}.jpg`)
}
function categoryName(id: string) {
  return categories.value.find((item) => item.id === id)?.name || '灵感模板'
}
function formatCount(value: unknown) {
  const count = Number(value || 0)
  if (count >= 10000) return `${(count / 10000).toFixed(1)}w`
  if (count >= 1000) return `${(count / 1000).toFixed(1)}k`
  return String(count)
}

async function load(reset = true) {
  if (loading.value) return
  loading.value = true
  if (reset) offset.value = 0
  const params = new URLSearchParams({ limit: String(pageSize), offset: String(offset.value), sort: sort.value })
  if (query.value.trim()) params.set('q', query.value.trim())
  if (mediaType.value !== 'all') params.set('media_type', mediaType.value)
  if (category.value !== 'all') params.set('category', category.value)
  const endpoint = auth.isAuthed ? '/prompt-library' : '/prompts'
  const response = await api(`${endpoint}?${params}`)
  loading.value = false
  if (!response.ok) return Message.error(response.data?.detail || '灵感内容加载失败')
  const incoming = response.data?.data || []
  rows.value = reset ? incoming : [...rows.value, ...incoming]
  total.value = Number(response.data?.total || 0)
  offset.value = rows.value.length
}

async function loadBase() {
  const [categoryResponse, favoriteResponse] = await Promise.all([
    api('/prompt-categories'),
    auth.isAuthed ? api('/prompt-favorites') : Promise.resolve({ ok: true, data: { data: [] } }),
  ])
  if (categoryResponse.ok) categories.value = categoryResponse.data?.data || []
  if (favoriteResponse.ok) favoriteIDs.value = new Set(favoriteResponse.data?.data || [])
  await load()
}

function setFilter(kind: 'media' | 'category' | 'sort', value: string) {
  if (kind === 'media') mediaType.value = value
  if (kind === 'category') category.value = value
  if (kind === 'sort') sort.value = value
  load()
}

function scrollCategories(event: WheelEvent) {
  const element = event.currentTarget as HTMLElement
  if (element.scrollWidth <= element.clientWidth) return
  const distance = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY
  if (!distance) return
  element.scrollLeft += distance
  event.preventDefault()
}

function openDetail(item: any) {
  selected.value = item
  Object.keys(values).forEach((key) => delete values[key])
  ;(Array.isArray(item.variables) ? item.variables : []).forEach((variable: PromptVariable) => {
    values[variable.name] = variable.default || ''
  })
  detailOpen.value = true
}

async function toggleFavorite(item: any, event?: Event) {
  event?.stopPropagation()
  if (!auth.isAuthed) return auth.openLogin('/inspiration')
  const favorite = !favoriteIDs.value.has(item.id)
  const response = await api(`/prompts/${item.id}/favorite`, { method: 'PUT', body: JSON.stringify({ favorite }) })
  if (!response.ok) return Message.error(response.data?.detail || '收藏操作失败')
  const next = new Set(favoriteIDs.value)
  favorite ? next.add(item.id) : next.delete(item.id)
  favoriteIDs.value = next
  item.favorite_count = response.data?.favorite_count ?? item.favorite_count
  if (selected.value?.id === item.id) selected.value.favorite_count = item.favorite_count
  Message.success(favorite ? '已加入收藏' : '已取消收藏')
}

async function useTemplate() {
  if (!selected.value) return
  if (!auth.isAuthed) return auth.openLogin('/inspiration')
  for (const variable of variables.value) {
    if (variable.required && !String(values[variable.name] || '').trim()) return Message.warning(`请填写${variable.label}`)
  }
  using.value = true
  const response = await api(`/prompts/${selected.value.id}/use`, { method: 'POST', body: JSON.stringify({ values }) })
  using.value = false
  if (!response.ok) return Message.error(response.data?.detail || '模板应用失败')
  const payload = { ...response.data, title: selected.value.title, stored_at: Date.now() }
  sessionStorage.setItem('creation_template_payload', JSON.stringify(payload))
  detailOpen.value = false
  await router.push(selected.value.media_type === 'video' ? `/app/video?template=${selected.value.id}` : `/app/generate?template=${selected.value.id}`)
}

watch(query, () => {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => load(), 320)
})
onMounted(loadBase)
</script>

<template>
  <div class="inspiration-page">
    <section class="library-intro">
      <div class="intro-copy">
        <span class="eyebrow">CURATED PROMPT LIBRARY</span>
        <h2>从一个好想法开始</h2>
        <p>挑选经过整理的创作模板，填写关键内容，直接进入图片或视频工作台。</p>
      </div>
      <div class="library-stat"><strong>{{ total.toLocaleString() }}</strong><span>个可用灵感</span><small>内容持续由运营团队更新</small></div>
    </section>

    <section class="library-controls">
      <div class="search-field"><IconSearch /><input v-model="query" type="search" placeholder="搜索场景、风格或用途" /><kbd>搜索</kbd></div>
      <div class="type-switch" aria-label="模板类型">
        <button :class="{ active: mediaType === 'all' }" @click="setFilter('media', 'all')"><IconStar />全部</button>
        <button :class="{ active: mediaType === 'image' }" @click="setFilter('media', 'image')"><IconImage />图片</button>
        <button :class="{ active: mediaType === 'video' }" @click="setFilter('media', 'video')"><IconPlayCircle />视频</button>
      </div>
    </section>

    <nav class="category-pills" aria-label="灵感分类" @wheel="scrollCategories">
      <button :class="{ active: category === 'all' }" @click="setFilter('category', 'all')">全部分类</button>
      <button v-for="item in categories" :key="item.id" :class="{ active: category === item.id }" @click="setFilter('category', item.id)">{{ item.name }}<span>{{ item.template_count }}</span></button>
    </nav>

    <div class="result-head">
      <div><strong>{{ category === 'all' ? '本周精选' : categoryName(category) }}</strong><span>共 {{ total }} 个模板</span></div>
      <div class="sort-switch">
        <button :class="{ active: sort === 'popular' }" @click="setFilter('sort', 'popular')"><IconFire />热门</button>
        <button :class="{ active: sort === 'latest' }" @click="setFilter('sort', 'latest')"><IconClockCircle />最新</button>
        <button v-if="auth.isAuthed" :class="{ active: sort === 'favorite' }" @click="setFilter('sort', 'favorite')"><IconBookmark />收藏优先</button>
      </div>
    </div>

    <a-spin :loading="loading && !rows.length" class="template-region">
      <div v-if="rows.length" class="template-grid">
        <article v-for="item in rows" :key="item.id" class="template-item" tabindex="0" @click="openDetail(item)" @keydown.enter="openDetail(item)">
          <div class="template-cover">
            <img :src="coverFor(item)" :alt="item.title" loading="lazy" />
            <span class="media-badge"><component :is="item.media_type === 'video' ? IconPlayCircle : IconImage" />{{ item.media_type === 'video' ? '视频' : '图片' }}</span>
            <button class="save-button" :class="{ saved: favoriteIDs.has(item.id) }" :aria-label="favoriteIDs.has(item.id) ? '取消收藏' : '收藏模板'" @click="toggleFavorite(item, $event)"><component :is="favoriteIDs.has(item.id) ? IconStarFill : IconBookmark" /></button>
          </div>
          <div class="template-copy"><span>{{ categoryName(item.category_id) }}</span><h3>{{ item.title }}</h3><p>{{ item.description }}</p><footer><span><IconThunderbolt />{{ formatCount(item.use_count) }} 次使用</span><span>{{ formatCount(item.favorite_count) }} 收藏</span></footer></div>
        </article>
      </div>
      <a-empty v-else-if="!loading" description="当前筛选下暂无模板" />
    </a-spin>
    <button v-if="hasMore" class="load-more" :disabled="loading" @click="load(false)">{{ loading ? '正在加载' : '加载更多' }}</button>

    <a-modal v-model:visible="detailOpen" :width="880" :footer="false" unmount-on-close modal-class="prompt-detail-modal">
      <template #title><span class="modal-title">模板详情</span></template>
      <div v-if="selected" class="detail-layout">
        <figure class="detail-visual"><img :src="coverFor(selected)" :alt="selected.title" /><figcaption><span><component :is="selected.media_type === 'video' ? IconPlayCircle : IconImage" />{{ selected.media_type === 'video' ? '视频模板' : '图片模板' }}</span><small>{{ categoryName(selected.category_id) }}</small></figcaption></figure>
        <section class="detail-editor">
          <div class="detail-heading"><span>{{ categoryName(selected.category_id) }}</span><h3>{{ selected.title }}</h3><p>{{ selected.description }}</p></div>
          <div v-if="variables.length" class="variable-fields">
            <label v-for="variable in variables" :key="variable.name"><span>{{ variable.label }}<i v-if="variable.required">必填</i></span>
              <a-select v-if="variable.type === 'select' && variable.options?.length" v-model="values[variable.name]" :placeholder="variable.placeholder || `选择${variable.label}`"><a-option v-for="option in variable.options" :key="option" :value="option">{{ option }}</a-option></a-select>
              <a-input v-else v-model="values[variable.name]" :placeholder="variable.placeholder || `填写${variable.label}`" />
            </label>
          </div>
          <div v-else class="ready-note"><IconStar /><span><strong>无需填写变量</strong><small>模板已经整理完成，可以直接带入创作页面。</small></span></div>
          <div class="prompt-preview"><span>提示词结构</span><p>{{ selected.prompt }}</p></div>
          <footer class="detail-actions"><button class="favorite-action" @click="toggleFavorite(selected)"><component :is="favoriteIDs.has(selected.id) ? IconStarFill : IconBookmark" />{{ favoriteIDs.has(selected.id) ? '已收藏' : '收藏' }}</button><a-button type="primary" size="large" :loading="using" @click="useTemplate"><IconThunderbolt />{{ auth.isAuthed ? '使用此模板' : '登录后使用' }}</a-button></footer>
        </section>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
.inspiration-page{max-width:1380px;margin:0 auto}.library-intro{min-height:210px;display:flex;align-items:center;justify-content:space-between;gap:40px;padding:34px 42px;margin-bottom:20px;overflow:hidden;position:relative;background:#2a302b;color:#fff;border-radius:8px}.library-intro:after{content:"";position:absolute;width:270px;height:270px;right:18%;top:-150px;border:48px solid rgba(222,190,72,.17);border-radius:50%}.intro-copy{position:relative;z-index:1}.eyebrow{display:block;margin-bottom:15px;color:#d9bf58;font-size:9px;font-weight:750;letter-spacing:.14em}.intro-copy h2{margin:0;font-size:34px;line-height:1.2;letter-spacing:0}.intro-copy p{max-width:560px;margin:14px 0 0;color:#cdd2cc;font-size:13px;line-height:1.8}.library-stat{min-width:180px;display:flex;flex-direction:column;position:relative;z-index:1;padding-left:26px;border-left:1px solid rgba(255,255,255,.2)}.library-stat strong{font-size:35px;color:#f2d66a}.library-stat span{margin-top:3px;font-size:12px;font-weight:650}.library-stat small{margin-top:9px;color:#aeb7af;font-size:9px}.library-controls{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:15px 0}.search-field{height:46px;width:min(100%,520px);display:flex;align-items:center;gap:11px;padding:0 15px;border:1px solid var(--ns-line);border-radius:999px;background:#fff}.search-field :deep(svg){color:#777f78}.search-field input{min-width:0;flex:1;border:0;outline:0;background:transparent;color:var(--ns-ink);font:inherit;font-size:12px}.search-field kbd{padding:4px 7px;border:1px solid #e0e2dc;border-radius:4px;background:#f5f6f2;color:#999e98;font:9px inherit}.type-switch,.sort-switch{display:flex;align-items:center;gap:4px;padding:4px;border-radius:999px;background:#e9ebe6}.type-switch button,.sort-switch button{height:34px;display:flex;align-items:center;gap:6px;padding:0 13px;border:0;border-radius:999px;background:transparent;color:var(--ns-ink-soft);font-size:10px;cursor:pointer}.type-switch button.active,.sort-switch button.active{background:#fff;color:var(--ns-ink);box-shadow:0 2px 7px rgba(32,38,33,.08)}.category-pills{display:flex;align-items:center;gap:7px;padding:4px 0 20px;overflow:auto;scrollbar-width:none}.category-pills::-webkit-scrollbar{display:none}.category-pills button{height:34px;flex:0 0 auto;padding:0 14px;border:1px solid var(--ns-line);border-radius:999px;background:#fff;color:var(--ns-ink-soft);font-size:10px;cursor:pointer}.category-pills button span{margin-left:6px;color:#a0a59e}.category-pills button.active{border-color:#2d342e;background:#2d342e;color:#fff}.category-pills button.active span{color:#d5dad4}.result-head{min-height:56px;display:flex;align-items:center;justify-content:space-between;gap:18px;border-top:1px solid var(--ns-line)}.result-head>div:first-child{display:flex;align-items:baseline;gap:10px}.result-head strong{font-size:14px}.result-head span{color:var(--ns-ink-faint);font-size:10px}.sort-switch{padding:2px;background:transparent}.sort-switch button{height:30px;padding-inline:10px}.sort-switch button.active{background:#e5e8e2;box-shadow:none}.template-region{display:block;min-height:280px}.template-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:22px 16px}.template-item{min-width:0;overflow:hidden;border:1px solid var(--ns-line);border-radius:7px;background:#fff;cursor:pointer;transition:transform .2s ease,box-shadow .2s ease}.template-item:hover{transform:translateY(-3px);box-shadow:0 12px 25px rgba(35,42,36,.09)}.template-cover{aspect-ratio:1.25;position:relative;overflow:hidden;background:#e8eae5}.template-cover img{width:100%;height:100%;display:block;object-fit:cover;transition:transform .45s ease}.template-item:hover .template-cover img{transform:scale(1.035)}.media-badge{height:26px;position:absolute;left:10px;top:10px;display:flex;align-items:center;gap:5px;padding:0 9px;border-radius:999px;background:rgba(34,39,35,.78);color:#fff;font-size:9px;backdrop-filter:blur(7px)}.save-button{width:30px;height:30px;position:absolute;right:9px;top:9px;display:grid;place-items:center;border:0;border-radius:50%;background:rgba(255,255,255,.9);color:#4f5750;cursor:pointer}.save-button.saved{background:#e2c65c;color:#302b17}.template-copy{padding:14px 15px 13px}.template-copy>span{color:#8a7628;font-size:8px;font-weight:720}.template-copy h3{margin:6px 0 6px;font-size:13px}.template-copy p{height:34px;overflow:hidden;margin:0;color:var(--ns-ink-soft);font-size:10px;line-height:1.7}.template-copy footer{display:flex;align-items:center;justify-content:space-between;margin-top:13px;padding-top:10px;border-top:1px solid #eceee8;color:var(--ns-ink-faint);font-size:9px}.template-copy footer span{display:flex;align-items:center;gap:4px}.load-more{height:40px;display:block;margin:28px auto 0;padding:0 24px;border:1px solid var(--ns-line);border-radius:999px;background:#fff;color:var(--ns-ink-soft);cursor:pointer}.detail-layout{display:grid;grid-template-columns:minmax(0,.92fr) minmax(360px,1.08fr);min-height:540px}.detail-visual{min-width:0;margin:0;position:relative;background:#e6e8e2;overflow:hidden}.detail-visual img{width:100%;height:100%;min-height:540px;display:block;object-fit:cover}.detail-visual figcaption{position:absolute;left:16px;right:16px;bottom:16px;display:flex;align-items:center;justify-content:space-between;padding:10px 12px;border-radius:6px;background:rgba(35,40,36,.82);color:#fff;backdrop-filter:blur(10px)}.detail-visual figcaption span{display:flex;align-items:center;gap:6px;font-size:10px}.detail-visual figcaption small{color:#d4d9d3;font-size:9px}.detail-editor{min-width:0;padding:26px 28px;display:flex;flex-direction:column}.detail-heading>span{color:#8a7628;font-size:9px;font-weight:700}.detail-heading h3{margin:8px 0;font-size:21px}.detail-heading p{margin:0;color:var(--ns-ink-soft);font-size:11px;line-height:1.7}.variable-fields{display:grid;grid-template-columns:1fr 1fr;gap:14px 12px;margin-top:22px}.variable-fields label{min-width:0}.variable-fields label>span{display:flex;align-items:center;justify-content:space-between;margin-bottom:7px;font-size:10px;font-weight:620}.variable-fields i{font-style:normal;color:#9c7720;font-size:8px}.prompt-preview{margin-top:20px;padding-top:16px;border-top:1px solid var(--ns-line)}.prompt-preview>span{color:var(--ns-ink-faint);font-size:9px}.prompt-preview p{max-height:88px;overflow:auto;margin:8px 0 0;color:#596159;font-size:10px;line-height:1.7}.ready-note{display:flex;align-items:center;gap:11px;margin-top:22px;padding:13px 0;border-top:1px solid var(--ns-line);border-bottom:1px solid var(--ns-line);color:#6a785f}.ready-note span{display:flex;flex-direction:column}.ready-note strong{color:var(--ns-ink);font-size:10px}.ready-note small{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.detail-actions{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:auto;padding-top:24px}.favorite-action{height:38px;display:flex;align-items:center;gap:7px;padding:0 14px;border:1px solid var(--ns-line);border-radius:999px;background:#fff;color:var(--ns-ink-soft);cursor:pointer}.modal-title{font-size:13px}
:global(.prompt-detail-modal .arco-modal-body){padding:0!important}:global(.prompt-detail-modal .arco-modal){overflow:hidden}
@media(max-width:1100px){.template-grid{grid-template-columns:repeat(3,minmax(0,1fr))}}@media(max-width:760px){.library-intro{min-height:190px;padding:26px}.library-stat{display:none}.intro-copy h2{font-size:27px}.library-controls{align-items:stretch;flex-direction:column}.search-field{width:100%}.type-switch{align-self:flex-start}.template-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.detail-layout{grid-template-columns:1fr}.detail-visual img{min-height:260px;max-height:330px}.variable-fields{grid-template-columns:1fr}}@media(max-width:480px){.template-grid{grid-template-columns:1fr}.result-head{align-items:flex-start;flex-direction:column;padding:12px 0}.sort-switch{max-width:100%;overflow:auto}.library-intro{padding:22px}.intro-copy h2{font-size:24px}.template-cover{aspect-ratio:1.45}}
.category-pills{padding-bottom:12px;overflow-x:auto;overflow-y:hidden;overscroll-behavior-inline:contain;touch-action:pan-x;scrollbar-width:thin;scrollbar-color:#b9beb7 transparent}
.category-pills::-webkit-scrollbar{display:block;height:5px}
.category-pills::-webkit-scrollbar-track{background:transparent}
.category-pills::-webkit-scrollbar-thumb{border-radius:999px;background:#b9beb7}
.category-pills::-webkit-scrollbar-thumb:hover{background:#8f9790}
@media(min-width:761px){.category-pills{min-width:0;flex-wrap:wrap;padding-bottom:16px;overflow:visible;scrollbar-width:none}.category-pills::-webkit-scrollbar{display:none}}
@media(max-width:760px){.category-pills{min-width:0;max-width:100%;flex-wrap:nowrap}}
</style>

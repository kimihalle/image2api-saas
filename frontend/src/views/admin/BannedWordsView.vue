<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconDelete, IconExclamationCircle, IconImport, IconPlus, IconRefresh, IconSafe, IconSearch,
} from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'

const pageSize = 20
const activeTab = ref('words')
const words = ref<any[]>([])
const hits = ref<any[]>([])
const stats = ref({ words: 0, hits: 0, today_hits: 0, auto_bans: 0 })
const wordTotal = ref(0)
const hitTotal = ref(0)
const wordPage = ref(1)
const hitPage = ref(1)
const wordQuery = ref('')
const wordCategory = ref('all')
const hitQuery = ref('')
const newWord = ref('')
const newCategory = ref('custom')
const newAutoBan = ref(false)
const importText = ref('')
const importCategory = ref('custom')
const importAutoBan = ref(false)
const importOpen = ref(false)
const loadingWords = ref(false)
const loadingHits = ref(false)
const adding = ref(false)
const importing = ref(false)
const savingPolicy = ref('')
let wordTimer: number | undefined
let hitTimer: number | undefined

const categoryOptions = [
  { value: 'custom', label: '自定义' },
  { value: 'sexual', label: '色情低俗' },
  { value: 'politics', label: '中国政治' },
  { value: 'violence', label: '暴力血腥' },
  { value: 'illegal', label: '违法犯罪' },
]

function categoryLabel(value?: string) {
  return categoryOptions.find((item) => item.value === value)?.label || '自定义'
}

function categoryColor(value?: string) {
  return ({ politics: 'orangered', sexual: 'magenta', violence: 'red', illegal: 'orange', custom: 'gray' } as Record<string, string>)[value || 'custom']
}

function applyRequiredPolicy(target: 'new' | 'import') {
  if (target === 'new') newAutoBan.value = newCategory.value === 'politics'
  if (target === 'import') importAutoBan.value = importCategory.value === 'politics'
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '--'
}

async function loadWords(reset = false) {
  if (reset) wordPage.value = 1
  loadingWords.value = true
  const params = new URLSearchParams({ limit: String(pageSize), offset: String((wordPage.value - 1) * pageSize) })
  if (wordQuery.value.trim()) params.set('q', wordQuery.value.trim())
  if (wordCategory.value !== 'all') params.set('category', wordCategory.value)
  const response = await api(`/banned-words?${params}`)
  loadingWords.value = false
  if (!response.ok) return Message.error(response.data?.detail || '违禁词词库加载失败')
  words.value = response.data?.data || []
  wordTotal.value = Number(response.data?.total || 0)
  stats.value = response.data?.stats || stats.value
}

async function loadHits(reset = false) {
  if (reset) hitPage.value = 1
  loadingHits.value = true
  const params = new URLSearchParams({ limit: String(pageSize), offset: String((hitPage.value - 1) * pageSize) })
  if (hitQuery.value.trim()) params.set('q', hitQuery.value.trim())
  const response = await api(`/banned-word-hits?${params}`)
  loadingHits.value = false
  if (!response.ok) return Message.error(response.data?.detail || '触发记录加载失败')
  hits.value = response.data?.data || []
  hitTotal.value = Number(response.data?.total || 0)
}

async function refreshAll() {
  await Promise.all([loadWords(), loadHits()])
}

async function addWord() {
  const word = newWord.value.trim()
  if (!word) return Message.warning('请输入违禁词')
  adding.value = true
  const response = await api('/banned-words', {
    method: 'POST', body: JSON.stringify({ word, category: newCategory.value, auto_ban: newAutoBan.value }),
  })
  adding.value = false
  if (!response.ok) return Message.error(response.data?.detail || '添加失败')
  newWord.value = ''
  Message.success('违禁词已添加')
  await loadWords(true)
}

async function importWords() {
  if (!importText.value.trim()) return Message.warning('请输入要导入的违禁词')
  importing.value = true
  const response = await api('/banned-words/import', {
    method: 'POST',
    body: JSON.stringify({ text: importText.value, category: importCategory.value, auto_ban: importAutoBan.value }),
  })
  importing.value = false
  if (!response.ok) return Message.error(response.data?.detail || '批量导入失败')
  importOpen.value = false
  importText.value = ''
  Message.success(`已添加 ${Number(response.data?.added || 0)} 个，跳过 ${Number(response.data?.skipped || 0)} 个`)
  await loadWords(true)
}

async function updatePolicy(row: any, category: string, autoBan: boolean) {
  if (savingPolicy.value) return
  savingPolicy.value = row.id
  const response = await api(`/banned-words/${row.id}`, {
    method: 'PATCH', body: JSON.stringify({ category, auto_ban: category === 'politics' ? true : autoBan }),
  })
  savingPolicy.value = ''
  if (!response.ok) {
    Message.error(response.data?.detail || '策略更新失败')
    return loadWords()
  }
  Object.assign(row, response.data?.data || {})
  Message.success('处置策略已更新')
}

function removeWord(row: any) {
  Modal.warning({
    title: '删除违禁词',
    content: `确认从词库中删除“${row.word}”？删除后新的图片请求将不再拦截该词。`,
    hideCancel: false,
    okText: '确认删除',
    onOk: async () => {
      const response = await api(`/banned-words/${row.id}`, { method: 'DELETE' })
      if (!response.ok) return Message.error(response.data?.detail || '删除失败')
      Message.success('已删除')
      await loadWords()
    },
  })
}

async function copyPrompt(value: string) {
  await navigator.clipboard.writeText(value || '')
  Message.success('原提示词已复制')
}

watch(wordQuery, () => {
  if (wordTimer) window.clearTimeout(wordTimer)
  wordTimer = window.setTimeout(() => loadWords(true), 280)
})
watch(wordCategory, () => loadWords(true))
watch(newCategory, () => applyRequiredPolicy('new'))
watch(importCategory, () => applyRequiredPolicy('import'))
watch(hitQuery, () => {
  if (hitTimer) window.clearTimeout(hitTimer)
  hitTimer = window.setTimeout(() => loadHits(true), 280)
})
onMounted(() => Promise.all([loadWords(), loadHits()]))
</script>

<template>
  <div class="safety-page">
    <div class="section-heading">
      <div><span class="eyebrow">CONTENT SAFETY</span><h2>违禁词管理</h2><p>图片生成请求在入队和执行前进行双重检查，视频生成由上游服务审核。</p></div>
      <a-button :loading="loadingWords || loadingHits" @click="refreshAll"><template #icon><IconRefresh /></template>刷新</a-button>
    </div>

    <section class="metrics" aria-label="内容安全统计">
      <div><span>词库总量</span><strong>{{ stats.words }}</strong><small>当前生效规则</small></div>
      <div><span>累计拦截</span><strong class="danger">{{ stats.hits }}</strong><small>图片请求命中次数</small></div>
      <div><span>今日拦截</span><strong class="warning">{{ stats.today_hits }}</strong><small>自今日 00:00 起</small></div>
      <div><span>自动封禁</span><strong class="danger">{{ stats.auto_bans }}</strong><small>政治违规账号</small></div>
      <div class="scope"><span class="scope-icon"><IconSafe /></span><strong>仅图片生成</strong><small>工作台与 OpenAI 图片接口</small></div>
    </section>

    <a-tabs v-model:active-key="activeTab" class="safety-tabs">
      <a-tab-pane key="words" title="违禁词词库">
        <div class="toolbar">
          <div class="filter-control"><a-input v-model="wordQuery" allow-clear placeholder="搜索违禁词"><template #prefix><IconSearch /></template></a-input><a-select v-model="wordCategory"><a-option value="all">全部分类</a-option><a-option v-for="item in categoryOptions" :key="item.value" :value="item.value">{{ item.label }}</a-option></a-select></div>
          <div class="add-control"><a-input v-model="newWord" placeholder="输入单个违禁词" @press-enter="addWord" /><a-select v-model="newCategory"><a-option v-for="item in categoryOptions" :key="item.value" :value="item.value">{{ item.label }}</a-option></a-select><a-button type="primary" :loading="adding" @click="addWord"><template #icon><IconPlus /></template>添加</a-button></div>
          <a-button @click="importOpen = true"><template #icon><IconImport /></template>批量导入</a-button>
        </div>
        <div class="table-shell">
          <a-table :data="words" :loading="loadingWords" :pagination="false" row-key="id">
            <template #columns>
              <a-table-column title="违禁词" data-index="word" :width="300"><template #cell="{ record }"><strong class="word-value">{{ record.word }}</strong></template></a-table-column>
              <a-table-column title="分类" :width="145"><template #cell="{ record }"><a-select class="policy-select" :model-value="record.category" :loading="savingPolicy === record.id" @change="updatePolicy(record, String($event), String($event) === 'politics')"><a-option v-for="item in categoryOptions" :key="item.value" :value="item.value">{{ item.label }}</a-option></a-select></template></a-table-column>
              <a-table-column title="处置" :width="150"><template #cell="{ record }"><div class="policy-cell"><a-switch :model-value="record.auto_ban" :disabled="record.category === 'politics' || savingPolicy === record.id" size="small" @change="updatePolicy(record, record.category, Boolean($event))" /><span :class="{ severe: record.auto_ban }">{{ record.auto_ban ? '自动封禁' : '仅拦截' }}</span></div></template></a-table-column>
              <a-table-column title="命中次数" data-index="hits" :width="140"><template #cell="{ record }"><span :class="['hit-count', { active: record.hits > 0 }]">{{ Number(record.hits || 0) }}</span></template></a-table-column>
              <a-table-column title="添加时间" :width="220"><template #cell="{ record }"><span class="muted">{{ formatTime(record.created_at) }}</span></template></a-table-column>
              <a-table-column title="操作" :width="90" align="right"><template #cell="{ record }"><a-tooltip content="删除"><a-button type="text" status="danger" aria-label="删除违禁词" @click="removeWord(record)"><IconDelete /></a-button></a-tooltip></template></a-table-column>
            </template>
            <template #empty><div class="empty"><IconSafe /><span>当前没有匹配的违禁词</span></div></template>
          </a-table>
        </div>
        <a-pagination v-if="wordTotal > pageSize" v-model:current="wordPage" :total="wordTotal" :page-size="pageSize" show-total @change="loadWords()" />
      </a-tab-pane>

      <a-tab-pane key="hits" title="触发记录">
        <div class="toolbar hits-toolbar"><a-input v-model="hitQuery" allow-clear placeholder="搜索用户、违禁词或原提示词"><template #prefix><IconSearch /></template></a-input><span>共 {{ hitTotal }} 条审计记录</span></div>
        <div class="table-shell">
          <a-table :data="hits" :loading="loadingHits" :pagination="false" row-key="id" :scroll="{ x: 980 }">
            <template #columns>
              <a-table-column title="命中词" data-index="word" :width="170"><template #cell="{ record }"><div class="hit-word"><a-tag :color="categoryColor(record.category)">{{ record.word }}</a-tag><small>{{ categoryLabel(record.category) }}</small></div></template></a-table-column>
              <a-table-column title="用户" :width="210"><template #cell="{ record }"><div class="user-cell"><strong>{{ record.user_name || 'API 用户' }}</strong><small>{{ record.user_id || '--' }}</small></div></template></a-table-column>
              <a-table-column title="处置结果" :width="130"><template #cell="{ record }"><a-tag :color="record.auto_banned ? 'red' : 'gray'">{{ record.auto_banned ? '账号已封禁' : '请求已拦截' }}</a-tag></template></a-table-column>
              <a-table-column title="原提示词" :width="400"><template #cell="{ record }"><button class="prompt-copy" :title="record.prompt" @click="copyPrompt(record.prompt)">{{ record.prompt }}</button></template></a-table-column>
              <a-table-column title="触发时间" :width="200"><template #cell="{ record }"><span class="muted">{{ formatTime(record.created_at) }}</span></template></a-table-column>
            </template>
            <template #empty><div class="empty"><IconExclamationCircle /><span>暂无触发记录</span></div></template>
          </a-table>
        </div>
        <a-pagination v-if="hitTotal > pageSize" v-model:current="hitPage" :total="hitTotal" :page-size="pageSize" show-total @change="loadHits()" />
      </a-tab-pane>
    </a-tabs>

    <a-modal v-model:visible="importOpen" title="批量导入违禁词" :ok-loading="importing" ok-text="开始导入" :width="600" @ok="importWords">
      <div class="import-note"><IconSafe /><span><strong>一行一个，也支持逗号、顿号和分号分隔</strong><small>系统自动跳过空行与已存在的词条，导入后立即对图片生成生效。</small></span></div>
      <div class="import-policy"><label><span>内容分类</span><a-select v-model="importCategory"><a-option v-for="item in categoryOptions" :key="item.value" :value="item.value">{{ item.label }}</a-option></a-select></label><label><span>命中后自动封禁</span><a-switch v-model="importAutoBan" :disabled="importCategory === 'politics'" /></label></div>
      <a-textarea v-model="importText" :auto-size="{ minRows: 10, maxRows: 16 }" placeholder="输入要导入的违禁词" />
    </a-modal>
  </div>
</template>

<style scoped>
.safety-page{max-width:1280px}.eyebrow{display:block;margin-bottom:6px;color:#8a7628;font-size:9px;font-weight:750;letter-spacing:.12em}.metrics{display:grid;grid-template-columns:repeat(4,1fr);margin-bottom:18px;border-block:1px solid var(--ns-line)}.metrics>div{min-height:92px;padding:17px 20px;display:flex;flex-direction:column;border-right:1px solid var(--ns-line)}.metrics>div:last-child{border:0}.metrics span,.metrics small{color:var(--ns-ink-faint);font-size:9px}.metrics strong{margin:5px 0 4px;font-size:22px}.metrics strong.danger{color:#9a493d}.metrics strong.warning{color:#987424}.metrics .scope{position:relative;padding-left:62px;justify-content:center}.scope-icon{width:32px;height:32px;position:absolute;left:20px;top:29px;display:grid;place-items:center;border-radius:50%;background:#e3e9df;color:#566650}.metrics .scope strong{font-size:13px}.safety-tabs :deep(.arco-tabs-nav-tab){justify-content:flex-start}.toolbar{display:grid;grid-template-columns:minmax(220px,340px) minmax(300px,440px) auto;gap:10px;align-items:center;margin:5px 0 14px}.add-control{display:grid;grid-template-columns:1fr auto}.add-control :deep(.arco-input-wrapper){border-radius:5px 0 0 5px}.add-control .arco-btn{border-radius:0 5px 5px 0}.hits-toolbar{grid-template-columns:minmax(280px,440px) 1fr}.hits-toolbar>span{text-align:right;color:var(--ns-ink-faint);font-size:10px}.table-shell{overflow:hidden;border:1px solid var(--ns-line);border-radius:8px;background:#fff}.word-value{font-size:12px}.hit-count{display:inline-grid;min-width:28px;height:24px;place-items:center;padding:0 7px;border-radius:999px;background:#eef0eb;color:var(--ns-ink-soft);font-size:10px}.hit-count.active{background:#f1e4df;color:#8a4d40}.muted{color:var(--ns-ink-faint);font-size:10px}.user-cell{min-width:0;display:flex;flex-direction:column}.user-cell strong{overflow:hidden;text-overflow:ellipsis;font-size:11px}.user-cell small{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.prompt-copy{max-width:100%;padding:0;border:0;background:transparent;color:var(--ns-ink-soft);font-size:10px;line-height:1.55;text-align:left;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;cursor:copy}.prompt-copy:hover{color:var(--ns-ink)}.empty{height:180px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:8px;color:var(--ns-ink-faint)}.empty svg{width:26px;height:26px}.pagination{display:flex;justify-content:flex-end;margin-top:14px}.import-note{display:flex;align-items:center;gap:11px;margin-bottom:16px;padding:13px 14px;border:1px solid #dfe4d9;border-radius:7px;background:#f5f7f2;color:#60705a}.import-note>span{display:flex;flex-direction:column}.import-note strong{color:var(--ns-ink);font-size:11px}.import-note small{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}@media(max-width:800px){.metrics{grid-template-columns:1fr 1fr}.metrics>div:nth-child(2){border-right:0}.toolbar{grid-template-columns:1fr}.hits-toolbar{grid-template-columns:1fr}.hits-toolbar>span{text-align:left}}@media(max-width:480px){.metrics{grid-template-columns:1fr}.metrics>div{border-right:0;border-bottom:1px solid var(--ns-line)}.metrics>div:last-child{border-bottom:0}.add-control{grid-template-columns:1fr}.add-control :deep(.arco-input-wrapper),.add-control .arco-btn{border-radius:5px}.add-control .arco-btn{margin-top:7px}}
.metrics{grid-template-columns:repeat(5,1fr)}
.toolbar{grid-template-columns:minmax(300px,390px) minmax(440px,1fr) auto}
.filter-control,.add-control{display:grid;grid-template-columns:minmax(0,1fr) 132px}
.add-control{grid-template-columns:minmax(0,1fr) 126px auto}
.filter-control,.add-control{height:34px;align-items:stretch}
.filter-control :deep(.arco-input-wrapper),.filter-control :deep(.arco-select-view),.add-control :deep(.arco-input-wrapper),.add-control :deep(.arco-select-view){height:34px}
.filter-control :deep(.arco-input-wrapper),.add-control :deep(.arco-input-wrapper){border-radius:7px 0 0 7px}
.filter-control :deep(.arco-select-view),.add-control :deep(.arco-select-view){border-left:0;border-radius:0 7px 7px 0}
.add-control .arco-btn{height:34px;margin-left:10px;padding-inline:18px;border-radius:999px!important;align-self:stretch}
.toolbar>.arco-btn{height:34px;align-self:center;white-space:nowrap}
.policy-select{width:118px}
.policy-cell{display:flex;align-items:center;gap:7px;color:var(--ns-ink-faint);font-size:10px}
.policy-cell .severe{color:#9a493d;font-weight:700}
.hit-word{display:flex;align-items:center;gap:7px}.hit-word small{color:var(--ns-ink-faint);font-size:9px}
.import-policy{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:14px}
.import-policy label{display:flex;align-items:center;justify-content:space-between;gap:12px;color:var(--ns-ink-soft);font-size:10px}
.import-policy .arco-select{width:150px}
@media(max-width:960px){.metrics{grid-template-columns:repeat(2,1fr)}.toolbar{grid-template-columns:1fr}.metrics>div:nth-child(2n){border-right:0}}
@media(max-width:560px){.metrics{grid-template-columns:1fr}.filter-control,.add-control{height:auto;grid-template-columns:1fr;gap:7px}.filter-control :deep(.arco-input-wrapper),.filter-control :deep(.arco-select-view),.add-control :deep(.arco-input-wrapper),.add-control :deep(.arco-select-view),.add-control .arco-btn{border:1px solid var(--color-border-2);border-radius:7px!important}.add-control .arco-btn{width:100%;margin-left:0}.toolbar>.arco-btn{width:100%}.import-policy{grid-template-columns:1fr}.import-policy label{align-items:flex-start;flex-direction:column}.import-policy .arco-select{width:100%}}
</style>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconApps, IconCheck, IconCloseCircle, IconDelete, IconDownload, IconImage, IconLoading, IconMessage, IconPlus, IconThunderbolt as IconSend, IconUpload } from '@arco-design/web-vue/es/icon'
import GenerationLoader from '../../components/GenerationLoader.vue'
import MediaPreview from '../../components/MediaPreview.vue'
import { api, imageUrl } from '../../services/api'
import { useAuthStore } from '../../stores/auth'

type OutputStatus = 'queued' | 'running' | 'success' | 'failed'
type OutputItem = { id: string; index: number; status: OutputStatus; data?: any; error?: string; position?: number }
type ImageMode = 'studio' | 'chat'
type ImageTurn = {
  id: string
  prompt: string
  createdAt: number
  model: string
  ratio: string
  resolution: string
  quantity: number
  cost: number
  tasks: OutputItem[]
  historical?: boolean
}

const MODE_KEY = 'image_generation_mode'

const auth = useAuthStore()

const models = ref<any[]>([])
const loading = ref(false)
const quantity = ref(1)
const outputs = ref<OutputItem[]>([])
const turns = ref<ImageTurn[]>([])
const previewOutput = ref<OutputItem | null>(null)
const mode = ref<ImageMode>('studio')
const modeChooserOpen = ref(false)
const promptEditorOpen = ref(false)
const rememberMode = ref(false)
const selectedChooserMode = ref<ImageMode>('studio')
const conversation = ref<HTMLElement>()
const deaiConfig = ref<any>({ enabled: false, price_1k: 0, price_2k: 0, price_4k: 0 })
const form = reactive({ model: '', prompt: '', ratio: '1:1', resolution: '1K', reference_images: [] as string[], deai: false })
const available = computed(() => models.value.filter((x) => x.type === 'image' && x.enabled !== false))
const selected = computed(() => available.value.find((x) => x.id === form.model || publicModelID(x) === form.model))
const referenceLimit = computed(() => Math.max(1, Number(selected.value?.max_reference_images || 1)))
const deaiCharge = computed(() => form.deai ? Number(deaiConfig.value[`price_${form.resolution.toLowerCase()}`] || 0) : 0)
const price = computed(() => { const m = selected.value; return m ? Number(m.prices?.[form.resolution] || 0) + deaiCharge.value : 0 })
const taskCount = computed(() => quantity.value)
const totalPrice = computed(() => Number((price.value * taskCount.value).toFixed(4)))
const completedCount = computed(() => outputs.value.filter((item) => item.status === 'success' || item.status === 'failed').length)
let pollTimer: ReturnType<typeof setTimeout> | undefined
let polling = false
let destroyed = false
let batchFinalized = true
let generationStream: EventSource | null = null

function publicModelID(model: any) {
  return String(model?.alias || model?.id || '').trim()
}

function publicModelName(model: any) {
  return String(model?.alias || model?.name || model?.id || '图片模型').trim()
}

function initializeMode() {
  const stored = localStorage.getItem(MODE_KEY)
  if (stored === 'studio' || stored === 'chat') {
    mode.value = stored
    selectedChooserMode.value = stored
    rememberMode.value = true
    return
  }
  modeChooserOpen.value = true
}

function snapConversationToBottom() {
  nextTick(() => {
    const element = conversation.value
    if (element) element.scrollTop = element.scrollHeight
  })
}

function chooseMode(value: ImageMode) {
  selectedChooserMode.value = value
}

function enterSelectedMode() {
  mode.value = selectedChooserMode.value
  if (rememberMode.value) localStorage.setItem(MODE_KEY, mode.value)
  else localStorage.removeItem(MODE_KEY)
  modeChooserOpen.value = false
  if (mode.value === 'chat') snapConversationToBottom()
}

function switchMode(value: ImageMode) {
  mode.value = value
  if (localStorage.getItem(MODE_KEY)) localStorage.setItem(MODE_KEY, value)
  if (value === 'chat') snapConversationToBottom()
}

function formatTime(value: number) {
  if (!value) return ''
  const milliseconds = value < 10_000_000_000 ? value * 1000 : value
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(milliseconds))
}

function historyFileUrl(file: unknown) {
  const value = String(file || '').replace(/^\/+/, '')
  return value ? imageUrl(`/images/${value}`) : ''
}

async function loadHistory() {
  const response = await api('/logs?kind=image&source=user&limit=36')
  if (!response.ok) return
  const rows = Array.isArray(response.data?.data) ? response.data.data : []
  const grouped = new Map<string, ImageTurn>()
  for (const row of [...rows].reverse()) {
    if (!['success', 'failed'].includes(String(row.status || '').toLowerCase())) continue
    const createdAt = Number(row.ts || row.created_at || 0)
    const bucket = Math.floor(createdAt / 12)
    const key = [row.prompt, row.model, row.ratio, row.resolution, bucket].join('|')
    let turn = grouped.get(key)
    if (!turn) {
      turn = {
        id: `history-${row.id}`,
        prompt: row.prompt || '未填写生成描述',
        createdAt,
        model: row.model || '图片模型',
        ratio: row.ratio || '—',
        resolution: row.resolution || '—',
        quantity: 0,
        cost: 0,
        tasks: [],
        historical: true,
      }
      grouped.set(key, turn)
    }
    const task: OutputItem = {
      id: String(row.id),
      index: turn.tasks.length,
      status: row.status === 'success' ? 'success' : 'failed',
      data: row.file ? { url: historyFileUrl(row.file), elapsed_ms: row.elapsed_ms } : undefined,
      error: row.error || undefined,
    }
    turn.tasks.push(task)
    turn.quantity += 1
    turn.cost += Number(row.cost || 0)
  }
  turns.value = [...grouped.values()]
  if (mode.value === 'chat') snapConversationToBottom()
}

function applyStoredTemplate() {
  try {
    const raw = sessionStorage.getItem('creation_template_payload')
    if (!raw) return
    const payload = JSON.parse(raw)
    if (payload.media_type !== 'image' || Date.now() - Number(payload.stored_at || 0) > 10 * 60 * 1000) return
    const compatible = Array.isArray(payload.compatible_models) ? payload.compatible_models : []
    const target = available.value.find((item) => !compatible.length || compatible.includes(item.id) || compatible.includes(publicModelID(item)))
    if (target) form.model = publicModelID(target)
    form.prompt = String(payload.prompt || '')
    const model = target || selected.value
    const allowedRatios = Array.isArray(model?.ratios) ? model.ratios : []
    const ratios = Array.isArray(payload.ratios) ? payload.ratios : []
    const ratio = ratios.find((item: string) => !allowedRatios.length || allowedRatios.includes(item))
    if (ratio) form.ratio = ratio
    const allowedResolutions = Object.keys(model?.prices || {})
    const resolutions = Array.isArray(payload.resolutions) ? payload.resolutions : []
    const resolution = resolutions.find((item: string) => !allowedResolutions.length || allowedResolutions.includes(item))
    if (resolution) form.resolution = resolution
    sessionStorage.removeItem('creation_template_payload')
    Message.success(`已应用“${payload.title || '灵感模板'}”`)
  } catch { sessionStorage.removeItem('creation_template_payload') }
}

onMounted(async () => {
  initializeMode()
  const [modelResponse, deaiResponse, activeResponse] = await Promise.all([
    api('/models'),
    api('/deai-pricing'),
    api('/generation/jobs'),
    loadHistory(),
  ])
  models.value = (modelResponse.data?.data || modelResponse.data || []).filter((item: any) => item.enabled !== false)
  if (deaiResponse.ok) deaiConfig.value = deaiResponse.data || deaiConfig.value
  form.model = publicModelID(models.value.find((x) => x.type === 'image'))
  applyStoredTemplate()
  if (activeResponse.ok && Array.isArray(activeResponse.data?.data) && activeResponse.data.data.length) {
    outputs.value = activeResponse.data.data.map((job: any, index: number) => mapQueueJob(job, index))
    turns.value.push({
      id: `active-${outputs.value[0]?.id || Date.now()}`,
      prompt: activeResponse.data.data[0]?.prompt || '正在恢复未完成的图片任务',
      createdAt: Number(activeResponse.data.data[0]?.created_at || Date.now()),
      model: activeResponse.data.data[0]?.model || '图片模型',
      ratio: activeResponse.data.data[0]?.ratio || '—',
      resolution: activeResponse.data.data[0]?.resolution || '—',
      quantity: outputs.value.length,
      cost: 0,
      tasks: outputs.value,
    })
    loading.value = true
    batchFinalized = false
    schedulePoll(250)
  }
  if (mode.value === 'chat') snapConversationToBottom()
  connectGenerationStream()
})
onBeforeUnmount(() => {
  destroyed = true
  if (pollTimer) clearTimeout(pollTimer)
  generationStream?.close()
})
watch(selected, (model) => {
  if (!model) return
  const ratios = model.ratios || ['1:1']
  const resolutions = Object.keys(model.prices || {})
  if (!ratios.includes(form.ratio)) form.ratio = ratios[0] || '1:1'
  if (resolutions.length && !resolutions.includes(form.resolution)) form.resolution = resolutions[0]
  if (form.reference_images.length > referenceLimit.value) form.reference_images.splice(referenceLimit.value)
})
function outputUrl(item?: OutputItem | null) {
  const data = item?.data
  return imageUrl(data?.url || data?.image_url || data?.data?.[0]?.url)
}

function mapQueueJob(job: any, index: number, current?: OutputItem): OutputItem {
  const statusMap: Record<string, OutputStatus> = {
    queued: 'queued',
    retry_wait: 'queued',
    processing: 'running',
    completed: 'success',
    failed: 'failed',
    dead_letter: 'failed',
    cancelled: 'failed',
  }
  return {
    id: String(job.id),
    index: current?.index ?? index,
    status: statusMap[job.status] || 'failed',
    data: job.result || current?.data,
    error: job.error || (job.status === 'cancelled' ? '任务已取消，未扣除额度' : current?.error),
    position: Number(job.position || 0),
  }
}

function isActiveTask(item: OutputItem) {
  return item.status === 'queued' || item.status === 'running'
}

function applyStreamJob(job: any) {
  if (!job?.id) return
  const index = outputs.value.findIndex((candidate) => candidate.id === String(job.id))
  if (index < 0) return
  replaceTask(mapQueueJob(job, index, outputs.value[index]))
  const active = outputs.value.some(isActiveTask)
  loading.value = active
  if (!active) finishBatch()
}

function connectGenerationStream() {
  generationStream?.close()
  generationStream = new EventSource('/admin/api/generation/events')
  generationStream.addEventListener('snapshot', (event: MessageEvent) => {
    try {
      const payload = JSON.parse(event.data)
      for (const job of payload.jobs || []) applyStreamJob(job)
    } catch { /* polling remains the fallback */ }
  })
  generationStream.addEventListener('generation', (event: MessageEvent) => {
    try { applyStreamJob(JSON.parse(event.data)?.job) } catch { /* polling remains the fallback */ }
  })
  generationStream.onerror = () => {
    if (outputs.value.some(isActiveTask)) schedulePoll(3000)
  }
}

function replaceTask(updated: OutputItem) {
  const outputIndex = outputs.value.findIndex((item) => item.id === updated.id)
  if (outputIndex >= 0) outputs.value[outputIndex] = updated
  for (const turn of turns.value) {
    const taskIndex = turn.tasks.findIndex((item) => item.id === updated.id)
    if (taskIndex >= 0) turn.tasks[taskIndex] = updated
  }
}

function schedulePoll(delay = 1600) {
  if (destroyed) return
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = setTimeout(pollJobs, delay)
}

async function pollJobs() {
  if (destroyed || polling) return
  const active = outputs.value.filter(isActiveTask)
  if (!active.length) {
    await finishBatch()
    return
  }
  polling = true
  try {
    const responses = await Promise.all(active.map(async (item) => ({ item, response: await api(`/generation/jobs/${item.id}`) })))
    for (const { item, response } of responses) {
      if (!response.ok) continue
      const index = outputs.value.findIndex((candidate) => candidate.id === item.id)
      if (index >= 0) replaceTask(mapQueueJob(response.data, index, outputs.value[index]))
    }
  } finally {
    polling = false
  }
  const stillActive = outputs.value.some(isActiveTask)
  loading.value = stillActive
  if (stillActive) schedulePoll()
  else await finishBatch()
}

async function finishBatch() {
  loading.value = false
  if (batchFinalized) return
  batchFinalized = true
  await auth.refreshUser()
  const succeeded = outputs.value.filter((item) => item.status === 'success').length
  const failed = outputs.value.filter((item) => item.status === 'failed').length
  if (!failed) Message.success(`${succeeded} 个任务已完成，额度已确认扣除`)
  else if (!succeeded) Message.error(outputs.value[0]?.error || '生成失败，未扣费或已自动退款')
  else Message.warning(`${succeeded} 个任务完成，${failed} 个失败；失败任务未扣费或已自动退款`)
}

async function generate() {
  if (!form.prompt.trim()) return Message.warning('请输入生成描述')
  if (!form.model) return Message.warning('请选择可用模型')
  if (Number(auth.user?.credits || 0) < totalPrice.value) return Message.warning(`可用额度不足，本次需要 ${totalPrice.value} 额度`)
  loading.value = true
  previewOutput.value = null
  outputs.value = []
  batchFinalized = false
  const response = await api('/generation/jobs', {
    method: 'POST',
    body: JSON.stringify({ ...form, reference_images: [...form.reference_images], count: taskCount.value }),
  })
  if (!response.ok) {
    loading.value = false
    batchFinalized = true
    return Message.error(response.data?.detail || '任务提交失败，请稍后重试')
  }
  outputs.value = (response.data?.data || []).map((job: any, index: number) => mapQueueJob(job, index))
  turns.value.push({
    id: `turn-${outputs.value[0]?.id || Date.now()}`,
    prompt: form.prompt.trim(),
    createdAt: Date.now(),
    model: publicModelName(selected.value),
    ratio: form.ratio,
    resolution: form.resolution,
    quantity: taskCount.value,
    cost: totalPrice.value,
    tasks: outputs.value,
  })
  Message.success(`${outputs.value.length} 个任务已进入生成队列`)
  snapConversationToBottom()
  schedulePoll(500)
}
function chooseReference() { (document.querySelector('#reference-file') as HTMLInputElement)?.click() }
async function readReference(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (!files.length) return
  const remaining = Math.max(0, referenceLimit.value - form.reference_images.length)
  if (!remaining) return Message.warning(`当前模型最多上传 ${referenceLimit.value} 张参考图`)
  const accepted = files.filter((file) => {
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) { Message.warning(`${file.name} 格式不支持`); return false }
    if (file.size > 20 * 1024 * 1024) { Message.warning(`${file.name} 超过 20MB`); return false }
    return true
  }).slice(0, remaining)
  const encoded = await Promise.all(accepted.map((file) => new Promise<string>((resolve) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.readAsDataURL(file)
  })))
  form.reference_images.push(...encoded.filter(Boolean))
  if (files.length > accepted.length) Message.info(`已保留 ${form.reference_images.length} / ${referenceLimit.value} 张参考图`)
}
function removeReference(index: number) { form.reference_images.splice(index, 1) }

function eventID(item: OutputItem, historical = false) {
  return String(item.data?.event_id || (historical ? item.id : '')).trim()
}

function downloadImage(item: OutputItem, index: number) {
  const url = outputUrl(item)
  if (!url) return Message.info('图片文件尚未准备完成')
  const link = document.createElement('a')
  link.href = url
  link.download = `image-${String(index + 1).padStart(2, '0')}.png`
  document.body.appendChild(link)
  link.click()
  link.remove()
}

function downloadTurn(turn: ImageTurn) {
  const completed = turn.tasks.filter((item) => item.status === 'success' && outputUrl(item))
  if (!completed.length) return Message.info('当前对话没有可下载的图片')
  completed.forEach((item, index) => window.setTimeout(() => downloadImage(item, index), index * 180))
}

async function deleteTaskRecord(item: OutputItem, historical = false) {
  const id = eventID(item, historical)
  if (!id) return true
  const response = await api(`/generation/history/${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!response.ok) {
    Message.error(response.data?.detail || '删除图片记录失败')
    return false
  }
  return true
}

function removeImageTask(turn: ImageTurn, item: OutputItem) {
  if (item.status === 'queued' || item.status === 'running') return Message.warning('任务仍在处理中，暂时不能删除')
  Modal.warning({
    title: '删除当前图片',
    content: '图片文件和对应生成记录将同时删除，此操作无法撤销。',
    hideCancel: false,
    onOk: async () => {
      if (!await deleteTaskRecord(item, Boolean(turn.historical))) return false
      turn.tasks = turn.tasks.filter((candidate) => candidate.id !== item.id)
      if (!turn.tasks.length) turns.value = turns.value.filter((candidate) => candidate.id !== turn.id)
      Message.success('图片已删除')
    },
  })
}

function removeTurn(turn: ImageTurn) {
  if (turn.tasks.some((item) => item.status === 'queued' || item.status === 'running')) return Message.warning('当前对话仍有任务处理中')
  Modal.warning({
    title: '删除整轮对话',
    content: `将删除本轮的 ${turn.tasks.length} 条图片记录及作品文件，此操作无法撤销。`,
    hideCancel: false,
    onOk: async () => {
      const results = await Promise.all(turn.tasks.map((item) => deleteTaskRecord(item, Boolean(turn.historical))))
      if (!results.every(Boolean)) return false
      turns.value = turns.value.filter((candidate) => candidate.id !== turn.id)
      Message.success('本轮对话已删除')
    },
  })
}

function clearConversation() {
  const active = turns.value.filter((turn) => turn.tasks.some((item) => item.status === 'queued' || item.status === 'running'))
  turns.value = active
  Message.success(active.length ? '已清除完成的对话，进行中的任务已保留' : '当前对话已清屏')
}
</script>

<template>
  <div class="generate-page">
    <div class="section-heading image-heading">
      <div><h2>图片生成</h2><p>{{ mode === 'studio' ? '集中配置参数并对照查看本轮结果。' : '以连续对话整理灵感、参数与每一轮作品。' }}</p></div>
      <div class="heading-actions">
        <button v-if="mode === 'chat' && turns.length" type="button" class="clear-screen" title="清空当前屏幕，不删除作品" @click="clearConversation"><IconDelete /><span>清屏</span></button>
        <span class="billing-state"><i></i>事务计费已启用</span>
        <div class="mode-switch" role="radiogroup" aria-label="图片创作模式">
          <a-tooltip content="工作台模式"><button type="button" :class="{ active: mode === 'studio' }" :aria-checked="mode === 'studio'" aria-label="工作台模式" role="radio" @click="switchMode('studio')"><IconApps /><span>工作台</span></button></a-tooltip>
          <a-tooltip content="对话模式"><button type="button" :class="{ active: mode === 'chat' }" :aria-checked="mode === 'chat'" aria-label="对话模式" role="radio" @click="switchMode('chat')"><IconMessage /><span>对话</span></button></a-tooltip>
        </div>
      </div>
    </div>

    <div v-if="mode === 'studio'" class="studio">
      <section class="composer">
        <div class="field"><label>模型</label><a-select v-model="form.model" size="large" :placeholder="available.length ? '选择模型' : '后台暂未发布此类模型'"><a-option v-for="m in available" :key="m.id" :value="publicModelID(m)" :label="m.alias || m.name || m.id" /></a-select></div>
        <div class="field"><div class="label-row"><label>生成描述</label><span>{{ form.prompt.length }} / 2000</span></div><a-textarea v-model="form.prompt" class="studio-prompt-input" :max-length="2000" :auto-size="false" :rows="7" placeholder="描述主体、场景、光线、构图和风格；可拖动增高，双击打开大输入框" @dblclick="promptEditorOpen = true" /></div>
        <div class="parameter-grid">
          <div class="field"><label>画面比例</label><a-select v-model="form.ratio"><a-option v-for="x in selected?.ratios || ['1:1']" :key="x">{{ x }}</a-option></a-select></div>
          <div class="field"><label>清晰度</label><a-select v-model="form.resolution"><a-option v-for="x in Object.keys(selected?.prices || {})" :key="x">{{ x }}</a-option></a-select></div>
          <div class="field"><label>生成张数</label><div class="quantity-control" role="radiogroup" aria-label="生成张数"><button v-for="value in [1, 2, 3, 4]" :key="value" type="button" :class="{ active: quantity === value }" :aria-checked="quantity === value" role="radio" @click="quantity = value">{{ value }}</button></div></div>
        </div>
        <input id="reference-file" type="file" accept="image/png,image/jpeg,image/webp" multiple hidden @change="readReference" />
        <button class="reference" @click="chooseReference"><IconPlus /><span><strong>{{ form.reference_images.length ? `已上传 ${form.reference_images.length} / ${referenceLimit} 张参考图` : '添加参考图' }}</strong><small>PNG、JPG 或 WEBP，单张不超过 20MB</small></span></button>
        <div v-if="form.reference_images.length" class="reference-strip"><figure v-for="(item, index) in form.reference_images" :key="`${index}-${item.length}`"><img :src="item" :alt="`参考图 ${index + 1}`" /><button :aria-label="`移除参考图 ${index + 1}`" @click="removeReference(index)"><IconDelete /></button><span>{{ index + 1 }}</span></figure></div>
        <div v-if="deaiConfig.enabled" class="deai-row"><span><strong>去 AI 特征</strong><small>生成后进行弱特征处理，本次增加 {{ deaiCharge || Number(deaiConfig[`price_${form.resolution.toLowerCase()}`] || 0) }} 额度</small></span><a-switch v-model="form.deai" /></div>
        <div class="submit-row"><span>预计消耗 <strong>{{ totalPrice }}</strong> 额度</span><a-button class="studio-submit" type="primary" size="large" :loading="loading" :disabled="loading" @click="generate"><IconSend /><span>{{ loading ? `${completedCount} / ${outputs.length || taskCount} 处理中` : `生成 ${taskCount} 张` }}</span><i></i><strong>{{ totalPrice }} 额度</strong></a-button></div>
      </section>
      <section class="preview" :class="{ 'has-results': outputs.length }" aria-live="polite">
        <div v-if="!outputs.length" class="empty"><span class="preview-mark"><IconImage /></span><h3>输出预览</h3><p>生成结果会显示在这里，并自动保存到生成记录。</p></div>
        <div v-else class="output-wrap">
          <div class="output-summary"><div><strong>{{ loading ? '任务正在执行' : '本轮生成结果' }}</strong><span>{{ completedCount }} / {{ outputs.length }} 已处理</span></div><span v-if="loading" class="live-state"><i></i>生成中</span></div>
          <div class="output-grid" :class="{ single: outputs.length === 1 }">
            <article v-for="item in outputs" :key="item.id" class="output-card" :class="item.status">
              <GenerationLoader v-if="item.status === 'queued' || item.status === 'running'" kind="image" :title="item.status === 'queued' ? '等待执行' : '正在显影'" :detail="item.status === 'queued' && item.position ? `队列前方 ${Math.max(0, item.position - 1)} 个任务` : `正在构建画面 · 任务 ${String(item.index + 1).padStart(2, '0')}`" />
              <button v-else-if="item.status === 'success'" type="button" class="result-thumb" :aria-label="`预览生成结果 ${item.index + 1}`" @click="previewOutput = item">
                <img :src="outputUrl(item)" :alt="`生成结果 ${item.index + 1}`" />
                <span class="success-mark"><IconCheck /></span>
              </button>
              <div v-else class="task-placeholder failed-state" :title="item.error"><span class="failed-mark"><IconCloseCircle /></span><strong>生成失败</strong><small>{{ item.error || '未扣费或已自动退款' }}</small></div>
              <footer><span>任务 {{ String(item.index + 1).padStart(2, '0') }}</span><strong v-if="item.status === 'success'">{{ item.data?.elapsed_ms ? `${(item.data.elapsed_ms / 1000).toFixed(1)}s` : '已完成' }}</strong><strong v-else-if="item.status === 'failed'">未扣费 / 已退款</strong><strong v-else>处理中</strong></footer>
            </article>
          </div>
        </div>
        <MediaPreview :visible="!!previewOutput" :src="outputUrl(previewOutput)" kind="image" :filename="`generated-${(previewOutput?.index || 0) + 1}`" downloadable @close="previewOutput = null" />
      </section>
    </div>

    <div v-else class="chat-studio">
      <div ref="conversation" class="image-conversation" aria-live="polite">
        <div v-if="!turns.length" class="chat-empty">
          <span><IconImage /></span>
          <h3>从一段画面描述开始</h3>
          <p>模型会在这里持续呈现每一轮创作，便于对照不同参数和结果。</p>
        </div>
        <article v-for="turn in turns" :key="turn.id" class="image-turn">
          <div class="prompt-message">
            <span class="message-avatar"><IconImage /></span>
            <div><p>{{ turn.prompt }}</p><small>{{ formatTime(turn.createdAt) }}</small></div>
          </div>
          <div class="assistant-result">
            <span class="assistant-avatar">25</span>
            <div class="turn-body">
              <div class="turn-meta">
                <div><strong>{{ turn.model }}</strong><span>{{ turn.ratio }} · {{ turn.resolution }} · {{ turn.quantity }} 张</span></div>
                <div class="turn-side"><span class="turn-cost">预计消耗 <strong>{{ Number(turn.cost || 0).toFixed(2) }}</strong> 额度</span><span class="turn-actions"><button type="button" title="下载本轮图片" aria-label="下载本轮图片" @click="downloadTurn(turn)"><IconDownload /></button><button type="button" title="删除整轮对话" aria-label="删除整轮对话" @click="removeTurn(turn)"><IconDelete /></button></span></div>
              </div>
              <div class="chat-output-grid" :class="{ single: turn.tasks.length === 1 }">
                <article v-for="item in turn.tasks" :key="item.id" class="chat-output" :class="item.status">
                  <GenerationLoader v-if="item.status === 'queued' || item.status === 'running'" kind="image" :title="item.status === 'queued' ? '等待执行' : '正在显影'" :detail="item.status === 'queued' && item.position ? `队列前方 ${Math.max(0, item.position - 1)} 个任务` : `正在构建画面 · 任务 ${String(item.index + 1).padStart(2, '0')}`" />
                  <div v-else-if="item.status === 'success'" class="chat-result-frame">
                    <button type="button" class="chat-result-thumb" :aria-label="`预览生成结果 ${item.index + 1}`" @click="previewOutput = item"><img :src="outputUrl(item)" :alt="`生成结果 ${item.index + 1}`" /></button>
                    <span class="chat-result-actions"><button type="button" title="下载图片" :aria-label="`下载生成结果 ${item.index + 1}`" @click="downloadImage(item, item.index)"><IconDownload /></button><button type="button" title="删除图片" :aria-label="`删除生成结果 ${item.index + 1}`" @click="removeImageTask(turn, item)"><IconDelete /></button></span>
                  </div>
                  <div v-else class="chat-task-state chat-failed" :title="item.error">
                    <span><IconCloseCircle /></span><strong>生成失败 · 已退款</strong><small>{{ item.error || '任务未扣费或额度已自动退回' }}</small>
                  </div>
                </article>
              </div>
            </div>
          </div>
        </article>
      </div>

      <section class="chat-composer">
        <div v-if="form.reference_images.length" class="chat-references">
          <figure v-for="(item, index) in form.reference_images" :key="`${index}-${item.length}`">
            <img :src="item" :alt="`参考图 ${index + 1}`" />
            <button type="button" :aria-label="`移除参考图 ${index + 1}`" @click="removeReference(index)"><IconDelete /></button>
          </figure>
        </div>
        <a-textarea v-model="form.prompt" class="chat-prompt-input" :max-length="2000" :auto-size="false" :rows="3" placeholder="描述你想创作的画面；可拖动增高，双击打开大输入框" @dblclick="promptEditorOpen = true" @keydown.ctrl.enter.prevent="generate" />
        <div class="chat-composer-foot">
          <div class="chat-tools">
            <div class="chat-model-control"><IconImage /><a-select v-model="form.model" :disabled="loading" :placeholder="available.length ? '选择模型' : '暂无模型'"><a-option v-for="m in available" :key="m.id" :value="publicModelID(m)" :label="m.alias || m.name || m.id" /></a-select></div>
            <button type="button" class="chat-tool-button" :disabled="loading" @click="chooseReference"><IconUpload /><span>{{ form.reference_images.length ? `${form.reference_images.length}/${referenceLimit}` : '参考图' }}</span></button>
            <div class="chat-parameter"><span class="ratio-glyph" :style="{ aspectRatio: form.ratio.replace(':', ' / ') }"></span><a-select v-model="form.ratio" :disabled="loading"><a-option v-for="x in selected?.ratios || ['1:1']" :key="x" :value="x">{{ x }}</a-option></a-select></div>
            <div class="chat-parameter"><IconImage /><a-select v-model="form.resolution" :disabled="loading"><a-option v-for="x in Object.keys(selected?.prices || {})" :key="x" :value="x">{{ x }}</a-option></a-select></div>
            <div class="chat-count" role="radiogroup" aria-label="生成张数"><button v-for="value in [1, 2, 3, 4]" :key="value" type="button" :class="{ active: quantity === value }" :aria-checked="quantity === value" role="radio" :disabled="loading" @click="quantity = value">{{ value }}</button></div>
            <label v-if="deaiConfig.enabled" class="chat-deai"><a-switch v-model="form.deai" size="small" :disabled="loading" /><span>去 AI 特征</span></label>
          </div>
          <div class="chat-submit">
            <button type="button" :disabled="loading || !available.length" @click="generate"><IconLoading v-if="loading" /><IconSend v-else /><span>{{ loading ? `${completedCount}/${outputs.length || taskCount}` : '开始创作' }}</span><i></i><strong>{{ totalPrice }} 额度</strong></button>
          </div>
        </div>
      </section>
      <MediaPreview :visible="!!previewOutput" :src="outputUrl(previewOutput)" kind="image" :filename="`generated-${(previewOutput?.index || 0) + 1}`" downloadable @close="previewOutput = null" />
    </div>

    <a-modal v-model:visible="promptEditorOpen" title="编辑生成描述" modal-class="prompt-editor-modal" width="760px" :mask-closable="false">
      <div class="prompt-editor">
        <p>完整描述主体、环境、构图、光线、材质与风格，编辑内容会实时同步到当前创作模式。</p>
        <a-textarea v-model="form.prompt" :max-length="2000" :auto-size="false" :rows="14" show-word-limit placeholder="在这里输入完整的画面描述" @keydown.ctrl.enter.prevent="promptEditorOpen = false" />
        <span>Ctrl + Enter 完成编辑</span>
      </div>
      <template #footer><a-button type="primary" @click="promptEditorOpen = false">完成编辑</a-button></template>
    </a-modal>

    <a-modal v-if="modeChooserOpen" :visible="modeChooserOpen" :footer="false" :closable="false" :mask-closable="false" :esc-to-close="false" modal-class="mode-chooser-modal" width="680px">
      <div class="mode-chooser">
        <div class="chooser-heading"><span>图片创作</span><h3>选择你的创作方式</h3><p>两种模式共用同一套模型、额度和生成记录，随时可以在右上角切换。</p></div>
        <div class="mode-options" role="radiogroup" aria-label="选择图片创作方式">
          <button type="button" :class="{ selected: selectedChooserMode === 'studio' }" :aria-checked="selectedChooserMode === 'studio'" role="radio" @click="chooseMode('studio')">
            <span class="choice-icon"><IconApps /></span><strong>工作台模式</strong><small>参数与预览左右并列，适合集中调试单轮作品。</small>
            <i class="studio-sketch"><b></b><em></em></i>
          </button>
          <button type="button" :class="{ selected: selectedChooserMode === 'chat' }" :aria-checked="selectedChooserMode === 'chat'" role="radio" @click="chooseMode('chat')">
            <span class="choice-icon"><IconMessage /></span><strong>对话模式</strong><small>保留连续创作上下文，适合比较多轮灵感与结果。</small>
            <i class="chat-sketch"><b></b><em></em><b></b></i>
          </button>
        </div>
        <div class="chooser-footer"><a-checkbox v-model="rememberMode">记住我的选择</a-checkbox><a-button type="primary" size="large" @click="enterSelectedMode">进入图片创作</a-button></div>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
.billing-state{font-size:11px;color:var(--ns-accent-strong);display:flex;align-items:center;gap:7px}.billing-state i{width:7px;height:7px;border-radius:50%;background:var(--ns-accent)}.mode-tabs{display:inline-flex;padding:3px;background:#e8eae4;border-radius:7px;margin-bottom:16px}.mode-tabs button{height:34px;padding:0 17px;border:0;border-radius:5px;background:transparent;color:var(--ns-ink-soft);display:flex;align-items:center;gap:8px;cursor:pointer}.mode-tabs button.active{background:#fff;color:var(--ns-ink);box-shadow:0 1px 4px rgba(0,0,0,.08)}.studio{display:grid;grid-template-columns:minmax(430px,.95fr) minmax(440px,1.25fr);min-height:650px;border:1px solid var(--ns-line);background:#fff;border-radius:var(--ns-radius);overflow:hidden}.composer{padding:26px;border-right:1px solid var(--ns-line);display:flex;flex-direction:column}.field{display:flex;flex-direction:column;gap:8px;margin-bottom:20px}.field label{font-size:12px;font-weight:620}.label-row{display:flex;justify-content:space-between}.label-row span{font-size:10px;color:var(--ns-ink-faint)}.parameter-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}.parameter-grid .field{margin-bottom:18px}.reference{min-height:70px;border:1px dashed var(--ns-line-strong);background:#fafaf7;border-radius:6px;display:flex;align-items:center;justify-content:center;gap:12px;color:var(--ns-ink-soft);cursor:pointer}.reference span{display:flex;flex-direction:column;text-align:left}.reference strong{font-size:12px;color:var(--ns-ink)}.reference small{font-size:10px;margin-top:4px}.submit-row{margin-top:auto;padding-top:24px;display:flex;align-items:center;justify-content:space-between}.submit-row>span{font-size:12px;color:var(--ns-ink-soft)}.submit-row strong{color:var(--ns-ink)}.preview{padding:28px;display:grid;place-items:center;background:#f0f1ed;position:relative;min-width:0}.preview.hasResult{padding:16px;background:#222722}.preview img{width:100%;height:100%;max-height:620px;object-fit:contain}.empty{text-align:center;max-width:310px}.empty h3{font-size:14px;margin:16px 0 7px}.empty p{font-size:12px;line-height:1.6;color:var(--ns-ink-faint)}.preview-mark{width:48px;height:48px;display:grid;place-items:center;border:1px solid var(--ns-line-strong);border-radius:50%;margin:auto;color:var(--ns-ink-faint)}.loading-mark{display:block;width:34px;height:34px;border:2px solid #ccd2c8;border-top-color:var(--ns-accent-strong);border-radius:50%;margin:auto;animation:spin .8s linear infinite}.result-meta{position:absolute;left:26px;right:26px;bottom:25px;display:flex;justify-content:space-between;color:white;font-size:11px;text-shadow:0 1px 4px #000}.result-meta span{display:flex;align-items:center;gap:7px}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:1050px){.studio{grid-template-columns:1fr}.composer{border-right:0;border-bottom:1px solid var(--ns-line)}.preview{min-height:480px}}@media(max-width:560px){.composer{padding:18px}.parameter-grid{grid-template-columns:1fr 1fr}.studio{min-height:0}.preview{min-height:360px}.submit-row{align-items:end}}
.deai-row{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-top:14px;padding:13px 14px;border:1px solid #dfd6aa;border-radius:7px;background:#faf8ed}.deai-row>span{display:flex;flex-direction:column}.deai-row strong{font-size:11px}.deai-row small{margin-top:4px;color:var(--ns-ink-soft);font-size:9px}
.reference-strip{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:8px;margin-top:10px}.reference-strip figure{aspect-ratio:1;margin:0;position:relative;overflow:hidden;border:1px solid var(--ns-line);border-radius:6px;background:#edf0ea}.reference-strip img{width:100%;height:100%;object-fit:cover}.reference-strip button{width:24px;height:24px;padding:0;position:absolute;top:5px;right:5px;display:grid;place-items:center;border:0;border-radius:50%;background:rgba(28,32,29,.78);color:#fff;cursor:pointer}.reference-strip span{position:absolute;left:6px;bottom:5px;padding:2px 5px;border-radius:999px;background:rgba(28,32,29,.72);color:#fff;font-size:8px}
.quantity-control{height:34px;display:grid;grid-template-columns:repeat(4,1fr);padding:3px;border:1px solid var(--ns-line);border-radius:6px;background:#f2f3ef}.quantity-control button{min-width:0;border:0;border-radius:4px;background:transparent;color:var(--ns-ink-soft);font-size:11px;font-weight:650;cursor:pointer;transition:background .18s ease,color .18s ease,box-shadow .18s ease}.quantity-control button:hover{color:var(--ns-ink);background:#e7e9e4}.quantity-control button.active{background:#242a25;color:#fff;box-shadow:0 3px 8px rgba(31,36,33,.18)}
.preview.has-results{display:block;padding:24px;background:#f0f1ed}.output-wrap{width:100%;height:100%;display:flex;flex-direction:column}.output-summary{min-height:44px;margin-bottom:18px;display:flex;align-items:flex-start;justify-content:space-between;gap:16px}.output-summary>div{display:flex;flex-direction:column;gap:5px}.output-summary strong{font-size:13px}.output-summary span{font-size:10px;color:var(--ns-ink-faint)}.live-state{display:flex;align-items:center;gap:7px!important;color:var(--ns-accent-strong)!important}.live-state i{width:7px;height:7px;border-radius:50%;background:var(--ns-accent);animation:status-pulse 1.4s ease-in-out infinite}.output-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;align-content:start}.output-card{min-width:0;overflow:hidden;border:1px solid #dfe1da;border-radius:8px;background:#fff;box-shadow:0 5px 16px rgba(31,36,33,.05)}.task-placeholder,.result-thumb{width:100%;aspect-ratio:1;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px}.task-placeholder{background:#e9ebe6;color:var(--ns-ink-soft)}.task-placeholder strong{font-size:11px}.task-placeholder small{max-width:82%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--ns-ink-faint);font-size:9px}.task-spinner{width:32px;height:32px;display:grid;place-items:center;border:1px solid #cfd4ca;border-radius:50%;background:#f7f8f5;color:#5f7059}.task-spinner :deep(svg){width:16px;height:16px;animation:spin .9s linear infinite}.result-thumb{position:relative;padding:0;border:0;background:#e7e9e4;overflow:hidden;cursor:zoom-in}.result-thumb img,.result-thumb video{width:100%;height:100%;max-height:none;display:block;object-fit:cover;transition:transform .32s ease}.result-thumb:hover img,.result-thumb:hover video{transform:scale(1.035)}.success-mark{width:24px;height:24px;position:absolute;top:8px;right:8px;display:grid;place-items:center;border-radius:50%;background:rgba(38,47,39,.82);color:#fff}.success-mark :deep(svg){width:13px;height:13px}.failed-state{padding:14px;background:#f3eeeb;color:#705248}.failed-mark{width:32px;height:32px;display:grid;place-items:center;border-radius:50%;background:#e8dcd7;color:#895f52}.failed-state small{max-width:100%}.output-card footer{height:38px;padding:0 10px;display:flex;align-items:center;justify-content:space-between;gap:8px;border-top:1px solid #e5e7e1}.output-card footer span,.output-card footer strong{font-size:9px}.output-card footer span{color:var(--ns-ink-faint)}.output-card footer strong{color:var(--ns-ink-soft);white-space:nowrap}@keyframes status-pulse{0%,100%{opacity:.45;transform:scale(.84)}50%{opacity:1;transform:scale(1)}}
@media(max-width:560px){.preview.has-results{padding:16px}.output-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:9px}.output-summary{margin-bottom:13px}}
@media(prefers-reduced-motion:reduce){.live-state i,.task-spinner :deep(svg){animation:none}.result-thumb img,.result-thumb video{transition:none}}
.image-heading{align-items:center}.heading-actions{display:flex;align-items:center;gap:16px}.mode-switch{height:38px;padding:3px;display:flex;align-items:center;border:1px solid #d8dcd4;border-radius:999px;background:#e9ece6}.mode-switch button{height:30px;padding:0 13px;display:flex;align-items:center;gap:7px;border:0;border-radius:999px;background:transparent;color:#667064;font-size:10px;font-weight:650;cursor:pointer;transition:background .18s ease,color .18s ease,box-shadow .18s ease}.mode-switch button :deep(svg){width:14px;height:14px}.mode-switch button.active{background:#252d27;color:#fff;box-shadow:0 2px 7px rgba(30,37,32,.18)}
.chat-studio{min-height:680px;display:flex;flex-direction:column;overflow:hidden;border:1px solid var(--ns-line);border-radius:8px;background:#f4f5f1}.image-conversation{height:min(58vh,670px);min-height:430px;overflow-y:auto;padding:30px clamp(22px,5vw,74px) 40px;scroll-behavior:auto}.image-conversation::-webkit-scrollbar{width:5px}.image-conversation::-webkit-scrollbar-thumb{border-radius:999px;background:#cbd0c8}.chat-empty{min-height:330px;display:grid;place-items:center;align-content:center;text-align:center;color:var(--ns-ink-faint)}.chat-empty>span{width:52px;height:52px;display:grid;place-items:center;border:1px solid #d7dbd3;border-radius:50%;background:#fff;color:#65725f}.chat-empty h3{margin:16px 0 7px;color:var(--ns-ink);font-size:15px;letter-spacing:0}.chat-empty p{max-width:360px;margin:0;font-size:11px;line-height:1.65}.image-turn{max-width:980px;margin:0 auto;padding:0 0 38px}.image-turn:last-child{padding-bottom:8px}.prompt-message{max-width:78%;margin:0 0 22px auto;display:flex;flex-direction:row-reverse;align-items:flex-start;gap:10px}.message-avatar,.assistant-avatar{width:31px;height:31px;flex:0 0 31px;display:grid;place-items:center;border-radius:50%}.message-avatar{background:#dfe3db;color:#586454}.message-avatar :deep(svg){width:14px;height:14px}.prompt-message>div{min-width:0;padding:12px 14px;border-radius:12px 4px 12px 12px;background:#252d27;color:#fff;box-shadow:0 6px 16px rgba(31,38,33,.09)}.prompt-message p{margin:0;white-space:pre-wrap;word-break:break-word;font-size:12px;line-height:1.65}.prompt-message small{display:block;margin-top:7px;color:rgba(255,255,255,.52);font-size:8px;text-align:right}.assistant-result{display:flex;align-items:flex-start;gap:11px}.assistant-avatar{background:#e7d36c;color:#31382f;font-size:9px;font-weight:750}.turn-body{min-width:0;flex:1}.turn-meta{min-height:34px;margin-bottom:11px;display:flex;align-items:flex-start;justify-content:space-between;gap:16px}.turn-meta>div{min-width:0;display:flex;flex-direction:column;gap:4px}.turn-meta strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:11px}.turn-meta span,.turn-meta small{color:var(--ns-ink-faint);font-size:9px}.chat-output-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.chat-output-grid.single{grid-template-columns:minmax(230px,420px)}.chat-output{min-width:0;aspect-ratio:1;overflow:hidden;border:1px solid #dce0d8;border-radius:7px;background:#e9ece6}.chat-task-state,.chat-result-thumb{width:100%;height:100%;min-height:160px}.chat-task-state{padding:16px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px;color:var(--ns-ink-soft)}.chat-task-state strong{font-size:10px}.chat-task-state small{max-width:90%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--ns-ink-faint);font-size:8px}.chat-result-thumb{padding:0;position:relative;display:block;overflow:hidden;border:0;background:#e9ece6;cursor:zoom-in}.chat-result-thumb img{width:100%;height:100%;display:block;object-fit:cover;transition:transform .3s ease}.chat-result-thumb:hover img{transform:scale(1.03)}.chat-result-thumb>span{width:25px;height:25px;position:absolute;top:8px;right:8px;display:grid;place-items:center;border-radius:50%;background:rgba(34,42,36,.8);color:#fff}.chat-result-thumb>span :deep(svg){width:12px}.chat-failed{background:#f4e9e5;color:#80564b}.chat-failed>span{width:30px;height:30px;display:grid;place-items:center;border-radius:50%;background:#ead9d3}
.chat-composer{margin:0 clamp(16px,4vw,58px) 22px;padding:14px 16px 13px;border:1px solid #d8dcd4;border-radius:14px;background:#fff;box-shadow:0 12px 32px rgba(31,39,33,.09)}.chat-composer :deep(.arco-textarea-wrapper){padding:0 2px 12px!important;border:0!important;background:transparent!important;box-shadow:none!important}.chat-composer :deep(textarea){padding:0!important;border:0!important;background:transparent!important;box-shadow:none!important;color:var(--ns-ink);font-size:12px;line-height:1.7;resize:none}.chat-references{display:flex;gap:8px;padding:0 0 11px;overflow-x:auto}.chat-references figure{width:58px;height:52px;flex:0 0 58px;position:relative;margin:0;overflow:hidden;border:1px solid var(--ns-line);border-radius:7px;background:#eef0eb}.chat-references img{width:100%;height:100%;object-fit:cover}.chat-references button{width:19px;height:19px;padding:0;position:absolute;top:3px;right:3px;display:grid;place-items:center;border:0;border-radius:50%;background:rgba(28,33,29,.8);color:#fff;cursor:pointer}.chat-references button :deep(svg){width:10px}.chat-composer-foot{display:flex;align-items:center;justify-content:space-between;gap:12px;padding-top:11px;border-top:1px solid #eceee9}.chat-tools{min-width:0;display:flex;align-items:center;gap:6px;flex-wrap:wrap}.chat-model-control,.chat-parameter,.chat-tool-button,.chat-count,.chat-deai{height:31px;border-radius:999px;background:#f0f2ed;color:#63705e}.chat-model-control{width:180px;display:flex;align-items:center;overflow:hidden;background:#e9ede6}.chat-model-control>svg,.chat-parameter>svg{width:14px;height:14px;flex:0 0 14px;margin-left:10px}.chat-model-control :deep(.arco-select),.chat-parameter :deep(.arco-select){min-width:0;flex:1}.chat-model-control :deep(.arco-select-view),.chat-parameter :deep(.arco-select-view){height:31px!important;padding:0 8px 0 6px!important;border:0!important;background:transparent!important;box-shadow:none!important;font-size:10px;font-weight:600}.chat-model-control :deep(.arco-select-view-arrow-icon),.chat-parameter :deep(.arco-select-view-arrow-icon){display:none}.chat-model-control :deep(.arco-select-view-value){overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.chat-tool-button{padding:0 11px;display:flex;align-items:center;gap:6px;border:0;font-size:10px;cursor:pointer}.chat-tool-button :deep(svg){width:13px}.chat-tool-button:disabled{opacity:.5;cursor:not-allowed}.chat-parameter{width:88px;display:flex;align-items:center;overflow:hidden}.chat-parameter .ratio-glyph{width:14px;min-width:7px;max-height:14px;display:block;flex:0 0 14px;margin-left:10px;border:1.5px solid currentColor;border-radius:2px}.chat-count{padding:3px;display:flex;align-items:center}.chat-count button{min-width:25px;height:25px;padding:0 7px;border:0;border-radius:999px;background:transparent;color:var(--ns-ink-soft);font-size:9px;font-weight:650;cursor:pointer}.chat-count button.active{background:#273029;color:#fff}.chat-count button:disabled{cursor:not-allowed}.chat-deai{padding:0 10px;display:flex;align-items:center;gap:6px;font-size:9px;cursor:pointer}.chat-submit{display:flex;align-items:center;white-space:nowrap}.chat-submit button{height:34px;min-width:154px;padding:0 15px;display:flex;align-items:center;justify-content:center;gap:7px;border:0;border-radius:999px;background:#273029;color:#fff;font-size:10px;font-weight:650;cursor:pointer}.chat-submit button:hover{background:#354139}.chat-submit button:disabled{opacity:.5;cursor:not-allowed}.chat-submit button :deep(svg){width:13px}.chat-submit button :deep(.arco-icon-loading){animation:spin .9s linear infinite}.chat-submit button i{width:1px;height:13px;margin:0 2px;background:rgba(255,255,255,.2)}.chat-submit button strong{color:#f1dc72;font-size:10px;font-weight:750}
.clear-screen{height:32px;padding:0 12px;display:flex;align-items:center;gap:6px;border:1px solid #d8ddd4;border-radius:999px;background:#fff;color:#65705f;font-size:10px;font-weight:650;cursor:pointer}.clear-screen:hover{border-color:#aeb8a9;background:#f4f6f1;color:#273029}.clear-screen :deep(svg){width:13px;height:13px}.turn-meta>.turn-side{align-items:flex-end;gap:7px}.turn-cost{padding:6px 9px;border:1px solid #e5d78d;border-radius:999px;background:#faf5d9!important;color:#76631f!important;font-size:9px!important;white-space:nowrap}.turn-cost strong{color:#403a20;font-size:11px}.turn-actions{display:flex;gap:5px}.turn-actions button,.chat-result-actions button{width:27px;height:27px;padding:0;display:grid;place-items:center;border:1px solid #d8ddd4;border-radius:50%;background:#fff;color:#64705f;cursor:pointer}.turn-actions button:hover{border-color:#aeb7aa;color:#263029}.turn-actions button :deep(svg),.chat-result-actions button :deep(svg){width:12px;height:12px}.chat-result-frame{width:100%;height:100%;min-height:160px;position:relative}.chat-result-frame>.chat-result-thumb{position:absolute;inset:0}.chat-result-actions{position:absolute;z-index:2;top:8px;right:8px;display:flex;gap:5px;opacity:0;transform:translateY(-2px);transition:opacity .18s ease,transform .18s ease}.chat-result-frame:hover .chat-result-actions,.chat-result-actions:focus-within{opacity:1;transform:none}.chat-result-actions button{border-color:rgba(255,255,255,.14);background:rgba(27,33,29,.78);color:#fff;backdrop-filter:blur(8px)}.chat-result-actions button:hover{background:rgba(27,33,29,.94)}
:global(.mode-chooser-modal){border-radius:10px!important;overflow:hidden}:global(.mode-chooser-modal .arco-modal-body){padding:0!important}.mode-chooser{padding:30px}.chooser-heading>span{display:inline-flex;padding:5px 9px;border-radius:999px;background:#f1e7a8;color:#5f5528;font-size:9px;font-weight:700}.chooser-heading h3{margin:13px 0 7px;font-size:22px;letter-spacing:0;color:var(--ns-ink)}.chooser-heading p{margin:0;color:var(--ns-ink-faint);font-size:11px;line-height:1.65}.mode-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin:24px 0 21px}.mode-options>button{min-height:210px;padding:18px;position:relative;overflow:hidden;display:flex;align-items:flex-start;flex-direction:column;border:1px solid #dde0da;border-radius:8px;background:#f7f8f5;color:var(--ns-ink);text-align:left;cursor:pointer;transition:border-color .18s ease,background .18s ease,box-shadow .18s ease}.mode-options>button:hover{border-color:#aeb7aa}.mode-options>button.selected{border-color:#65735d;background:#f0f3ed;box-shadow:inset 0 0 0 1px #65735d}.choice-icon{width:34px;height:34px;display:grid;place-items:center;border-radius:7px;background:#273029;color:#fff}.choice-icon :deep(svg){width:16px;height:16px}.mode-options strong{margin:13px 0 6px;font-size:13px}.mode-options small{max-width:92%;color:var(--ns-ink-faint);font-size:9px;line-height:1.55}.studio-sketch,.chat-sketch{height:50px;margin-top:auto;width:100%;display:flex;padding:7px;border:1px solid #d9ddd6;border-radius:5px;background:#fff}.studio-sketch b,.studio-sketch em,.chat-sketch b,.chat-sketch em{display:block;border-radius:3px;background:#dfe4dc}.studio-sketch b{width:34%;margin-right:6px;background:#e7d36c}.studio-sketch em{flex:1}.chat-sketch{flex-direction:column;gap:5px}.chat-sketch b{width:48%;height:8px;margin-left:auto}.chat-sketch b:last-child{width:36%;margin:0}.chat-sketch em{width:68%;height:14px;background:#cbd4c7}.chooser-footer{display:flex;align-items:center;justify-content:space-between;gap:16px;padding-top:18px;border-top:1px solid #e8eae5}.chooser-footer :deep(.arco-btn-primary){min-width:142px;border-radius:999px;background:#273029;border-color:#273029}
@media(max-width:900px){.heading-actions{gap:10px}.billing-state{display:none}.image-conversation{padding-inline:24px}.chat-composer{margin-inline:18px}.chat-composer-foot{align-items:flex-end}.chat-submit>span{display:none}}
@media(max-width:640px){.image-heading{align-items:flex-start}.image-heading>div:first-child p{max-width:220px}.mode-switch button{width:34px;padding:0;justify-content:center}.mode-switch button span{display:none}.chat-studio{min-height:620px}.image-conversation{height:54vh;min-height:360px;padding:20px 12px 28px}.prompt-message{max-width:90%}.assistant-result{gap:8px}.assistant-avatar,.message-avatar{width:28px;height:28px;flex-basis:28px}.chat-output-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:7px}.chat-output-grid.single{grid-template-columns:minmax(0,1fr)}.chat-composer{margin:0 9px 9px;padding:12px;border-radius:12px}.chat-composer-foot{display:block}.chat-tools{gap:5px}.chat-model-control{width:190px}.chat-parameter{width:78px}.chat-deai{padding-inline:8px}.chat-submit{margin-top:10px}.chat-submit button{width:100%;height:35px}.mode-chooser{padding:22px}.mode-options{grid-template-columns:1fr;margin-block:18px}.mode-options>button{min-height:168px}.chooser-footer{align-items:stretch;flex-direction:column}.chooser-footer :deep(.arco-btn){width:100%}}
@media(max-width:640px){.clear-screen{width:34px;padding:0;justify-content:center}.clear-screen span{display:none}.turn-meta{align-items:flex-start}.turn-meta>.turn-side{align-items:flex-end}.turn-cost{font-size:8px!important}.chat-result-actions{opacity:1;transform:none}.chat-result-actions button{width:25px;height:25px}}
@media(prefers-reduced-motion:reduce){.image-conversation{scroll-behavior:auto}.chat-result-thumb img,.mode-options>button{transition:none}}
.studio-submit{min-width:184px}.studio-submit i{width:1px;height:14px;margin:0 2px;background:rgba(255,255,255,.22)}.studio-submit strong{color:#f1dc72;font-size:11px;font-weight:750}
.studio-prompt-input :deep(textarea){min-height:180px;resize:vertical}.chat-composer .chat-prompt-input :deep(textarea){min-height:58px;max-height:220px;resize:vertical}.prompt-editor p{margin:0 0 14px;color:var(--ns-ink-soft);font-size:11px;line-height:1.65}.prompt-editor>span{display:block;margin-top:8px;color:var(--ns-ink-faint);font-size:9px;text-align:right}.prompt-editor :deep(textarea){min-height:320px;resize:vertical;line-height:1.75}:global(.prompt-editor-modal){border-radius:8px!important;overflow:hidden}:global(.prompt-editor-modal .arco-modal-title){font-size:15px;font-weight:700}:global(.prompt-editor-modal .arco-modal-footer){display:flex;justify-content:flex-end}:global(.prompt-editor-modal .arco-modal-footer .arco-btn-primary){min-width:112px;border-radius:999px!important}
@media(max-width:640px){:global(.prompt-editor-modal){width:calc(100vw - 24px)!important}.prompt-editor :deep(textarea){min-height:48vh}}
</style>

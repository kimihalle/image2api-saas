<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import {
  IconCheck,
  IconClockCircle,
  IconClose,
  IconComputer,
  IconDelete,
  IconDownload,
  IconExclamationCircle,
  IconImage,
  IconLoading,
  IconMusic,
  IconUpload,
  IconVideoCamera,
} from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../../services/api'
import { useAuthStore } from '../../stores/auth'

type MediaKind = 'images' | 'videos' | 'audios'
type VideoTask = {
  id: string
  status: string
  progress: number
  error?: string
  url?: string
  cost: number
  model: string
  ratio: string
  resolution: string
  duration: string
}
type CreationTurn = {
  id: string
  prompt: string
  createdAt: number
  tasks: VideoTask[]
  historical?: boolean
}

const auth = useAuthStore()
const models = ref<any[]>([])
const generatedImages = ref<any[]>([])
const turns = ref<CreationTurn[]>([])
const pickerOpen = ref(false)
const creating = ref(false)
const loadingHistory = ref(true)
const composer = ref<HTMLElement>()
const fileInput = ref<HTMLInputElement>()
const objectURLs = new Set<string>()
const form = reactive({
  model: '',
  prompt: '',
  ratio: '',
  resolution: '',
  duration: '',
  reference_mode: 'all',
  count: 1,
  images: [] as string[],
  videos: [] as string[],
  audios: [] as string[],
})

const selected = computed(() => models.value.find((item) => item.id === form.model || publicModelID(item) === form.model))
const caps = computed<Record<string, any>>(() => selected.value?.capabilities || {})
const ratios = computed<string[]>(() => stringList(selected.value?.ratios || caps.value.ratios))
const resolutions = computed<string[]>(() => stringList(selected.value?.resolutions || caps.value.resolutions))
const durations = computed<string[]>(() => stringList(selected.value?.durations || caps.value.durations).map(normalizeDuration))
const countOptions = computed<number[]>(() => {
  const values = numberList(caps.value.concurrency_options).filter((value) => value > 0)
  return values.length ? values : [1]
})
const maxPromptLength = computed(() => positiveInt(caps.value.max_prompt_length, 2000))
const mediaLimits = computed(() => ({
  images: nonNegativeInt(caps.value.max_images, nonNegativeInt(selected.value?.max_reference_images, 0)),
  videos: nonNegativeInt(caps.value.max_videos, 0),
  audios: nonNegativeInt(caps.value.max_audios, 0),
}))
const minImages = computed(() => nonNegativeInt(caps.value.min_images, 0))
const maxMediaFiles = computed(() => nonNegativeInt(caps.value.max_media_files, 0))
const mediaCount = computed(() => form.images.length + form.videos.length + form.audios.length)
const modelName = computed(() => selected.value ? publicModelName(selected.value) : '选择模型')
function ratioIconStyle(value: string) {
  const match = String(value || '').match(/(\d+(?:\.\d+)?)\s*:\s*(\d+(?:\.\d+)?)/)
  if (!match) return { width: '14px', height: '10px' }
  const width = Number(match[1])
  const height = Number(match[2])
  if (!width || !height) return { width: '14px', height: '10px' }
  const scale = 14 / Math.max(width, height)
  return {
    width: `${Math.max(7, Math.round(width * scale))}px`,
    height: `${Math.max(7, Math.round(height * scale))}px`,
  }
}
const price = computed(() => {
  const model = selected.value
  if (!model) return 0
  const base = Number(model.prices?.[form.resolution] || 0)
  const durationPrice = model.duration_prices || {}
  const seconds = Number.parseInt(form.duration) || 0
  const timed = durationPrice.per_second != null
    ? Number(durationPrice.per_second) * seconds
    : Number(durationPrice[form.duration] || 0)
  return Number((base + timed).toFixed(4))
})
const totalPrice = computed(() => Number((price.value * form.count).toFixed(4)))
const pendingCount = computed(() => turns.value.flatMap((turn) => turn.tasks).filter((task) => !isFinished(task.status)).length)

let pollTimer: number | undefined
let generationStream: EventSource | null = null

function applyStoredTemplate() {
  try {
    const raw = sessionStorage.getItem('creation_template_payload')
    if (!raw) return
    const payload = JSON.parse(raw)
    if (payload.media_type !== 'video' || Date.now() - Number(payload.stored_at || 0) > 10 * 60 * 1000) return
    const compatible = Array.isArray(payload.compatible_models) ? payload.compatible_models : []
    const minReferences = Number(payload.min_references || 0)
    const target = models.value.find((item) => {
      if (compatible.length && !compatible.includes(item.id) && !compatible.includes(publicModelID(item))) return false
      const maxImages = nonNegativeInt(item.capabilities?.max_images, nonNegativeInt(item.max_reference_images, 0))
      return payload.reference_mode !== 'required' || maxImages >= minReferences
    })
    if (target) form.model = publicModelID(target)
    applyCapabilities()
    form.prompt = String(payload.prompt || '')
    const ratio = stringList(payload.ratios).find((item) => ratios.value.includes(item))
    const resolution = stringList(payload.resolutions).find((item) => resolutions.value.includes(item))
    const duration = stringList(payload.durations).map(normalizeDuration).find((item) => durations.value.includes(item))
    if (ratio) form.ratio = ratio
    if (resolution) form.resolution = resolution
    if (duration) form.duration = duration
    sessionStorage.removeItem('creation_template_payload')
    Message.success(`已应用“${payload.title || '灵感模板'}”`)
  } catch { sessionStorage.removeItem('creation_template_payload') }
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.map(String).map((item) => item.trim()).filter(Boolean) : []
}
function numberList(value: unknown): number[] {
  if (!Array.isArray(value)) return []
  return [...new Set(value.map(Number).filter(Number.isFinite))].sort((a, b) => a - b)
}
function nonNegativeInt(value: unknown, fallback = 0) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? Math.floor(parsed) : fallback
}
function positiveInt(value: unknown, fallback: number) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : fallback
}
function normalizeDuration(value: unknown) {
  const text = String(value || '').trim()
  return text && !text.endsWith('s') && /^\d+$/.test(text) ? `${text}s` : text
}
function publicModelID(model: any) {
	return String(model?.alias || model?.id || '').trim()
}
function publicModelName(model: any) {
	return safeModelName(model.alias || model.name || model.id)
}
function safeModelName(value: unknown) {
	const cleaned = String(value || '').replace(/sanbao|三宝/gi, '').replace(/^\s*[-_:：·]+|[-_:：·]+\s*$/g, '').trim()
	return cleaned || '视频模型'
}
function safeError(value: unknown, fallback = '视频生成失败，请稍后重试') {
	const cleaned = String(value || '').replace(/sanbao|三宝/gi, '视频服务').trim()
	if (/INSUFFICIENT_CREDITS|credits exhausted|insufficient_credits/i.test(cleaned)) return '视频服务当前额度不足，请稍后重试'
	return cleaned || fallback
}
function isFinished(status: string) {
  return ['completed', 'failed', 'success'].includes(status)
}
function isPending(status: string) {
  return !isFinished(status)
}
function statusText(task: VideoTask) {
  if (task.status === 'completed' || task.status === 'success') return '生成完成'
  if (task.status === 'failed') return '失败 · 已退款'
  if (task.status === 'queued') return '排队中'
  return task.progress > 0 ? `生成中 ${task.progress}%` : '生成中'
}
function formatDate(timestamp: number) {
  if (!timestamp) return ''
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(timestamp * 1000))
}
function inputLimit(kind: MediaKind) {
  return mediaLimits.value[kind]
}
function canAddMedia(kind: MediaKind) {
  if (form[kind].length >= inputLimit(kind)) return false
  return maxMediaFiles.value <= 0 || mediaCount.value < maxMediaFiles.value
}
function sizeLimitMB(kind: MediaKind) {
  const key = kind === 'images' ? 'max_image_size_mb' : kind === 'videos' ? 'max_video_size_mb' : 'max_audio_size_mb'
  return positiveInt(caps.value[key], kind === 'images' ? 20 : 100)
}
function acceptFor(kind: MediaKind) {
  return kind === 'images' ? 'image/png,image/jpeg,image/webp' : kind === 'videos' ? 'video/*' : 'audio/*'
}
function mediaLabel(kind: MediaKind) {
  return kind === 'images' ? '图片' : kind === 'videos' ? '视频' : '音频'
}
function applyCapabilities() {
  const model = selected.value
  if (!model) return
  const capability = model.capabilities || {}
  form.ratio = ratios.value.includes(form.ratio) ? form.ratio : String(capability.default_ratio || ratios.value[0] || '')
  form.resolution = resolutions.value.includes(form.resolution) ? form.resolution : String(capability.default_resolution || resolutions.value[0] || '')
  const preferredDuration = capability.default_duration_seconds ? `${capability.default_duration_seconds}s` : durations.value[0]
  form.duration = durations.value.includes(form.duration) ? form.duration : String(preferredDuration || '')
  form.count = countOptions.value.includes(form.count) ? form.count : countOptions.value[0]
  ;(['images', 'videos', 'audios'] as MediaKind[]).forEach((kind) => form[kind].splice(inputLimit(kind)))
  if (maxMediaFiles.value > 0 && mediaCount.value > maxMediaFiles.value) {
    let excess = mediaCount.value - maxMediaFiles.value
    for (const kind of ['audios', 'videos', 'images'] as MediaKind[]) {
      if (!excess) break
      const removeCount = Math.min(excess, form[kind].length)
      form[kind].splice(form[kind].length - removeCount, removeCount)
      excess -= removeCount
    }
  }
}
watch(selected, applyCapabilities)

onMounted(async () => {
  const [modelResponse] = await Promise.all([api('/models'), loadHistory()])
  models.value = (modelResponse.data?.data || modelResponse.data || [])
    .filter((item: any) => item.type === 'video' && item.enabled !== false)
    .sort((a: any, b: any) => Number(b.weight || 0) - Number(a.weight || 0))
  form.model = publicModelID(models.value[0])
  applyCapabilities()
  applyStoredTemplate()
  connectGenerationStream()
  if (pendingCount.value) schedulePoll(800)
})

onBeforeUnmount(() => {
  if (pollTimer) window.clearTimeout(pollTimer)
  generationStream?.close()
  objectURLs.forEach((url) => URL.revokeObjectURL(url))
})

function connectGenerationStream() {
  generationStream?.close()
  generationStream = new EventSource('/admin/api/generation/events')
  generationStream.addEventListener('generation', (event: MessageEvent) => {
    try { applyStreamJob(JSON.parse(event.data)?.job) } catch { /* polling remains the fallback */ }
  })
  generationStream.onerror = () => {
    if (pendingCount.value) schedulePoll(3000)
  }
}

async function applyStreamJob(job: any) {
  if (!job?.id || job.kind !== 'video') return
  const task = turns.value.flatMap((turn) => turn.tasks).find((item) => item.id === String(job.id))
  if (!task) return
  task.status = String(job.status || task.status)
  task.progress = Number(job.progress || 0)
  task.error = job.error ? safeError(job.error?.message || job.error) : undefined
  if (task.status === 'completed') await loadTaskVideo(task)
  if (isFinished(task.status)) await auth.refreshUser()
}

async function loadHistory() {
  loadingHistory.value = true
  try {
    const response = await api('/logs?kind=video&source=user&limit=24')
    if (!response.ok) return
    const rows = Array.isArray(response.data?.data) ? response.data.data : []
    const grouped = new Map<string, CreationTurn>()
    for (const row of [...rows].reverse()) {
      const timestamp = Number(row.ts || row.created_at || 0)
      const key = [row.prompt, row.model, row.ratio, row.resolution, row.duration, Math.floor(timestamp / 12)].join('|')
      let turn = grouped.get(key)
      if (!turn) {
        turn = { id: `history-${row.id}`, prompt: row.prompt || '未填写创作描述', createdAt: timestamp, tasks: [], historical: true }
        grouped.set(key, turn)
      }
      turn.tasks.push({
        id: row.id,
        status: row.status === 'success' ? 'completed' : row.status === 'failed' ? 'failed' : 'queued',
        progress: row.status === 'success' ? 100 : 0,
		error: row.error ? safeError(row.error) : undefined,
        cost: Number(row.cost || 0),
		model: safeModelName(row.model),
        ratio: row.ratio || '—',
        resolution: row.resolution || '—',
        duration: row.duration || '—',
      })
    }
    turns.value = [...grouped.values()]
    await Promise.all(turns.value.flatMap((turn) => turn.tasks).filter((task) => task.status === 'completed').map(loadTaskVideo))
  } finally {
    loadingHistory.value = false
  }
}

function readAsDataURL(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

async function addFiles(event: Event, kind: MediaKind) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (!files.length) return
  const kindRoom = Math.max(0, inputLimit(kind) - form[kind].length)
  const totalRoom = maxMediaFiles.value > 0 ? Math.max(0, maxMediaFiles.value - mediaCount.value) : kindRoom
  const room = Math.min(kindRoom, totalRoom)
  if (!room) return Message.warning(`当前模型不支持更多${mediaLabel(kind)}素材`)
  const maxBytes = sizeLimitMB(kind) * 1024 * 1024
  const accepted = files.filter((file) => {
    if (file.size > maxBytes) {
      Message.warning(`${file.name} 超过当前模型的 ${sizeLimitMB(kind)}MB 限制`)
      return false
    }
    return true
  }).slice(0, room)
  form[kind].push(...await Promise.all(accepted.map(readAsDataURL)))
  if (accepted.length < files.length) Message.info(`已保留 ${accepted.length} 个符合模型能力的素材`)
}

async function openPicker() {
  if (!inputLimit('images') || (maxMediaFiles.value > 0 && mediaCount.value >= maxMediaFiles.value)) {
    return Message.warning('当前模型无法继续添加图片素材')
  }
  const response = await api('/my-images')
  generatedImages.value = response.data?.data || []
  pickerOpen.value = true
}

async function useGenerated(item: any) {
  if (form.images.length >= inputLimit('images')) return Message.warning(`当前模型最多使用 ${inputLimit('images')} 张图片`)
  try {
    const response = await fetch(imageUrl(item.url || item.file || item.image), { headers: auth.token ? { Authorization: `Bearer ${auth.token}` } : {} })
    if (!response.ok) throw new Error('load failed')
    const blob = await response.blob()
    if (blob.size > sizeLimitMB('images') * 1024 * 1024) return Message.warning(`这张图片超过当前模型的 ${sizeLimitMB('images')}MB 限制`)
    form.images.push(await readAsDataURL(new File([blob], 'generated.png', { type: blob.type || 'image/png' })))
    pickerOpen.value = false
  } catch {
    Message.error('读取已生成图片失败')
  }
}

function remove(kind: MediaKind, index: number) {
  form[kind].splice(index, 1)
}

function downloadTask(task: VideoTask, index: number) {
  if (!task.url) return Message.info('视频文件正在准备中')
  const link = document.createElement('a')
  link.href = task.url
  link.download = `video-${String(index + 1).padStart(2, '0')}.mp4`
  link.click()
}

function clearConversation() {
  const active = turns.value.filter((turn) => turn.tasks.some((task) => isPending(task.status)))
  turns.value = active
  Message.success(active.length ? '已清除完成的对话，进行中的任务已保留' : '当前对话已清屏')
}

async function removeTask(turn: CreationTurn, task: VideoTask) {
  const response = await api(`/video/jobs/${task.id}`, { method: 'DELETE' })
  if (!response.ok) return Message.error(response.data?.detail || '删除视频失败')
  if (task.url && objectURLs.has(task.url)) {
    URL.revokeObjectURL(task.url)
    objectURLs.delete(task.url)
  }
  turn.tasks = turn.tasks.filter((item) => item.id !== task.id)
  if (!turn.tasks.length) turns.value = turns.value.filter((item) => item.id !== turn.id)
  Message.success('已从当前对话移除')
}

function validate(): string | null {
  if (!selected.value) return '后台暂未启用视频模型'
  if (!form.prompt.trim()) return '请输入视频创作描述'
  if (form.prompt.trim().length > maxPromptLength.value) return `当前模型最多支持 ${maxPromptLength.value} 个字符`
  if (!ratios.value.includes(form.ratio)) return '当前模型不支持所选画面比例'
  if (!resolutions.value.includes(form.resolution)) return '当前模型不支持所选清晰度'
  if (!durations.value.includes(form.duration)) return '当前模型不支持所选视频时长'
  if (!countOptions.value.includes(form.count)) return '当前模型不支持所选生成数量'
  if (form.images.length < minImages.value) return `当前模型至少需要 ${minImages.value} 张参考图片`
  if (form.images.length > inputLimit('images')) return `当前模型最多支持 ${inputLimit('images')} 张参考图片`
  if (form.videos.length > inputLimit('videos')) return `当前模型最多支持 ${inputLimit('videos')} 个参考视频`
  if (form.audios.length > inputLimit('audios')) return `当前模型最多支持 ${inputLimit('audios')} 个参考音频`
  if (maxMediaFiles.value > 0 && mediaCount.value > maxMediaFiles.value) return `当前模型最多支持 ${maxMediaFiles.value} 个参考素材`
  if (Number(auth.user?.credits || 0) < totalPrice.value) return `可用额度不足，本次需要 ${totalPrice.value} 额度`
  return null
}

async function create() {
  const error = validate()
  if (error) return Message.warning(error)
  creating.value = true
  const snapshot = {
    model: form.model,
    modelName: modelName.value,
    prompt: form.prompt.trim(),
    ratio: form.ratio,
    resolution: form.resolution,
    duration: form.duration,
    reference_mode: form.reference_mode,
    images: [...form.images],
    videos: [...form.videos],
    audios: [...form.audios],
    count: form.count,
    cost: price.value,
  }
  const turn: CreationTurn = { id: crypto.randomUUID(), prompt: snapshot.prompt, createdAt: Math.floor(Date.now() / 1000), tasks: [] }
  turns.value.push(turn)
  form.prompt = ''
  await nextTick()
  turn.tasks = Array.from({ length: snapshot.count }, () => ({
    id: crypto.randomUUID(), status: 'submitting', progress: 0, cost: snapshot.cost,
    model: snapshot.modelName, ratio: snapshot.ratio, resolution: snapshot.resolution, duration: snapshot.duration,
  }))
  composer.value?.scrollIntoView({ behavior: 'smooth', block: 'end' })
  await Promise.all(turn.tasks.map(async (task) => {
    const response = await api('/video/jobs', {
      method: 'POST',
      body: JSON.stringify({
        model: snapshot.model,
        prompt: snapshot.prompt,
        ratio: snapshot.ratio,
        resolution: snapshot.resolution,
        duration: snapshot.duration,
        reference_mode: snapshot.reference_mode,
        reference_images: snapshot.images,
        reference_videos: snapshot.videos,
        reference_audios: snapshot.audios,
        concurrency: 1,
      }),
    })
    if (response.ok) {
      task.id = response.data.id
      task.status = response.data.status || 'queued'
    } else {
      task.status = 'failed'
		task.error = safeError(response.data?.detail, '提交失败，额度未扣除')
    }
  }))
  creating.value = false
  await auth.refreshUser()
  if (turn.tasks.some((task) => isPending(task.status))) schedulePoll(800)
  if (turn.tasks.every((task) => task.status === 'failed')) Message.error(turn.tasks[0]?.error || '任务提交失败')
  else Message.success(`${turn.tasks.filter((task) => task.status !== 'failed').length} 个视频任务已提交`)
}

function schedulePoll(delay = 3500) {
  if (pollTimer) window.clearTimeout(pollTimer)
  pollTimer = window.setTimeout(poll, delay)
}

async function loadTaskVideo(task: VideoTask) {
  if (task.url) return
  try {
    const response = await fetch(`/admin/api/video/jobs/${task.id}/content`, { headers: auth.token ? { Authorization: `Bearer ${auth.token}` } : {} })
    if (!response.ok) return
    const url = URL.createObjectURL(await response.blob())
    objectURLs.add(url)
    task.url = url
  } catch { /* The result remains available from the generation record. */ }
}

async function poll() {
  const pending = turns.value.flatMap((turn) => turn.tasks).filter((task) => isPending(task.status) && task.status !== 'submitting')
  if (!pending.length) return
  await Promise.all(pending.map(async (task) => {
    const response = await api(`/video/jobs/${task.id}`)
    if (!response.ok) {
      task.status = 'failed'
		task.error = safeError(response.data?.detail, '任务状态查询失败')
      return
    }
    task.status = response.data.status
    task.progress = Number(response.data.progress || 0)
		task.error = response.data.error ? safeError(response.data.error?.message || response.data.error) : undefined
    if (task.status === 'completed') await loadTaskVideo(task)
  }))
  if (turns.value.some((turn) => turn.tasks.some((task) => isPending(task.status)))) {
    schedulePoll()
  } else {
    await auth.refreshUser()
    const latest = turns.value.at(-1)
    if (latest?.tasks.some((task) => task.status === 'completed')) Message.success('本轮视频已生成并保存')
  }
}

function thumb(item: any) {
  return imageUrl(item.thumbnail_url || item.thumb || item.url || item.file || item.image)
}
</script>

<template>
  <div class="video-page">
    <header class="page-head">
      <div>
        <h2>视频创作</h2>
        <p>描述你的想法，选择模型支持的参数和素材，创作结果会保存在当前会话中。</p>
      </div>
      <div class="page-head-actions">
        <button v-if="turns.length" type="button" class="clear-screen" title="清空当前屏幕，不删除作品" @click="clearConversation"><IconDelete /><span>清屏</span></button>
        <span v-if="pendingCount" class="pending-state"><i></i>{{ pendingCount }} 个任务处理中</span>
      </div>
    </header>

    <main class="conversation" aria-live="polite">
      <div v-if="loadingHistory" class="history-loading"><IconLoading /><span>正在读取创作记录</span></div>
      <section v-else-if="!turns.length" class="conversation-empty">
        <span><IconVideoCamera /></span>
        <h3>开始一段视频创作</h3>
        <p>输入画面、动作和镜头描述，生成后的视频与参数会按对话顺序呈现在这里。</p>
      </section>

      <article v-for="turn in turns" :key="turn.id" class="turn">
        <div class="user-message">
          <div class="message-copy">
            <p>{{ turn.prompt }}</p>
            <time>{{ formatDate(turn.createdAt) }}</time>
          </div>
          <span class="user-mark">我</span>
        </div>
        <div class="assistant-message">
          <span class="assistant-mark"><IconVideoCamera /></span>
          <div class="answer">
            <div class="answer-head">
              <strong>{{ turn.tasks.some(task => isPending(task.status)) ? '正在创作视频' : '视频创作结果' }}</strong>
              <span>{{ turn.tasks.length }} 个结果</span>
            </div>
            <div class="video-grid" :class="{ single: turn.tasks.length === 1 }">
                <section v-for="(task, index) in turn.tasks" :key="task.id" class="video-result" :class="task.status">
                  <div class="video-frame">
                    <div v-if="isFinished(task.status)" class="video-actions">
                      <button v-if="task.status === 'completed' || task.status === 'success'" type="button" title="下载视频" aria-label="下载视频" @click.stop="downloadTask(task, index)"><IconDownload /></button>
                      <button type="button" title="删除当前结果" aria-label="删除当前结果" @click.stop="removeTask(turn, task)"><IconDelete /></button>
                    </div>
                    <video v-if="task.url" :src="task.url" controls playsinline preload="metadata" />
                  <div v-else-if="isPending(task.status)" class="rendering">
                    <span class="render-ring"><IconLoading /></span>
                    <strong>{{ task.status === 'queued' ? '等待生成' : task.status === 'submitting' ? '正在提交' : '正在生成' }}</strong>
                    <small>{{ task.progress > 0 ? `${task.progress}%` : `任务 ${String(index + 1).padStart(2, '0')}` }}</small>
                    <div class="progress"><i :style="{ width: `${Math.max(6, task.progress)}%` }"></i></div>
                  </div>
                  <div v-else-if="task.status === 'completed'" class="rendering unavailable">
                    <IconVideoCamera /><strong>视频已生成</strong><small>请前往生成记录查看文件</small>
                  </div>
                  <div v-else class="rendering failed-state">
                    <span><IconExclamationCircle /></span>
                    <strong>生成失败 · 额度已退回</strong>
                    <small :title="task.error">{{ task.error || '生成服务未返回具体原因' }}</small>
                  </div>
                </div>
                <div class="result-info">
                  <div class="result-title"><strong>视频 {{ String(index + 1).padStart(2, '0') }}</strong><span :class="{ failed: task.status === 'failed' }"><IconCheck v-if="task.status === 'completed'" />{{ statusText(task) }}</span></div>
                  <dl>
                    <div><dt>模型</dt><dd>{{ task.model }}</dd></div>
                    <div><dt>比例</dt><dd>{{ task.ratio }}</dd></div>
                    <div><dt>时长</dt><dd>{{ task.duration }}</dd></div>
                    <div><dt>清晰度</dt><dd>{{ task.resolution }}</dd></div>
                    <div class="cost-detail" :class="{ refunded: task.status === 'failed' }"><dt>消耗</dt><dd>{{ task.status === 'failed' ? '已退款' : `${task.cost} 额度` }}</dd></div>
                  </dl>
                </div>
              </section>
            </div>
          </div>
        </div>
      </article>
    </main>

    <section ref="composer" class="composer" :class="{ disabled: !models.length }">
      <div v-if="mediaCount" class="material-strip">
        <figure v-for="(item, index) in form.images" :key="`image-${index}`"><img :src="item" :alt="`参考图片 ${index + 1}`"><button aria-label="移除图片" @click="remove('images', index)"><IconClose /></button><small>图片</small></figure>
        <figure v-for="(_, index) in form.videos" :key="`video-${index}`" class="media-file"><IconVideoCamera /><button aria-label="移除视频" @click="remove('videos', index)"><IconClose /></button><small>视频 {{ index + 1 }}</small></figure>
        <figure v-for="(_, index) in form.audios" :key="`audio-${index}`" class="media-file"><IconMusic /><button aria-label="移除音频" @click="remove('audios', index)"><IconClose /></button><small>音频 {{ index + 1 }}</small></figure>
      </div>
      <a-textarea
        v-model="form.prompt"
        class="prompt-input"
        :max-length="maxPromptLength"
        :auto-size="{ minRows: 3, maxRows: 8 }"
        :disabled="!models.length"
        :placeholder="models.length ? '描述主体、动作、镜头、光线与氛围…' : '后台暂未启用视频模型'"
      />
      <div class="composer-foot">
        <div class="composer-tools">
          <div class="model-control" :class="{ disabled: !models.length }" :title="modelName">
            <IconVideoCamera class="model-control-icon" />
            <a-select v-model="form.model" class="model-select" :bordered="false" size="small" :disabled="!models.length" :trigger-props="{ position: 'top', autoFitPosition: false, autoFitPopupWidth: false, popupStyle: { width: '340px' } }">
              <a-option v-for="model in models" :key="model.id" :value="publicModelID(model)" :title="publicModelName(model)"><span class="model-option"><IconVideoCamera />{{ publicModelName(model) }}</span></a-option>
            </a-select>
          </div>
          <button v-if="canAddMedia('images')" class="tool-button" title="上传参考图片" @click="fileInput?.click()"><IconUpload /><span>图片</span></button>
          <button v-if="canAddMedia('images')" class="tool-button" title="选择已生成图片" @click="openPicker"><IconImage /><span>作品库</span></button>
          <label v-if="canAddMedia('videos')" class="tool-button" title="上传参考视频"><IconVideoCamera /><span>视频</span><input hidden type="file" :accept="acceptFor('videos')" @change="addFiles($event, 'videos')"></label>
          <label v-if="canAddMedia('audios')" class="tool-button" title="上传参考音频"><IconMusic /><span>音频</span><input hidden type="file" :accept="acceptFor('audios')" @change="addFiles($event, 'audios')"></label>
          <input ref="fileInput" hidden type="file" :accept="acceptFor('images')" multiple @change="addFiles($event, 'images')">
          <div class="parameter-control ratio-control" :class="{ disabled: !ratios.length }">
            <span class="ratio-glyph" :style="ratioIconStyle(form.ratio)" aria-hidden="true"></span>
            <a-select v-model="form.ratio" class="parameter-select" :bordered="false" size="small" :disabled="!ratios.length" :trigger-props="{ position: 'top', autoFitPosition: false, autoFitPopupWidth: false, popupStyle: { width: '132px' } }">
              <a-option v-for="item in ratios" :key="item" :value="item"><span class="parameter-option"><i class="ratio-glyph" :style="ratioIconStyle(item)"></i>{{ item }}</span></a-option>
            </a-select>
          </div>
          <div class="parameter-control duration-control" :class="{ disabled: !durations.length }">
            <IconClockCircle />
            <a-select v-model="form.duration" class="parameter-select" :bordered="false" size="small" :disabled="!durations.length" :trigger-props="{ position: 'top', autoFitPosition: false, autoFitPopupWidth: false, popupStyle: { width: '118px' } }">
              <a-option v-for="item in durations" :key="item" :value="item"><span class="parameter-option"><IconClockCircle />{{ item }}</span></a-option>
            </a-select>
          </div>
          <div class="parameter-control resolution-control" :class="{ disabled: !resolutions.length }">
            <IconComputer />
            <a-select v-model="form.resolution" class="parameter-select" :bordered="false" size="small" :disabled="!resolutions.length" :trigger-props="{ position: 'top', autoFitPosition: false, autoFitPopupWidth: false, popupStyle: { width: '112px' } }">
              <a-option v-for="item in resolutions" :key="item" :value="item"><span class="parameter-option"><IconComputer />{{ item }}</span></a-option>
            </a-select>
          </div>
          <div class="count-control" role="radiogroup" aria-label="生成数量">
            <button v-for="item in countOptions" :key="item" :class="{ active: form.count === item }" role="radio" :aria-checked="form.count === item" @click="form.count = item">{{ item }}</button>
          </div>
        </div>
        <div class="create-area">
          <button class="create-button" :disabled="creating || !models.length" @click="create"><IconLoading v-if="creating" /><IconVideoCamera v-else /><span>{{ creating ? '提交中' : `生成 ${form.count} 个` }}</span><i></i><strong>{{ totalPrice }} 额度</strong></button>
        </div>
      </div>
      <p v-if="selected" class="capability-note">
        <strong>模型能力</strong><span>支持 {{ ratios.join('、') || '默认比例' }}，{{ durations.join('、') || '默认时长' }}，{{ resolutions.join('、') || '默认清晰度' }}<template v-if="minImages">；至少需要 {{ minImages }} 张参考图</template></span>
      </p>
    </section>

    <a-modal v-model:visible="pickerOpen" title="选择已生成图片" :footer="false" width="760px" modal-class="user-dialog">
      <div v-if="generatedImages.length" class="picker-grid"><button v-for="item in generatedImages" :key="item.file || item.url" @click="useGenerated(item)"><img :src="thumb(item)" alt="已生成图片"></button></div>
      <a-empty v-else description="暂无可用图片" />
    </a-modal>
  </div>
</template>

<style scoped>
.video-page{max-width:1120px;margin:0 auto;padding-bottom:26px}.page-head{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;margin-bottom:24px}.page-head h2{margin:0;color:var(--ns-ink);font-size:22px;letter-spacing:0}.page-head p{margin:7px 0 0;color:var(--ns-ink-faint);font-size:12px;line-height:1.6}.page-head-actions{display:flex;align-items:center;gap:12px}.clear-screen{height:32px;padding:0 12px;display:flex;align-items:center;gap:6px;border:1px solid #d8dcd4;border-radius:999px;background:#fff;color:var(--ns-ink-soft);font-size:10px;cursor:pointer}.clear-screen:hover{border-color:#bfc6bc;background:#f6f7f3;color:var(--ns-ink)}.clear-screen :deep(svg){width:13px;height:13px}.pending-state{display:flex;align-items:center;gap:7px;color:var(--ns-accent-strong);font-size:11px;white-space:nowrap}.pending-state i{width:7px;height:7px;border-radius:50%;background:#74866b;animation:pulse 1.3s ease-in-out infinite}.conversation{min-height:360px;padding:8px 6px 18px}.history-loading,.conversation-empty{min-height:330px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;color:var(--ns-ink-faint)}.history-loading :deep(svg){width:20px;height:20px;margin-bottom:10px;animation:spin .9s linear infinite}.history-loading span{font-size:11px}.conversation-empty>span{width:52px;height:52px;display:grid;place-items:center;border:1px solid var(--ns-line-strong);border-radius:50%;background:#f4f5f1;color:#6b7865}.conversation-empty>span :deep(svg){width:21px;height:21px}.conversation-empty h3{margin:16px 0 7px;color:var(--ns-ink);font-size:14px;letter-spacing:0}.conversation-empty p{max-width:440px;margin:0;font-size:11px;line-height:1.7}.turn{padding:14px 0 30px}.turn+.turn{border-top:1px solid #e6e8e2}.user-message{display:flex;justify-content:flex-end;align-items:flex-start;gap:10px;margin:4px 0 22px;padding-left:15%}.user-mark,.assistant-mark{width:30px;height:30px;flex:0 0 30px;display:grid;place-items:center;border-radius:50%;font-size:10px}.user-mark{background:#252b26;color:#fff}.message-copy{max-width:720px;padding:13px 16px;border:1px solid #dfe3da;border-radius:14px 4px 14px 14px;background:#f1f3ee}.message-copy p{margin:0;color:var(--ns-ink);font-size:12px;line-height:1.7;white-space:pre-wrap;overflow-wrap:anywhere}.message-copy time{display:block;margin-top:7px;color:var(--ns-ink-faint);font-size:9px;text-align:right}.assistant-message{display:flex;align-items:flex-start;gap:12px}.assistant-mark{border:1px solid #cfd5ca;background:#fff;color:#60705a}.assistant-mark :deep(svg){width:15px;height:15px}.answer{min-width:0;flex:1}.answer-head{min-height:30px;display:flex;align-items:flex-start;justify-content:space-between;gap:16px}.answer-head strong{font-size:12px}.answer-head>span{color:var(--ns-ink-faint);font-size:10px}.video-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin-top:10px}.video-grid.single{grid-template-columns:minmax(0,680px)}.video-result{min-width:0;overflow:hidden;border:1px solid #dfe2dc;border-radius:8px;background:#fff;box-shadow:0 7px 24px rgba(35,42,36,.055)}.video-frame{aspect-ratio:16/9;overflow:hidden;background:#252a26}.video-frame video{width:100%;height:100%;display:block;object-fit:contain;background:#171a18}.rendering{width:100%;height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:7px;background:#e9ece6;color:#596454}.rendering strong{font-size:11px}.rendering small{max-width:82%;overflow:hidden;color:#7a8376;font-size:9px;text-overflow:ellipsis;white-space:nowrap}.render-ring{width:34px;height:34px;display:grid;place-items:center;border:1px solid #cdd3c9;border-radius:50%;background:#f7f8f5}.render-ring :deep(svg){width:16px;height:16px;animation:spin .9s linear infinite}.progress{width:120px;height:3px;margin-top:5px;overflow:hidden;border-radius:4px;background:#d8ddd4}.progress i{display:block;height:100%;border-radius:4px;background:#74866b;transition:width .4s ease}.rendering.unavailable :deep(svg){width:25px;height:25px}.failed-state{padding:18px;background:#f2ece9;color:#79584d}.failed-state>span{width:32px;height:32px;display:grid;place-items:center;border-radius:50%;background:#e6d8d2}.failed-state small{white-space:normal;text-align:center;line-height:1.5}.result-info{padding:13px 14px 14px}.result-title{display:flex;align-items:center;justify-content:space-between;gap:12px}.result-title>strong{font-size:11px}.result-title>span{display:flex;align-items:center;gap:4px;color:#66755f;font-size:9px;white-space:nowrap}.result-title>span.failed{color:#8b5f52}.result-title :deep(svg){width:11px;height:11px}.result-info dl{display:grid;grid-template-columns:1.4fr repeat(4,1fr);gap:9px;margin:13px 0 0;padding-top:11px;border-top:1px solid #eceee9}.result-info dl>div{min-width:0}.result-info dt{margin-bottom:4px;color:var(--ns-ink-faint);font-size:8px}.result-info dd{margin:0;overflow:hidden;color:var(--ns-ink-soft);font-size:9px;text-overflow:ellipsis;white-space:nowrap}.composer{position:relative;z-index:2;margin-top:10px;padding:14px 16px 11px;border:1px solid #cfd5cb;border-radius:16px;background:#fff;box-shadow:0 12px 38px rgba(34,42,35,.1);transition:border-color .2s ease,box-shadow .2s ease}.composer:focus-within{border-color:#8c9a86;box-shadow:0 14px 42px rgba(34,42,35,.13)}.composer.disabled{background:#f7f8f5}.prompt-input{margin:0;border:0!important;background:transparent!important;box-shadow:none!important}.prompt-input:focus-within{border:0!important;box-shadow:none!important}.prompt-input :deep(.arco-textarea){padding:4px 2px 12px;border:0!important;background:transparent!important;box-shadow:none!important;color:var(--ns-ink);font-size:13px;line-height:1.7;resize:none}.material-strip{display:flex;gap:8px;padding:0 0 10px;overflow-x:auto}.material-strip figure{width:64px;height:54px;flex:0 0 64px;position:relative;display:grid;place-items:center;margin:0;overflow:hidden;border:1px solid var(--ns-line);border-radius:7px;background:#eef0eb;color:#687462}.material-strip img{width:100%;height:100%;object-fit:cover}.material-strip figure>button{width:18px;height:18px;padding:0;position:absolute;top:3px;right:3px;display:grid;place-items:center;border:0;border-radius:50%;background:rgba(28,33,29,.78);color:#fff;cursor:pointer}.material-strip figure>button :deep(svg){width:10px}.material-strip figure>small{position:absolute;left:4px;bottom:3px;padding:2px 4px;border-radius:4px;background:rgba(28,33,29,.72);color:#fff;font-size:7px}.media-file> :deep(svg){width:20px;height:20px}.composer-foot{display:flex;align-items:center;justify-content:space-between;gap:12px;padding-top:11px;border-top:1px solid #eceee9}.composer-tools{min-width:0;display:flex;align-items:center;gap:6px;flex-wrap:wrap}.tool-button,.create-button,.count-control{height:30px;border-radius:999px}.tool-button{padding:0 11px;display:flex;align-items:center;gap:6px;border:1px solid #dce0d8;background:#f5f6f2;color:var(--ns-ink-soft);font-size:10px;line-height:30px;cursor:pointer}.tool-button :deep(svg){width:13px;flex:0 0 13px}.pill-select{width:82px;border-radius:999px;background:#f1f3ee}.pill-select.model-select{width:150px}.pill-select :deep(.arco-select-view){height:30px;padding:0 10px;border-radius:999px;background:#f1f3ee;font-size:10px}.count-control{display:flex;align-items:center;padding:3px;background:#f1f3ee}.count-control button{min-width:25px;height:24px;padding:0 7px;border:0;border-radius:999px;background:transparent;color:var(--ns-ink-soft);font-size:9px;cursor:pointer}.count-control button.active{background:#263029;color:#fff}.create-area{display:flex;align-items:center;gap:10px;white-space:nowrap}.create-area>span{color:var(--ns-ink-faint);font-size:9px}.create-button{min-width:102px;padding:0 15px;display:flex;align-items:center;justify-content:center;gap:7px;border:0;background:#273129;color:#fff;font-size:10px;font-weight:650;cursor:pointer}.create-button:hover{background:#344039}.create-button:disabled{opacity:.5;cursor:not-allowed}.create-button :deep(svg){width:13px}.create-button :deep(.arco-icon-loading){animation:spin .9s linear infinite}.capability-note{margin:8px 2px 0;color:var(--ns-ink-faint);font-size:8px;line-height:1.5}.picker-grid{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:9px}.picker-grid button{padding:0;aspect-ratio:1;overflow:hidden;border:1px solid var(--ns-line);border-radius:6px;background:#eef0eb;cursor:pointer}.picker-grid img{width:100%;height:100%;display:block;object-fit:cover}@keyframes spin{to{transform:rotate(360deg)}}@keyframes pulse{0%,100%{opacity:.45;transform:scale(.84)}50%{opacity:1;transform:scale(1)}}
@media(max-width:800px){.video-grid{grid-template-columns:1fr}.video-grid.single{grid-template-columns:1fr}.composer-foot{align-items:flex-end}.create-area>span{display:none}.pill-select.model-select{width:132px}}
@media(max-width:560px){.video-page{padding-bottom:14px}.page-head{margin-bottom:14px}.page-head p{max-width:290px}.pending-state{display:none}.conversation{padding-inline:0}.conversation-empty{min-height:280px}.turn{padding-bottom:24px}.user-message{padding-left:8%;margin-bottom:18px}.user-mark,.assistant-mark{width:27px;height:27px;flex-basis:27px}.message-copy{padding:11px 13px}.assistant-message{gap:8px}.result-info dl{grid-template-columns:repeat(3,1fr);row-gap:10px}.composer{padding:12px;border-radius:13px}.composer-foot{display:block}.composer-tools{gap:5px}.tool-button{width:30px;padding:0;justify-content:center}.tool-button span{display:none}.pill-select{width:72px}.pill-select.model-select{width:124px}.count-control{max-width:100%;margin-top:2px}.create-area{justify-content:flex-end;margin-top:10px}.create-button{width:100%;height:34px}.capability-note{display:none}.picker-grid{grid-template-columns:repeat(3,minmax(0,1fr))}}
@media(prefers-reduced-motion:reduce){.pending-state i,.render-ring :deep(svg),.history-loading :deep(svg),.create-button :deep(.arco-icon-loading){animation:none}.progress i{transition:none}}
.composer-tools>.pill-select{width:82px!important;height:30px!important;flex:0 0 82px!important;padding:0 10px!important;border-radius:999px!important;background:#f1f3ee!important;font-size:10px!important}.composer-tools>.pill-select :deep(.arco-select-view-arrow-icon){display:none}
.composer-tools :deep(.pill-select){width:82px!important;height:30px!important;flex:0 0 82px!important;padding:0 10px!important;border-radius:999px!important;background:#f1f3ee!important;font-size:10px!important}.composer-tools :deep(.pill-select .arco-select-view-arrow-icon){display:none}
.model-control{width:150px;height:30px;order:-10;flex:0 0 150px;display:flex;align-items:center;overflow:hidden;border-radius:999px;background:#e9ede6;color:#596653}.model-control.disabled{opacity:.55}.model-control-icon{width:14px;height:14px;flex:0 0 14px;margin-left:10px}.model-select{min-width:0;height:30px;flex:1;background:transparent!important}.model-control :deep(.arco-select-view){height:30px!important;padding:0 9px 0 6px!important;background:transparent!important;font-size:10px!important}.model-control :deep(.arco-select-view-value){overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.model-control :deep(.arco-select-view-arrow-icon){display:none}
@media(max-width:800px){.model-control{width:132px;flex-basis:132px}}
@media(max-width:560px){.composer-tools>.pill-select{width:72px!important;flex-basis:72px!important}.composer-tools :deep(.pill-select){width:72px!important;flex-basis:72px!important}.model-control{width:124px;flex-basis:124px}}
.model-control{width:200px;flex-basis:200px}.model-control :deep(.arco-select-view-value){font-weight:600}.model-option,.parameter-option{display:flex;align-items:center;gap:9px;white-space:nowrap}.model-option :deep(svg),.parameter-option :deep(svg){width:14px;height:14px;flex:0 0 14px;color:#66745f}.parameter-control{height:30px;flex:0 0 auto;display:flex;align-items:center;overflow:hidden;border-radius:999px;background:#f1f3ee;color:#63705e}.parameter-control.disabled{opacity:.55}.parameter-control>svg{width:14px;height:14px;flex:0 0 14px;margin-left:10px}.ratio-control{width:82px}.duration-control{width:92px}.resolution-control{width:102px}.parameter-select{min-width:0;height:30px;flex:1;background:transparent!important}.parameter-control :deep(.arco-select-view){height:30px!important;padding:0 8px 0 6px!important;background:transparent!important;font-size:10px!important}.parameter-control :deep(.arco-select-view-value){font-weight:600}.parameter-control :deep(.arco-select-view-arrow-icon){display:none}.ratio-glyph{display:inline-block;box-sizing:border-box;flex:0 0 auto;border:1.5px solid currentColor;border-radius:3px}.ratio-control>.ratio-glyph{margin-left:10px}.parameter-option .ratio-glyph{color:#66745f}
@media(max-width:800px){.model-control{width:190px;flex-basis:190px}}
@media(max-width:560px){.model-control{width:170px;flex-basis:170px}.ratio-control{width:76px}.duration-control{width:82px}.resolution-control{width:92px}.parameter-control>svg,.ratio-control>.ratio-glyph{margin-left:8px}.parameter-control :deep(.arco-select-view){padding-right:6px!important}}
@media(max-width:560px){.composer-tools>.tool-button{width:auto;padding:0 9px}.composer-tools>.tool-button span{display:inline}}
.video-frame{position:relative}.video-actions{position:absolute;z-index:3;top:9px;right:9px;display:flex;gap:6px;opacity:0;transform:translateY(-2px);transition:opacity .18s ease,transform .18s ease}.video-frame:hover .video-actions,.video-actions:focus-within{opacity:1;transform:none}.video-actions button{width:30px;height:30px;padding:0;display:grid;place-items:center;border:1px solid rgba(255,255,255,.16);border-radius:50%;background:rgba(20,23,21,.74);color:#fff;cursor:pointer;backdrop-filter:blur(8px)}.video-actions button:hover{background:rgba(20,23,21,.94)}.video-actions :deep(svg){width:14px;height:14px}@media(max-width:800px){.video-actions{opacity:1;transform:none}}
.create-button{min-width:170px;height:34px}.create-button i{width:1px;height:13px;margin:0 2px;background:rgba(255,255,255,.2)}.create-button strong{color:#f1dc72;font-size:10px;font-weight:750}.capability-note{min-height:34px;margin:9px 0 0;padding:8px 11px;display:flex;align-items:center;gap:9px;border:1px solid #e3d893;border-radius:7px;background:#faf6dc;color:#5f5a3a;font-size:9px;line-height:1.5}.capability-note strong{flex:0 0 auto;padding:3px 7px;border-radius:999px;background:#e7d36c;color:#3d3b26;font-size:8px}.capability-note span{font-weight:600}.model-control{border:1px solid #cbd5c7!important;background:#e5ebe1!important;color:#40503f!important;box-shadow:0 1px 0 rgba(36,45,37,.04)}.model-control :deep(.arco-select-view-value){color:#29342b!important;font-weight:700!important}.parameter-control{border:1px solid #d8ddcf!important;background:#f0f2e9!important;color:#56654f!important}.parameter-control :deep(.arco-select-view-value){color:#303a31!important;font-weight:700!important}.count-control{border:1px solid #d8ddcf;background:#f0f2e9}.result-info dl{gap:7px;padding-top:10px}.result-info dl>div{padding:8px 9px;border:1px solid #e2e5dd;border-radius:6px;background:#f4f5f1}.result-info dt{color:#778172;font-size:8px;font-weight:600}.result-info dd{color:#303a31;font-size:10px;font-weight:700}.result-info .cost-detail{border-color:#e3d893;background:#faf6dc}.result-info .cost-detail dd{color:#78651f}.result-info .cost-detail.refunded{border-color:#e7cec4;background:#f7ece8}.result-info .cost-detail.refunded dd{color:#925c4f}
@media(max-width:560px){.capability-note{display:flex;align-items:flex-start}.create-button{min-width:0}.result-info dl>div{padding:7px}.result-info dd{font-size:9px}.model-control{width:100%;flex-basis:100%}.model-select{width:calc(100% - 30px)!important}}
</style>

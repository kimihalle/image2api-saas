<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconImage, IconThunderbolt, IconVideoCamera } from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../services/api'

const props = defineProps<{ account?: any; model?: any }>()
const emit = defineEmits<{ close: [] }>()
const models = ref<any[]>([])
const modelId = ref('')
const busy = ref(false)
const status = ref('')
const error = ref('')
const result = reactive({ url: '', kind: '', elapsed: 0, provider: '' })
const form = reactive({ prompt: '一只坐在木桌上的猫，柔和工作室光线', ratio: '', resolution: '', duration: '', references: '' })
let pollTimer: number | undefined
let submittedAt = 0

const availableModels = computed(() => models.value.filter((item) => !props.account || item.provider === props.account.pool))
const currentModel = computed(() => availableModels.value.find((item) => (item.alias || item.id) === modelId.value) || null)
const isVideo = computed(() => currentModel.value?.type === 'video')
const previewUrl = computed(() => imageUrl(result.url))

function selectDefaults() {
  const item = currentModel.value
  if (!item) return
  form.ratio = item.ratios?.[0] || (item.type === 'video' ? '16:9' : '1:1')
  form.resolution = item.resolutions?.[0] || Object.keys(item.prices || {})[0] || ''
  form.duration = item.durations?.[0] || Object.keys(item.duration_prices || {}).find((key) => key !== 'per_second') || ''
}

async function load() {
  const response = await api('/managed-models')
  models.value = (response.data?.data || response.data || []).filter((item: any) => item.enabled !== false)
  const preferred = props.model && models.value.find((item) => item.id === props.model.id)
  const first = preferred || availableModels.value[0]
  modelId.value = first ? (first.alias || first.id) : ''
  selectDefaults()
}

function finish(payload: any) {
  busy.value = false
  result.url = payload.url || payload.file || ''
  result.kind = payload.kind || (isVideo.value ? 'video' : 'image')
  result.elapsed = Number(payload.elapsed_ms || 0)
  result.provider = payload.provider || currentModel.value?.provider || ''
  status.value = result.url ? '测试完成' : '任务已完成'
}

async function recover() {
  const response = await api('/jobs/mine?source=admin')
  const latest = response.data?.latest
  const pending = response.data?.pending
  if (pending) { pollTimer = window.setTimeout(recover, 3000); return }
  if (latest && Number(latest.ts || 0) * 1000 >= submittedAt - 3000) {
    if (latest.status === 'success') { finish(latest); return }
    if (latest.status === 'failed') { busy.value = false; error.value = latest.error || '测试失败'; status.value = ''; return }
  }
  pollTimer = window.setTimeout(recover, 3000)
}

async function run() {
  if (!currentModel.value) return Message.warning('当前账号没有可测试的已启用模型')
  if (!form.prompt.trim()) return Message.warning('请输入测试提示词')
  busy.value = true
  error.value = ''
  status.value = isVideo.value ? '视频生成中，请勿关闭窗口' : '图像生成中'
  Object.assign(result, { url: '', kind: '', elapsed: 0, provider: '' })
  submittedAt = Date.now()
  const payload: any = {
    model: modelId.value,
    prompt: form.prompt.trim(),
    ratio: form.ratio,
    resolution: form.resolution,
    reference_images: form.references.split(/\r?\n/).map((item) => item.trim()).filter(Boolean),
  }
  if (form.duration) payload.duration = form.duration
  if (props.account?.id) payload.account_id = props.account.id
  try {
    const response = await api('/test', { method: 'POST', body: JSON.stringify(payload) })
    if (response.ok) { finish(response.data); return }
    if ([408, 504, 520, 521, 522, 523, 524, 525].includes(response.status)) {
      pollTimer = window.setTimeout(recover, 3000)
      return
    }
    busy.value = false
    status.value = ''
    error.value = response.data?.detail || `测试失败 (${response.status})`
  } catch (reason) {
    busy.value = false
    status.value = ''
    error.value = String(reason)
  }
}

onMounted(load)
onBeforeUnmount(() => { if (pollTimer) window.clearTimeout(pollTimer) })
</script>

<template>
  <a-modal :visible="true" :footer="false" :width="1040" title="模型调用测试" @cancel="emit('close')">
    <div class="test-layout">
      <section class="parameter-panel">
        <div class="test-context"><span><IconVideoCamera v-if="isVideo" /><IconImage v-else /></span><div><strong>{{ props.account ? (props.account.account_email || props.account.email || props.account.name || props.account.id) : (currentModel?.alias || currentModel?.id || '选择模型') }}</strong><small>{{ props.account ? `${props.account.pool} · 固定使用该账号` : '使用 Provider 调度池' }}</small></div></div>
        <a-form :model="form" layout="vertical">
          <a-form-item label="测试模型"><a-select v-model="modelId" placeholder="选择已启用模型" @change="selectDefaults"><a-option v-for="item in availableModels" :key="item.id" :value="item.alias || item.id">{{ item.alias || item.id }} · {{ item.type === 'video' ? '视频' : '图像' }}</a-option></a-select></a-form-item>
          <a-form-item label="提示词"><a-textarea v-model="form.prompt" :auto-size="{ minRows: 4, maxRows: 7 }" /></a-form-item>
          <div class="test-grid"><a-form-item label="画面比例"><a-select v-model="form.ratio"><a-option v-for="item in currentModel?.ratios || []" :key="item" :value="item">{{ item }}</a-option></a-select></a-form-item><a-form-item label="分辨率"><a-select v-model="form.resolution"><a-option v-for="item in currentModel?.resolutions || Object.keys(currentModel?.prices || {})" :key="item" :value="item">{{ item }}</a-option></a-select></a-form-item><a-form-item v-if="isVideo" label="时长"><a-select v-model="form.duration"><a-option v-for="item in currentModel?.durations || []" :key="item" :value="item">{{ item }}</a-option></a-select></a-form-item></div>
          <a-form-item v-if="Number(currentModel?.max_reference_images || 0) > 0" label="参考图地址（一行一个，可选）"><a-textarea v-model="form.references" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="填写站内图片路径或可访问图片地址" /></a-form-item>
        </a-form>
      </section>

      <section class="result-panel" :class="{ ready: previewUrl }">
        <template v-if="previewUrl">
          <video v-if="result.kind === 'video'" :src="previewUrl" controls />
          <img v-else :src="previewUrl" alt="模型测试结果" />
          <div class="result-caption"><strong>{{ status }}</strong><span>{{ result.provider }}{{ result.elapsed ? ` · ${(result.elapsed / 1000).toFixed(1)}s` : '' }}</span></div>
        </template>
        <div v-else class="result-empty">
          <span class="result-mark"><IconVideoCamera v-if="isVideo" /><IconImage v-else /></span>
          <strong>{{ busy ? status : '等待测试结果' }}</strong>
          <p>{{ busy ? '任务正在执行，完成后会自动显示结果。' : '左侧设置参数，开始测试后在这里查看真实输出。' }}</p>
        </div>
        <p v-if="error" class="test-error">{{ error }}</p>
      </section>
    </div>
    <div class="test-actions"><a-button @click="emit('close')">关闭</a-button><a-button type="primary" :loading="busy" :disabled="!availableModels.length" @click="run"><IconThunderbolt />开始测试</a-button></div>
  </a-modal>
</template>

<style scoped>
.test-layout{display:grid;grid-template-columns:minmax(0,.92fr) minmax(380px,1.08fr);min-height:560px;border:1px solid var(--ns-line);border-radius:8px;overflow:hidden}.parameter-panel{padding:22px;background:#fff}.test-context{display:flex;align-items:center;gap:11px;margin-bottom:18px;padding:12px;border:1px solid var(--ns-line);border-radius:7px;background:#fafaf8}.test-context>span{width:34px;height:34px;display:grid;place-items:center;border-radius:6px;background:#e8ece4;color:#53614e}.test-context>div{display:flex;flex-direction:column;min-width:0}.test-context strong{font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.test-context small{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.test-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:12px}.result-panel{min-width:0;padding:22px;position:relative;display:grid;place-items:center;background:#eff1ec;border-left:1px solid var(--ns-line)}.result-panel.ready{background:#252a26}.result-panel img,.result-panel video{display:block;width:100%;height:100%;max-height:510px;object-fit:contain}.result-empty{max-width:280px;text-align:center}.result-mark{width:50px;height:50px;display:grid;place-items:center;margin:0 auto 15px;border:1px solid var(--ns-line-strong);border-radius:50%;color:var(--ns-ink-faint)}.result-empty strong{font-size:13px}.result-empty p{margin:7px 0 0;color:var(--ns-ink-faint);font-size:10px;line-height:1.7}.result-caption{position:absolute;left:22px;right:22px;bottom:20px;padding:10px 12px;display:flex;justify-content:space-between;background:rgba(27,31,28,.8);color:#fff;font-size:10px}.result-caption span{color:#d4dad4}.test-error{position:absolute;left:20px;right:20px;bottom:20px;padding:10px 12px;border:1px solid #ead2ce;border-radius:6px;background:#fff7f5;color:var(--ns-danger);font-size:10px}.test-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:16px}@media(max-width:820px){.test-layout{grid-template-columns:1fr}.result-panel{min-height:360px;border-left:0;border-top:1px solid var(--ns-line)}}@media(max-width:620px){.parameter-panel{padding:16px}.test-grid{grid-template-columns:1fr 1fr}.result-panel{padding:16px;min-height:300px}}
</style>

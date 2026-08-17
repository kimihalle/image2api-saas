<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconCode, IconCopy, IconDelete, IconEdit, IconExperiment, IconPlus, IconRefresh, IconThunderbolt } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'

const tab = ref('usage')
const range = ref('7d')
const usage = ref<any>({ summary: {}, paths: [], models: [] })
const webhooks = ref<any[]>([])
const deliveries = ref<any[]>([])
const workflows = ref<any[]>([])
const models = ref<any[]>([])
const loading = ref(false)
const webhookOpen = ref(false)
const workflowOpen = ref(false)
const editingWebhook = ref<any>(null)
const editingWorkflow = ref<any>(null)
const webhookForm = reactive({ name: '', url: '', events: ['generation.completed', 'generation.failed'] as string[], enabled: true })
const workflowForm = reactive({ name: '', description: '', kind: 'image', model_id: '', prompt: '', enabled: true })
const eventOptions = [
  { value: 'generation.completed', label: '生成成功' },
  { value: 'generation.failed', label: '生成失败' },
  { value: 'generation.dead_letter', label: '任务进入死信' },
  { value: 'billing.refunded', label: '额度已退款' },
]

const summary = computed(() => usage.value.summary || {})
function time(value: any) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '--' }
function errorRate(item: any) { return item?.requests ? `${(Number(item.errors || 0) / Number(item.requests) * 100).toFixed(1)}%` : '0%' }
function modelName(id: string) { return models.value.find((x) => String(x.alias || x.id) === id)?.name || id || '未指定' }

async function load() {
  loading.value = true
  const [u, w, d, f, m] = await Promise.all([
    api(`/api-platform/usage?range=${range.value}`), api('/webhooks'), api('/webhook-deliveries'), api('/workflows'), api('/models'),
  ])
  if (u.ok) usage.value = u.data || usage.value
  if (w.ok) webhooks.value = w.data?.data || []
  if (d.ok) deliveries.value = d.data?.data || []
  if (f.ok) workflows.value = f.data?.data || []
  if (m.ok) models.value = (m.data?.data || []).filter((x: any) => x.enabled !== false)
  loading.value = false
}
async function loadUsage() {
  const response = await api(`/api-platform/usage?range=${range.value}`)
  if (response.ok) usage.value = response.data || usage.value
}

function editWebhook(item?: any) {
  editingWebhook.value = item || null
  webhookForm.name = item?.name || ''
  webhookForm.url = item?.url || ''
  try { webhookForm.events = Array.isArray(item?.events) ? item.events : JSON.parse(item?.events || '[]') } catch { webhookForm.events = [] }
  webhookForm.enabled = item?.enabled ?? true
  webhookOpen.value = true
}
async function saveWebhook() {
  if (!webhookForm.url.trim()) return Message.warning('请填写回调地址')
  const path = editingWebhook.value ? `/webhooks/${editingWebhook.value.id}` : '/webhooks'
  const response = await api(path, { method: editingWebhook.value ? 'PUT' : 'POST', body: JSON.stringify(webhookForm) })
  if (!response.ok) return Message.error(response.data?.detail || 'Webhook 保存失败')
  webhookOpen.value = false
  if (response.data?.secret) {
    Modal.info({ title: '签名密钥仅展示一次', content: response.data.secret, hideCancel: true })
  }
  Message.success('Webhook 已保存')
  await load()
}
async function testWebhook(item: any) {
  const response = await api(`/webhooks/${item.id}/test`, { method: 'POST' })
  if (!response.ok) return Message.error(response.data?.detail || '测试投递失败')
  Message.success('测试事件已进入投递队列')
  setTimeout(load, 800)
}
function removeWebhook(item: any) {
  Modal.warning({ title: '删除 Webhook', content: `确认删除“${item.name}”吗？`, hideCancel: false, onOk: async () => {
    const response = await api(`/webhooks/${item.id}`, { method: 'DELETE' })
    if (response.ok) { Message.success('已删除'); await load() } else Message.error(response.data?.detail || '删除失败')
  } })
}

function editWorkflow(item?: any) {
  editingWorkflow.value = item || null
  workflowForm.name = item?.name || ''
  workflowForm.description = item?.description || ''
  workflowForm.kind = item?.kind || 'image'
  workflowForm.model_id = item?.model_id || ''
  workflowForm.prompt = item?.prompt || ''
  workflowForm.enabled = item?.enabled ?? true
  workflowOpen.value = true
}
async function saveWorkflow() {
  if (!workflowForm.name.trim() || !workflowForm.prompt.trim()) return Message.warning('名称和提示词不能为空')
  const payload = { ...workflowForm, visibility: 'private', variables: [], defaults: {} }
  const path = editingWorkflow.value ? `/workflows/${editingWorkflow.value.id}` : '/workflows'
  const response = await api(path, { method: editingWorkflow.value ? 'PUT' : 'POST', body: JSON.stringify(payload) })
  if (!response.ok) return Message.error(response.data?.detail || '工作流保存失败')
  workflowOpen.value = false
  Message.success('工作流已保存')
  await load()
}
function removeWorkflow(item: any) {
  Modal.warning({ title: '删除工作流', content: `确认删除“${item.name}”吗？`, hideCancel: false, onOk: async () => {
    const response = await api(`/workflows/${item.id}`, { method: 'DELETE' })
    if (response.ok) { Message.success('已删除'); await load() } else Message.error(response.data?.detail || '删除失败')
  } })
}
async function copy(text: string) { await navigator.clipboard.writeText(text); Message.success('已复制') }

onMounted(load)
</script>

<template>
  <div class="platform-page">
    <header class="platform-head">
      <div><span>DEVELOPER PLATFORM</span><h2>开放平台</h2><p>查看接口质量，管理事件回调，并保存可复用的创作工作流。</p></div>
      <button class="refresh" aria-label="刷新" @click="load"><IconRefresh /></button>
    </header>

    <nav class="platform-tabs"><button :class="{ active: tab === 'usage' }" @click="tab = 'usage'">API 数据</button><button :class="{ active: tab === 'webhooks' }" @click="tab = 'webhooks'">Webhook</button><button :class="{ active: tab === 'workflows' }" @click="tab = 'workflows'">我的工作流</button></nav>

    <a-spin :loading="loading" class="platform-spin">
      <template v-if="tab === 'usage'">
        <div class="usage-tools"><div><strong>接口调用概览</strong><span>仅统计 OpenAI 兼容接口，不记录提示词正文</span></div><a-radio-group v-model="range" type="button" @change="loadUsage"><a-radio value="24h">24 小时</a-radio><a-radio value="7d">7 天</a-radio><a-radio value="30d">30 天</a-radio></a-radio-group></div>
        <section class="stat-band"><div><span>请求总数</span><strong>{{ Number(summary.requests || 0).toLocaleString() }}</strong></div><div><span>错误请求</span><strong class="danger">{{ Number(summary.errors || 0).toLocaleString() }}</strong></div><div><span>平均响应</span><strong>{{ Number(summary.avg_latency_ms || 0).toFixed(0) }}<i> ms</i></strong></div><div><span>接口消耗</span><strong>{{ Number(summary.credits || 0).toFixed(2) }}<i> 额度</i></strong></div></section>
        <div class="usage-grid">
          <section class="data-section"><header><IconCode /><div><strong>接口分布</strong><span>按路径聚合请求质量</span></div></header><div class="data-table"><table><thead><tr><th>接口</th><th>请求</th><th>错误率</th><th>延迟</th></tr></thead><tbody><tr v-for="item in usage.paths" :key="item.name"><td class="mono">{{ item.name }}</td><td>{{ item.requests }}</td><td :class="{ 'danger-text': item.errors }">{{ errorRate(item) }}</td><td>{{ Number(item.avg_latency_ms || 0).toFixed(0) }} ms</td></tr><tr v-if="!usage.paths?.length"><td colspan="4" class="empty">暂无调用数据</td></tr></tbody></table></div></section>
          <section class="data-section"><header><IconThunderbolt /><div><strong>模型分布</strong><span>按模型聚合请求与额度</span></div></header><div class="data-table"><table><thead><tr><th>模型</th><th>请求</th><th>错误率</th><th>消耗</th></tr></thead><tbody><tr v-for="item in usage.models" :key="item.name"><td>{{ modelName(item.name) }}</td><td>{{ item.requests }}</td><td :class="{ 'danger-text': item.errors }">{{ errorRate(item) }}</td><td>{{ Number(item.credits || 0).toFixed(2) }}</td></tr><tr v-if="!usage.models?.length"><td colspan="4" class="empty">暂无模型数据</td></tr></tbody></table></div></section>
        </div>
      </template>

      <template v-else-if="tab === 'webhooks'">
        <div class="toolbar"><div><strong>事件回调</strong><span>签名、重试与投递记录由系统统一管理</span></div><a-button type="primary" shape="round" @click="editWebhook()"><IconPlus />新建 Webhook</a-button></div>
        <section class="data-section endpoint-section"><div class="endpoint-list"><article v-for="item in webhooks" :key="item.id"><span :class="['health-dot', { off: !item.enabled }]"></span><div class="endpoint-main"><strong>{{ item.name }}</strong><button @click="copy(item.url)">{{ item.url }}<IconCopy /></button><small>{{ Array.isArray(item.events) ? item.events.join(' · ') : item.events }}</small></div><div class="endpoint-state"><span>最近状态</span><strong :class="{ danger: item.last_status >= 400 }">{{ item.last_status || '未投递' }}</strong></div><div class="row-actions"><button title="测试" @click="testWebhook(item)"><IconExperiment /></button><button title="编辑" @click="editWebhook(item)"><IconEdit /></button><button title="删除" @click="removeWebhook(item)"><IconDelete /></button></div></article><div v-if="!webhooks.length" class="empty-panel">尚未配置 Webhook</div></div></section>
        <section class="data-section delivery-section"><header><IconRefresh /><div><strong>最近投递</strong><span>失败事件最多自动重试 8 次</span></div></header><div class="data-table"><table><thead><tr><th>事件</th><th>状态</th><th>尝试</th><th>HTTP</th><th>时间</th><th>错误</th></tr></thead><tbody><tr v-for="item in deliveries" :key="item.id"><td>{{ item.event_type }}</td><td><span :class="['badge', item.status]">{{ item.status }}</span></td><td>{{ item.attempts }}</td><td>{{ item.http_status || '--' }}</td><td>{{ time(item.created_at) }}</td><td class="error-cell">{{ item.last_error || '--' }}</td></tr><tr v-if="!deliveries.length"><td colspan="6" class="empty">暂无投递记录</td></tr></tbody></table></div></section>
      </template>

      <template v-else>
        <div class="toolbar"><div><strong>我的工作流</strong><span>保存模型、参数和提示词结构，减少重复配置</span></div><a-button type="primary" shape="round" @click="editWorkflow()"><IconPlus />新建工作流</a-button></div>
        <section class="workflow-list"><article v-for="item in workflows" :key="item.id"><div class="workflow-mark"><component :is="item.kind === 'video' ? IconExperiment : IconThunderbolt" /></div><div><span>{{ item.kind === 'video' ? '视频' : '图片' }} · {{ modelName(item.model_id) }}</span><strong>{{ item.name }}</strong><p>{{ item.description || item.prompt }}</p></div><small>{{ item.use_count || 0 }} 次使用</small><div class="row-actions"><button title="复制提示词" @click="copy(item.prompt)"><IconCopy /></button><button title="编辑" @click="editWorkflow(item)"><IconEdit /></button><button title="删除" @click="removeWorkflow(item)"><IconDelete /></button></div></article><div v-if="!workflows.length" class="empty-panel">还没有保存工作流</div></section>
      </template>
    </a-spin>

    <a-modal v-model:visible="webhookOpen" :title="editingWebhook ? '编辑 Webhook' : '新建 Webhook'" :width="600" @ok="saveWebhook">
      <div class="form-stack"><label><span>名称</span><a-input v-model="webhookForm.name" placeholder="例如：生产环境通知" /></label><label><span>回调地址</span><a-input v-model="webhookForm.url" placeholder="https://example.com/webhooks/generation" /></label><label><span>订阅事件</span><a-checkbox-group v-model="webhookForm.events"><a-checkbox v-for="item in eventOptions" :key="item.value" :value="item.value">{{ item.label }}</a-checkbox></a-checkbox-group></label><label class="inline-label"><span>启用投递</span><a-switch v-model="webhookForm.enabled" size="small" /></label></div>
    </a-modal>
    <a-modal v-model:visible="workflowOpen" :title="editingWorkflow ? '编辑工作流' : '新建工作流'" :width="680" @ok="saveWorkflow">
      <div class="form-stack"><div class="form-grid"><label><span>名称</span><a-input v-model="workflowForm.name" placeholder="例如：电商主图" /></label><label><span>类型</span><a-select v-model="workflowForm.kind"><a-option value="image">图片</a-option><a-option value="video">视频</a-option></a-select></label></div><label><span>模型</span><a-select v-model="workflowForm.model_id" allow-search placeholder="选择已启用模型"><a-option v-for="item in models.filter((x) => x.type === workflowForm.kind)" :key="item.id" :value="item.alias || item.id">{{ item.name }}</a-option></a-select></label><label><span>说明</span><a-input v-model="workflowForm.description" placeholder="这个工作流适用于什么场景" /></label><label><span>提示词</span><a-textarea v-model="workflowForm.prompt" :auto-size="{ minRows: 6, maxRows: 12 }" placeholder="可复用的提示词结构" /></label></div>
    </a-modal>
  </div>
</template>

<style scoped>
.platform-page{max-width:1380px;margin:0 auto}.platform-head{display:flex;align-items:center;justify-content:space-between}.platform-head>div>span{color:#8a7628;font-size:9px;font-weight:750}.platform-head h2{margin:6px 0 5px;font-size:25px}.platform-head p{margin:0;color:var(--ns-ink-soft);font-size:11px}.refresh{width:38px;height:38px;display:grid;place-items:center;border:1px solid var(--ns-line);border-radius:50%;background:#fff;color:var(--ns-ink-soft);cursor:pointer}.platform-tabs{display:flex;gap:6px;margin:22px 0 16px;border-bottom:1px solid var(--ns-line)}.platform-tabs button{height:42px;padding:0 16px;border:0;border-bottom:2px solid transparent;background:transparent;color:var(--ns-ink-soft);font-size:11px;cursor:pointer}.platform-tabs button.active{border-color:#354238;color:var(--ns-ink);font-weight:700}.platform-spin{display:block;min-height:420px}.usage-tools,.toolbar{min-height:58px;display:flex;align-items:center;justify-content:space-between;gap:18px}.usage-tools>div,.toolbar>div{display:flex;flex-direction:column}.usage-tools strong,.toolbar strong{font-size:13px}.usage-tools span,.toolbar span{margin-top:4px;color:var(--ns-ink-faint);font-size:9px}.stat-band{display:grid;grid-template-columns:repeat(4,1fr);border:1px solid var(--ns-line);border-radius:8px;background:#fff;overflow:hidden}.stat-band>div{padding:20px 22px;border-right:1px solid var(--ns-line)}.stat-band>div:last-child{border:0}.stat-band span{display:block;color:var(--ns-ink-faint);font-size:9px}.stat-band strong{display:block;margin-top:8px;font-size:24px}.stat-band strong i{font-size:10px;font-style:normal;color:var(--ns-ink-faint)}.danger,.danger-text{color:#ad4a42!important}.usage-grid{display:grid;grid-template-columns:1fr 1fr;gap:18px;margin-top:18px}.data-section{border:1px solid var(--ns-line);border-radius:8px;background:#fff;overflow:hidden}.data-section>header{height:62px;padding:0 18px;display:flex;align-items:center;gap:10px;border-bottom:1px solid var(--ns-line)}.data-section>header>svg{color:#718067}.data-section>header div{display:flex;flex-direction:column}.data-section>header strong{font-size:11px}.data-section>header span{margin-top:3px;color:var(--ns-ink-faint);font-size:9px}.data-table{overflow:auto}.data-table table{width:100%;border-collapse:collapse;font-size:10px}.data-table th{height:36px;padding:0 15px;background:#f6f7f3;color:var(--ns-ink-faint);font-size:9px;text-align:left}.data-table td{height:45px;padding:8px 15px;border-top:1px solid #edf0ea;color:var(--ns-ink-soft)}.mono{font-family:ui-monospace,SFMono-Regular,Consolas,monospace}.empty{text-align:center!important;color:var(--ns-ink-faint)!important}.endpoint-section{margin-top:2px}.endpoint-list article{min-height:82px;display:grid;grid-template-columns:8px minmax(260px,1fr) 90px auto;align-items:center;gap:15px;padding:12px 18px;border-bottom:1px solid #edf0ea}.endpoint-list article:last-child{border:0}.health-dot{width:7px;height:7px;border-radius:50%;background:#69825a}.health-dot.off{background:#a8aaa5}.endpoint-main{min-width:0;display:flex;flex-direction:column}.endpoint-main strong{font-size:11px}.endpoint-main button{width:max-content;max-width:100%;margin:5px 0;padding:0;border:0;background:transparent;color:#647462;font-size:9px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;cursor:pointer}.endpoint-main small{color:var(--ns-ink-faint);font-size:8px}.endpoint-state{display:flex;flex-direction:column}.endpoint-state span{color:var(--ns-ink-faint);font-size:8px}.endpoint-state strong{margin-top:4px;font-size:11px}.row-actions{display:flex;align-items:center;gap:5px;white-space:nowrap}.row-actions button{width:31px;height:31px;display:inline-grid;place-items:center;border:1px solid var(--ns-line);border-radius:50%;background:#fff;color:var(--ns-ink-soft);cursor:pointer}.delivery-section{margin-top:18px}.error-cell{max-width:240px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.badge{padding:4px 8px;border-radius:999px;background:#f0f1ed;font-size:8px}.badge.delivered{background:#e6eee1;color:#58704d}.badge.failed{background:#f7e7e4;color:#a1453d}.workflow-list{border-top:1px solid var(--ns-line)}.workflow-list article{min-height:92px;display:grid;grid-template-columns:42px minmax(260px,1fr) auto auto;align-items:center;gap:15px;padding:14px 8px;border-bottom:1px solid var(--ns-line)}.workflow-mark{width:40px;height:40px;display:grid;place-items:center;border-radius:50%;background:#e7eadf;color:#67765f}.workflow-list article>div:nth-child(2){min-width:0;display:flex;flex-direction:column}.workflow-list article>div:nth-child(2)>span{color:#8b762a;font-size:8px}.workflow-list article strong{margin-top:4px;font-size:12px}.workflow-list p{margin:5px 0 0;color:var(--ns-ink-soft);font-size:9px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.workflow-list article>small{color:var(--ns-ink-faint);font-size:9px}.empty-panel{height:120px;display:grid;place-items:center;color:var(--ns-ink-faint);font-size:10px}.form-stack{display:flex;flex-direction:column;gap:16px}.form-stack label{display:flex;flex-direction:column;gap:7px}.form-stack label>span{font-size:10px;font-weight:650}.form-stack .inline-label{flex-direction:row;align-items:center;justify-content:space-between}.form-grid{display:grid;grid-template-columns:1fr 180px;gap:12px}
@media(max-width:900px){.usage-grid{grid-template-columns:1fr}.stat-band{grid-template-columns:1fr 1fr}.stat-band>div:nth-child(2){border-right:0}.stat-band>div:nth-child(-n+2){border-bottom:1px solid var(--ns-line)}}@media(max-width:620px){.platform-head p{display:none}.usage-tools,.toolbar{align-items:flex-start;flex-direction:column;padding-bottom:12px}.stat-band{grid-template-columns:1fr}.stat-band>div{border-right:0;border-bottom:1px solid var(--ns-line)}.endpoint-list article{grid-template-columns:8px 1fr}.endpoint-state,.row-actions{grid-column:2}.workflow-list article{grid-template-columns:42px 1fr}.workflow-list article>small,.workflow-list .row-actions{grid-column:2}.form-grid{grid-template-columns:1fr}}
</style>

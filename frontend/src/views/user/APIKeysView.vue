<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconCopy, IconDelete, IconPlus, IconRefresh } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const fullKey = ref('')
const keyPreview = ref('')
const hasKey = ref(false)
const visible = ref(false)
const loading = ref(false)

function maskedKey() {
  if (fullKey.value) return `${fullKey.value.slice(0, 8)}••••••••••••${fullKey.value.slice(-4)}`
  if (keyPreview.value) return `sk-••••••••••••${keyPreview.value.replace(/^…/, '')}`
  return '暂无 API Key'
}

async function load() {
  const response = await api('/auth/api-key')
  if (!response.ok) return Message.error(response.data?.detail || 'API Key 加载失败')
  const value = response.data?.data?.key ?? response.data?.key
  hasKey.value = Boolean(value)
  if (typeof value === 'string') {
    fullKey.value = value
    keyPreview.value = value.slice(-4)
    auth.setApiKey(value)
    return
  }
  fullKey.value = typeof auth.apiKey === 'string' ? auth.apiKey : ''
  keyPreview.value = typeof value?.key_preview === 'string' ? value.key_preview : ''
}

async function createKey() {
  loading.value = true
  const response = await api('/auth/api-key', { method: 'POST' })
  loading.value = false
  if (!response.ok) return Message.error(response.data?.detail || '创建失败')
  const value = response.data?.data?.key ?? response.data?.key
  if (typeof value !== 'string') return Message.error('服务端未返回有效密钥')
  fullKey.value = value
  keyPreview.value = String(response.data?.preview || value.slice(-4))
  hasKey.value = true
  auth.setApiKey(value)
  visible.value = true
  Message.success('API Key 已创建')
}

async function revoke() { Modal.warning({ title: '撤销 API Key', content: '撤销后当前凭证将立即失效，确认继续吗？', onOk: async () => { const response = await api('/auth/api-key', { method: 'DELETE' }); if (!response.ok) return Message.error(response.data?.detail || '撤销失败'); fullKey.value = ''; keyPreview.value = ''; hasKey.value = false; auth.setApiKey(''); Message.success('API Key 已撤销') } }) }
async function copy() { if (!fullKey.value) return Message.warning('完整密钥仅在创建或轮换后显示'); await navigator.clipboard.writeText(fullKey.value); Message.success('API Key 已复制') }
onMounted(load)
</script>

<template><div><div class="section-heading"><div><h2>API Keys</h2><p>用于 OpenAI 兼容接口鉴权。凭证只在创建后完整展示。</p></div><a-button type="primary" :loading="loading" @click="createKey"><IconPlus />{{ hasKey ? '轮换密钥' : '创建密钥' }}</a-button></div><section class="key-panel"><div class="key-head"><div><span class="key-icon">sk</span><div><strong>Default API key</strong><small>{{ hasKey ? '当前凭证可用于 OpenAI 兼容接口' : '尚未创建凭证' }}</small></div></div><a-tag :color="hasKey ? 'green' : 'gray'">{{ hasKey ? '正常' : '未创建' }}</a-tag></div><div class="key-value"><code>{{ visible && fullKey ? fullKey : maskedKey() }}</code><a-space><a-button :disabled="!fullKey" @click="visible = !visible">{{ visible ? '隐藏' : '显示' }}</a-button><a-button class="frontend-icon-button" :disabled="!fullKey" title="复制" @click="copy"><IconCopy /></a-button></a-space></div><div class="key-footer"><span>{{ fullKey ? '完整密钥仅在本次创建后可见，请及时保存' : '权限：图像生成、视频生成、模型列表' }}</span><a-space><a-button type="text" size="small" :disabled="loading" @click="createKey"><IconRefresh />轮换</a-button><a-button type="text" status="danger" size="small" :disabled="!hasKey" @click="revoke"><IconDelete />撤销</a-button></a-space></div></section><section class="security"><h3>调用安全</h3><div><strong>服务端保存凭证</strong><p>API Key 应保存在服务端环境变量或 Secret 管理系统中。</p></div><div><strong>设置请求幂等键</strong><p>重试请求时复用同一个 Idempotency-Key，避免重复创建任务。</p></div><div><strong>定期轮换</strong><p>轮换后旧凭证立即失效，请同步更新你的服务端配置。</p></div></section></div></template>
<style scoped>.key-panel{border:1px solid var(--ns-line);background:#fff;border-radius:var(--ns-radius);overflow:hidden}.key-head,.key-footer{padding:17px 20px;display:flex;align-items:center;justify-content:space-between}.key-head>div{display:flex;align-items:center;gap:11px}.key-head>div>div{display:flex;flex-direction:column}.key-head small{font-size:10px;color:var(--ns-ink-faint);margin-top:4px}.key-icon{width:34px;height:34px;display:grid;place-items:center;border-radius:5px;background:#e5eae1;color:var(--ns-accent-strong);font:700 12px ui-monospace}.key-value{padding:18px 20px;border-block:1px solid var(--ns-line);background:#f7f7f4;display:flex;align-items:center;justify-content:space-between;gap:18px}.key-value code{font-size:13px;word-break:break-all}.key-footer{font-size:11px;color:var(--ns-ink-soft)}.security{margin-top:28px;border-top:1px solid var(--ns-line)}.security h3{font-size:14px;margin:22px 0}.security>div{display:grid;grid-template-columns:230px 1fr;padding:14px 0;border-top:1px solid var(--ns-line)}.security strong{font-size:12px}.security p{margin:0;color:var(--ns-ink-soft);font-size:12px}@media(max-width:650px){.key-value,.key-footer{align-items:flex-start;flex-direction:column}.security>div{grid-template-columns:1fr;gap:7px}}</style>

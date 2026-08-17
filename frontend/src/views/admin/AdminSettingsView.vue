<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCheckCircle, IconDelete, IconExclamationCircle, IconFileImage as IconUpload, IconNotification, IconRefresh } from '@arco-design/web-vue/es/icon'
import { api, imageUrl } from '../../services/api'
import { useSiteStore } from '../../stores/site'

const siteStore = useSiteStore()
const loadingData = ref(false)
const busy = ref('')
const pendingLogo = ref('')
const pendingLogoType = ref('')
const currentLogo = ref('')
const activeTab = ref('site')
const smtpTestEmail = ref('')
type SMTPTemplateKey = 'register' | 'reset' | 'welcome'
const smtpTemplateKey = ref<SMTPTemplateKey>('register')
const proxyTesting = ref('')
const proxyResults = reactive<Record<string, { ok: boolean; exitIp?: string; elapsedMs?: number; detail?: string } | null>>({ default: null, image: null, video: null })

const siteForm = reactive({ title: '', subtitle: '', contact: { email: '', qq: '', qq_link: '', qq_group: '', qq_group_link: '', shop: '' } })
const registration = reactive({ open: true, email_code: false, allow_password_reset: true, allowed_email_domains: [] as string[], code_ttl_seconds: 600 })
const smtp = reactive({ host: '', port: 587, username: '', password: '', from_addr: '', use_tls: true })
const smtpTemplates = reactive({
  register: { subject: '', html: '' },
  reset: { subject: '', html: '' },
  welcome: { subject: '', html: '' },
  welcome_enabled: false,
})
const credits = reactive({ registration_reward: 0, checkin_enabled: true, checkin_reward: 3, invite_enabled: true, invite_reward: 3, cdk_redeem_enabled: true })
const deai = reactive({ enabled: false, price_1k: 1, price_2k: 2, price_4k: 3 })
const retention = reactive({ media: 30, logs: 30 })
const announcement = reactive({ content: '' })
const ticker = reactive({ enabled: false, content: '' })
const proxy = reactive({ proxy: '', image_proxy: '', video_proxy: '' })
const pay = reactive({ enabled: false, pid: '', key: '', api_base: '', methods: ['wxpay', 'alipay'] as string[], min_amount: 1, points_ratio: 100, packages: [] as any[] })
const proxyItems = [
  { key: 'default' as const, field: 'proxy' as const, title: '默认代理', note: '账号导入、身份识别、额度同步，以及未单独设置的生成请求' },
  { key: 'image' as const, field: 'image_proxy' as const, title: '图像生成代理', note: '仅覆盖图像生成和图像结果下载' },
  { key: 'video' as const, field: 'video_proxy' as const, title: '视频生成代理', note: '仅覆盖视频生成和视频结果下载' },
]
const logoPreview = computed(() => pendingLogo.value || imageUrl(currentLogo.value))
const smtpConfigured = computed(() => Boolean(smtp.host.trim() && smtp.username.trim() && smtp.from_addr.trim()))
const currentSMTPTemplate = computed(() => smtpTemplates[smtpTemplateKey.value])
const smtpTemplatePlaceholders = computed(() => smtpTemplateKey.value === 'welcome'
  ? ['{{site_name}}', '{{email}}', '{{username}}']
  : ['{{site_name}}', '{{email}}', '{{username}}', '{{code}}', '{{expire_minutes}}'])
const smtpTemplatePreview = computed(() => {
  const values: Record<string, string> = { site_name: siteForm.title || 'Vivid', code: '483 921', expire_minutes: String(Math.max(1, Math.round(registration.code_ttl_seconds / 60))), email: 'creator@example.com', username: 'Creator' }
  let html = currentSMTPTemplate.value.html
  Object.entries(values).forEach(([key, value]) => { html = html.replaceAll(`{{${key}}}`, value) })
  return html
})

async function load() {
  loadingData.value = true
  const [site, reg, smtpResponse, templateResponse, creditResponse, deaiResponse, mediaResponse, logResponse, announcementResponse, tickerResponse, proxyResponse, payResponse] = await Promise.all([
    api('/settings/site'), api('/settings/registration'), api('/settings/smtp'), api('/settings/smtp/templates'),
    api('/settings/credits'), api('/settings/deai'), api('/settings/media'), api('/settings/logs'), api('/settings/announcement'), api('/settings/ticker'), api('/settings/proxy'), api('/settings/pay'),
  ])
  if (site.ok) { siteForm.title = site.data.title || ''; siteForm.subtitle = site.data.subtitle || ''; currentLogo.value = site.data.logo || ''; Object.assign(siteForm.contact, site.data.contact || {}) }
  if (reg.ok) Object.assign(registration, reg.data)
  if (smtpResponse.ok) Object.assign(smtp, smtpResponse.data)
  if (templateResponse.ok) Object.assign(smtpTemplates, templateResponse.data)
  if (creditResponse.ok) Object.assign(credits, creditResponse.data)
  if (deaiResponse.ok) Object.assign(deai, deaiResponse.data)
  if (mediaResponse.ok) retention.media = Number(mediaResponse.data?.retention_days || 30)
  if (logResponse.ok) retention.logs = Number(logResponse.data?.retention_days || 30)
  if (announcementResponse.ok) announcement.content = announcementResponse.data?.content || ''
  if (tickerResponse.ok) Object.assign(ticker, { enabled: Boolean(tickerResponse.data?.enabled), content: tickerResponse.data?.content || '' })
  if (proxyResponse.ok) Object.assign(proxy, proxyResponse.data)
  if (payResponse.ok) Object.assign(pay, payResponse.data, { packages: payResponse.data?.packages || [] })
  loadingData.value = false
}

function chooseLogo(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) return Message.warning('请上传 PNG、JPG 或 WebP 图片')
  if (file.size > 2 * 1024 * 1024) return Message.warning('Logo 文件不能超过 2MB')
  const reader = new FileReader()
  reader.onload = () => { pendingLogo.value = String(reader.result || ''); pendingLogoType.value = file.type }
  reader.readAsDataURL(file)
}

async function removeLogo() {
  if (pendingLogo.value) { pendingLogo.value = ''; pendingLogoType.value = ''; return }
  if (!currentLogo.value) return
  const response = await api('/settings/logo', { method: 'DELETE' })
  if (!response.ok) return Message.error(response.data?.detail || 'Logo 移除失败')
  currentLogo.value = ''
  await siteStore.loadSite(true)
  Message.success('Logo 已移除')
}

async function saveSite() {
  if (!siteForm.title.trim()) return Message.warning('请输入站点名称')
  busy.value = 'site'
  const response = await api('/settings/site', { method: 'PUT', body: JSON.stringify({ title: siteForm.title.trim(), subtitle: siteForm.subtitle.trim(), contact: siteForm.contact }) })
  if (response.ok && pendingLogo.value) {
    const upload = await api('/settings/logo', { method: 'POST', body: JSON.stringify({ data: pendingLogo.value, content_type: pendingLogoType.value }) })
    if (!upload.ok) { busy.value = ''; return Message.error(upload.data?.detail || 'Logo 上传失败') }
    currentLogo.value = upload.data?.logo || ''; pendingLogo.value = ''; pendingLogoType.value = ''
  }
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '站点设置保存失败')
  await siteStore.loadSite(true)
  Message.success('站点品牌已同步到前台')
}

async function saveRegistration() {
  if (registration.email_code && !smtpConfigured.value) return Message.warning('请先保存 SMTP 配置，再开启邮箱验证码')
  busy.value = 'registration'
  const response = await api('/settings/registration', { method: 'PUT', body: JSON.stringify(registration) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '注册策略保存失败')
  Object.assign(registration, response.data?.data || {})
  Message.success('注册与验证策略已保存')
}

async function saveSMTP() {
  if (!smtp.host.trim() || !smtp.username.trim() || !smtp.from_addr.trim()) return Message.warning('请填写 SMTP 主机、用户名和发件地址')
  busy.value = 'smtp'
  const payload: any = { ...smtp, port: Number(smtp.port) || 587 }
  if (payload.password === '***') delete payload.password
  const response = await api('/settings/smtp', { method: 'PUT', body: JSON.stringify(payload) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || 'SMTP 保存失败')
  Object.assign(smtp, response.data?.data || {})
  Message.success('SMTP 配置已保存')
}

async function testSMTP() {
  if (!smtpTestEmail.value.includes('@')) return Message.warning('请输入有效的测试收件邮箱')
  busy.value = 'smtp-test'
  const response = await api('/settings/smtp/test', { method: 'POST', body: JSON.stringify({ email: smtpTestEmail.value.trim() }) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '测试邮件发送失败')
  Message.success(response.data?.detail || '测试邮件已发送')
}

async function saveSMTPTemplates() {
  busy.value = 'smtp-templates'
  const response = await api('/settings/smtp/templates', { method: 'PUT', body: JSON.stringify(smtpTemplates) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '邮件模板保存失败')
  Object.assign(smtpTemplates, response.data?.data || {})
  Message.success('邮件模板已保存并用于实际发送')
}

async function saveCredits() {
  busy.value = 'credits'
  const response = await api('/settings/credits', { method: 'PUT', body: JSON.stringify({
    ...credits,
    registration_reward: Number(credits.registration_reward) || 0,
    checkin_reward: Number(credits.checkin_reward) || 0,
    invite_reward: Number(credits.invite_reward) || 0,
  }) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '积分奖励保存失败')
  Object.assign(credits, response.data?.data || {})
  Message.success('积分奖励规则已生效')
}

async function saveDeAI() {
  busy.value = 'deai'
  const response = await api('/settings/deai', { method: 'PUT', body: JSON.stringify({ enabled: deai.enabled, price_1k: Number(deai.price_1k) || 0, price_2k: Number(deai.price_2k) || 0, price_4k: Number(deai.price_4k) || 0 }) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '去 AI 设置保存失败')
  Object.assign(deai, response.data?.data || {})
  Message.success('去 AI 特征规则已同步到生成工作台')
}

async function saveRetention(type: 'media' | 'logs') {
  busy.value = type
  const response = await api(`/settings/${type}`, { method: 'PUT', body: JSON.stringify({ retention_days: Number(retention[type]) || 30 }) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '留存设置保存失败')
  retention[type] = Number(response.data?.data?.retention_days || retention[type])
  if (type === 'media') Message.success(`作品留存已更新，本次清理 ${response.data?.removed || 0} 个过期文件`)
  else Message.success('日志留存周期已更新')
}

async function saveAnnouncement() {
  busy.value = 'announcement'
  const response = await api('/settings/announcement', { method: 'PUT', body: JSON.stringify({ content: announcement.content }) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '公告保存失败')
  window.dispatchEvent(new Event('announcement-updated'))
  Message.success(announcement.content.trim() ? '公告已发布，将向未读用户展示' : '公告已关闭')
}

async function saveTicker() {
  if (ticker.enabled && !ticker.content.trim()) return Message.warning('开启滚动公告前请先填写内容')
  busy.value = 'ticker'
  const response = await api('/settings/ticker', { method: 'PUT', body: JSON.stringify({ enabled: ticker.enabled, content: ticker.content }) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '滚动公告保存失败')
  Object.assign(ticker, response.data?.data || {})
  window.dispatchEvent(new Event('ticker-updated'))
  Message.success(ticker.enabled ? '顶部滚动公告已发布' : '顶部滚动公告已关闭')
}

async function testProxy(type: 'default' | 'image' | 'video') {
  const key = type === 'default' ? 'proxy' : `${type}_proxy`
  const value = String(proxy[key as keyof typeof proxy] || '').trim()
  proxyResults[type] = null
  if (!value) return Message.warning('请先填写需要测试的代理地址')
  proxyTesting.value = type
  const response = await api('/settings/proxy/test', { method: 'POST', body: JSON.stringify({ proxy: value }) })
  proxyTesting.value = ''
  proxyResults[type] = response.ok
    ? { ok: true, exitIp: response.data?.data?.exit_ip || '未返回', elapsedMs: Number(response.data?.data?.elapsed_ms || 0) }
    : { ok: false, detail: response.data?.detail || '代理连通性测试失败' }
}

async function saveProxy() {
  busy.value = 'proxy'
  const response = await api('/settings/proxy', { method: 'PUT', body: JSON.stringify(proxy) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '代理设置保存失败')
  Object.assign(proxy, response.data?.data || {})
  Message.success('代理策略已保存')
}

function addPayPackage() {
  pay.packages.push({ id: `package-${Date.now()}`, name: '新套餐', amount: 10, points: 1000, enabled: true, sort: pay.packages.length + 1 })
}
function removePayPackage(index: number) { pay.packages.splice(index, 1) }
async function savePay() {
  busy.value = 'pay'
  const response = await api('/settings/pay', { method: 'PUT', body: JSON.stringify({ ...pay, min_amount: Number(pay.min_amount) || 0, points_ratio: Number(pay.points_ratio) || 1, packages: pay.packages.map((item, index) => ({ ...item, amount: Number(item.amount) || 0, points: Number(item.points) || 0, sort: index + 1 })) }) })
  busy.value = ''
  if (!response.ok) return Message.error(response.data?.detail || '支付设置保存失败')
  Message.success('支付设置与充值套餐已保存')
}

onMounted(load)
</script>

<template>
  <div class="settings">
    <div class="section-heading"><div><span class="eyebrow">SYSTEM CONFIGURATION</span><h2>系统设置</h2><p>统一管理站点、注册、奖励、内容与基础设施策略。</p></div></div>
    <a-spin :loading="loadingData" class="settings-body">
      <a-tabs v-model:active-key="activeTab" type="line">
        <a-tab-pane key="site" title="站点品牌">
          <section><div class="section-copy"><h3>品牌标识</h3><p>同步显示在公共首页、工作台侧栏和登录入口。</p></div><div class="logo-editor"><div class="logo-preview"><img v-if="logoPreview" :src="logoPreview" alt="站点 Logo" /><span v-else>{{ siteForm.title.slice(0, 1).toUpperCase() || 'V' }}</span></div><div><strong>站点 Logo</strong><p>PNG、JPG 或 WebP，最大 2MB</p><div class="logo-actions"><label class="upload-command"><IconUpload />选择图片<input type="file" accept="image/png,image/jpeg,image/webp" @change="chooseLogo" /></label><button v-if="logoPreview" class="remove-command" @click="removeLogo"><IconDelete />移除</button></div></div></div></section>
          <section><div class="section-copy"><h3>公开信息</h3><p>用于网页标题、品牌名称、公共首页页脚和运营联系方式。</p></div><a-form :model="siteForm" layout="vertical"><div class="form-grid"><a-form-item label="站点名称" required><a-input v-model="siteForm.title" /></a-form-item><a-form-item label="站点描述"><a-input v-model="siteForm.subtitle" /></a-form-item><a-form-item label="客服邮箱"><a-input v-model="siteForm.contact.email" /></a-form-item><a-form-item label="客服 QQ"><a-input v-model="siteForm.contact.qq" /></a-form-item><a-form-item label="客服 QQ 链接"><a-input v-model="siteForm.contact.qq_link" placeholder="https://qm.qq.com/..." /></a-form-item><a-form-item label="QQ 群"><a-input v-model="siteForm.contact.qq_group" /></a-form-item><a-form-item label="QQ 群链接"><a-input v-model="siteForm.contact.qq_group_link" placeholder="https://qm.qq.com/..." /></a-form-item><a-form-item label="兑换码购买地址"><a-input v-model="siteForm.contact.shop" placeholder="https://shop.example.com" /></a-form-item></div><div class="form-actions"><a-button type="primary" :loading="busy === 'site'" @click="saveSite">保存站点品牌</a-button></div></a-form></section>
        </a-tab-pane>

        <a-tab-pane key="registration" title="注册安全">
          <section><div class="section-copy"><h3>注册策略</h3><p>控制新用户注册、邮箱校验和密码找回。</p></div><div><div class="switches"><label><span><strong>开放注册</strong><small>允许访客创建账号</small></span><a-switch v-model="registration.open" /></label><label><span><strong>注册邮箱验证码</strong><small>开启后注册必须完成邮箱验证</small></span><a-switch v-model="registration.email_code" :disabled="!smtpConfigured" /></label><label><span><strong>允许找回密码</strong><small>通过邮箱验证码重置密码</small></span><a-switch v-model="registration.allow_password_reset" :disabled="!smtpConfigured" /></label></div><a-form :model="registration" layout="vertical" class="compact-form"><div class="form-grid"><a-form-item label="允许的邮箱域名"><a-select v-model="registration.allowed_email_domains" multiple allow-create allow-search placeholder="留空表示不限制，例如 gmail.com" /></a-form-item><a-form-item label="验证码有效期（秒）"><a-input-number v-model="registration.code_ttl_seconds" :min="60" :max="3600" /></a-form-item></div></a-form><div class="form-actions"><a-button type="primary" :loading="busy === 'registration'" @click="saveRegistration">保存注册策略</a-button></div></div></section>
        </a-tab-pane>

        <a-tab-pane key="pay" title="支付设置">
          <section><div class="section-copy"><h3>在线充值</h3><p>配置易支付商户信息、支付方式和额度换算。密钥仅在后台显示。</p></div><div><div class="switches"><label><span><strong>开放在线充值</strong><small>关闭后前台仅保留兑换码充值</small></span><a-switch v-model="pay.enabled" /></label></div><a-form :model="pay" layout="vertical" class="compact-form"><div class="form-grid"><a-form-item label="支付接口地址" required><a-input v-model="pay.api_base" placeholder="https://pay.example.com" /></a-form-item><a-form-item label="商户 ID" required><a-input v-model="pay.pid" /></a-form-item><a-form-item label="商户密钥" required><a-input-password v-model="pay.key" /></a-form-item><a-form-item label="支付方式"><a-select v-model="pay.methods" multiple><a-option value="wxpay">微信</a-option><a-option value="alipay">支付宝</a-option></a-select></a-form-item><a-form-item label="最低充值金额"><a-input-number v-model="pay.min_amount" :min="0" :precision="2" /></a-form-item><a-form-item label="默认换算比例（额度 / 元）"><a-input-number v-model="pay.points_ratio" :min="1" :precision="0" /></a-form-item></div></a-form></div></section>
          <section><div class="section-copy"><h3>充值套餐</h3><p>前台优先展示启用套餐。订单会在服务端按套餐 ID 校验金额与到账额度。</p></div><div><div class="package-editor"><div v-for="(item, index) in pay.packages" :key="item.id || index" class="package-row"><a-input v-model="item.id" placeholder="套餐 ID" /><a-input v-model="item.name" placeholder="套餐名称" /><a-input-number v-model="item.amount" :min="0.01" :precision="2" placeholder="金额" /><a-input-number v-model="item.points" :min="1" :precision="0" placeholder="到账额度" /><a-switch v-model="item.enabled" /><a-button type="text" status="danger" aria-label="删除套餐" @click="removePayPackage(index)"><IconDelete /></a-button></div></div><div class="package-actions"><a-button @click="addPayPackage">新增套餐</a-button><a-button type="primary" :loading="busy === 'pay'" @click="savePay">保存支付设置</a-button></div></div></section>
        </a-tab-pane>

        <a-tab-pane key="smtp" title="SMTP 邮件">
          <section><div class="section-copy"><h3>邮件服务器</h3><p>用于注册验证码、密码找回和系统通知。密码只返回掩码。</p></div><a-form :model="smtp" layout="vertical"><div class="form-grid"><a-form-item label="SMTP 主机" required><a-input v-model="smtp.host" placeholder="smtp.example.com" /></a-form-item><a-form-item label="端口" required><a-input-number v-model="smtp.port" :min="1" :max="65535" /></a-form-item><a-form-item label="用户名" required><a-input v-model="smtp.username" /></a-form-item><a-form-item label="密码"><a-input-password v-model="smtp.password" autocomplete="new-password" /></a-form-item><a-form-item label="发件地址" required><a-input v-model="smtp.from_addr" placeholder="noreply@example.com" /></a-form-item><a-form-item label="TLS 加密"><a-switch v-model="smtp.use_tls" /></a-form-item></div><div class="form-actions"><a-button type="primary" :loading="busy === 'smtp'" @click="saveSMTP">保存 SMTP</a-button></div></a-form></section>
          <section><div class="section-copy"><h3>发送测试</h3><p>测试使用已保存的配置，不会使用尚未保存的表单内容。</p></div><div class="inline-control"><a-input v-model="smtpTestEmail" placeholder="接收测试邮件的邮箱" /><a-button :loading="busy === 'smtp-test'" @click="testSMTP">发送测试邮件</a-button></div></section>
          <section><div class="section-copy"><h3>邮件模板</h3><p>分别管理注册验证、密码找回和注册欢迎信。右侧使用示例数据即时预览。</p></div><div class="template-manager">
            <div class="template-toolbar"><a-radio-group v-model="smtpTemplateKey" type="button"><a-radio value="register">注册验证</a-radio><a-radio value="reset">找回密码</a-radio><a-radio value="welcome">欢迎邮件</a-radio></a-radio-group><label v-if="smtpTemplateKey === 'welcome'" class="welcome-toggle"><span>注册后发送</span><a-switch v-model="smtpTemplates.welcome_enabled" /></label></div>
            <div class="placeholder-list"><span>可用变量</span><code v-for="item in smtpTemplatePlaceholders" :key="item">{{ item }}</code></div>
            <a-form :model="currentSMTPTemplate" layout="vertical"><a-form-item label="邮件主题"><a-input v-model="currentSMTPTemplate.subject" /></a-form-item></a-form>
            <div class="template-grid"><div class="template-editor"><span>HTML 内容</span><a-textarea v-model="currentSMTPTemplate.html" :auto-size="{ minRows: 18, maxRows: 28 }" /></div><div class="template-preview"><span>邮件预览</span><iframe :srcdoc="smtpTemplatePreview" title="邮件模板预览" sandbox="allow-same-origin" /></div></div>
            <div class="form-actions"><a-button type="primary" :loading="busy === 'smtp-templates'" @click="saveSMTPTemplates">保存邮件模板</a-button></div>
          </div></section>
        </a-tab-pane>

        <a-tab-pane key="credits" title="积分奖励">
          <section><div class="section-copy"><h3>新用户奖励</h3><p>用户注册成功后立即进入账户余额，设为 0 表示不赠送。</p></div><div class="number-setting"><span><strong>注册默认积分</strong><small>每个新注册账号只发放一次</small></span><a-input-number v-model="credits.registration_reward" :min="0" :max="100000" /></div></section>
          <section><div class="section-copy"><h3>活跃与邀请</h3><p>签到按天发放；邀请奖励在被邀请人首次成功生成后发放。</p></div><div><div class="switches"><label><span><strong>每日签到</strong><small>允许用户在奖励中心签到</small></span><a-switch v-model="credits.checkin_enabled" /></label><label><span><strong>邀请奖励</strong><small>允许用户通过专属邀请链接拉新</small></span><a-switch v-model="credits.invite_enabled" /></label><label><span><strong>兑换码</strong><small>允许用户兑换运营生成的积分码</small></span><a-switch v-model="credits.cdk_redeem_enabled" /></label></div><div class="form-grid reward-inputs"><a-form-item label="每日签到积分"><a-input-number v-model="credits.checkin_reward" :min="0" /></a-form-item><a-form-item label="每个有效邀请积分"><a-input-number v-model="credits.invite_reward" :min="0" /></a-form-item></div><div class="form-actions"><a-button type="primary" :loading="busy === 'credits'" @click="saveCredits">保存积分规则</a-button></div></div></section>
        </a-tab-pane>

        <a-tab-pane key="deai" title="去 AI 特征">
          <section><div class="section-copy"><h3>图像后处理</h3><p>边缘裁剪、低幅噪声、色调扰动、重编码和元数据剥离，可弱化部分统计特征，但不承诺通过所有 AI 检测器。</p></div><div><div class="switches"><label><span><strong>开放去 AI 特征</strong><small>开启后用户生成工作台显示可选开关</small></span><a-switch v-model="deai.enabled" /></label></div><div class="tier-grid"><label><span>1K 附加积分</span><a-input-number v-model="deai.price_1k" :min="0" /></label><label><span>2K 附加积分</span><a-input-number v-model="deai.price_2k" :min="0" /></label><label><span>4K 附加积分</span><a-input-number v-model="deai.price_4k" :min="0" /></label></div><div class="form-actions"><a-button type="primary" :loading="busy === 'deai'" @click="saveDeAI">保存去 AI 设置</a-button></div></div></section>
        </a-tab-pane>

        <a-tab-pane key="announcement" title="平台公告">
          <section><div class="section-copy"><h3>弹窗公告</h3><p>保存新内容后会向所有尚未阅读该版本的登录用户弹出，用户确认阅读后不再显示。</p></div><div><a-textarea v-model="announcement.content" :max-length="5000" show-word-limit :auto-size="{ minRows: 10, maxRows: 18 }" placeholder="输入维护通知、活动规则或服务变更" /><div v-if="announcement.content.trim()" class="announcement-preview"><strong>弹窗内容预览</strong><p>{{ announcement.content }}</p></div><div class="form-actions"><a-button type="primary" :loading="busy === 'announcement'" @click="saveAnnouncement">{{ announcement.content.trim() ? '发布弹窗公告' : '关闭弹窗公告' }}</a-button></div></div></section>
          <section><div class="section-copy"><h3>顶部滚动公告</h3><p>独立显示在用户页面顶部，对游客和登录用户均可见，不影响弹窗公告的已读状态。</p></div><div><div class="switches"><label><span><strong>显示滚动公告</strong><small>关闭后立即从用户端顶部隐藏</small></span><a-switch v-model="ticker.enabled" /></label></div><a-textarea v-model="ticker.content" class="ticker-editor" :max-length="500" show-word-limit :auto-size="{ minRows: 4, maxRows: 8 }" placeholder="例如：新模型现已上线，活动期间生成可享额度优惠" /><div v-if="ticker.content.trim()" class="ticker-preview"><span><IconNotification /></span><strong>平台公告</strong><p>{{ ticker.content }}</p></div><div class="form-actions"><a-button type="primary" :loading="busy === 'ticker'" @click="saveTicker">保存滚动公告</a-button></div></div></section>
        </a-tab-pane>

        <a-tab-pane key="storage" title="文件存储">
          <section><div class="section-copy"><h3>生成作品留存</h3><p>超过周期的用户图像、视频及缩略图会自动从对象存储删除。</p></div><div class="retention-row"><div><strong>作品保留天数</strong><small>允许范围 1 至 365 天，缩短周期会立即清理过期文件</small></div><a-input-number v-model="retention.media" :min="1" :max="365" /><a-button type="primary" :loading="busy === 'media'" @click="saveRetention('media')">保存</a-button></div></section>
          <section><div class="section-copy"><h3>生成日志留存</h3><p>控制运营日志的保存周期，不影响累计统计数据。</p></div><div class="retention-row"><div><strong>日志保留天数</strong><small>过期明细自动清理，累计生成计数仍会保留</small></div><a-input-number v-model="retention.logs" :min="1" :max="365" /><a-button type="primary" :loading="busy === 'logs'" @click="saveRetention('logs')">保存</a-button></div></section>
        </a-tab-pane>

        <a-tab-pane key="network" title="网络代理">
          <section><div class="section-copy"><h3>出口策略</h3><p>默认代理用于账号识别和额度同步；图像、视频可配置独立生成出口。</p></div><div class="proxy-list"><div v-for="item in proxyItems" :key="item.key" class="proxy-item"><div class="proxy-title"><span><strong>{{ item.title }}</strong><small>{{ item.note }}</small></span><i :class="proxy[item.field] ? 'configured' : ''">{{ proxy[item.field] ? '已配置' : '继承默认' }}</i></div><div class="inline-control"><a-input-password v-model="proxy[item.field]" allow-clear autocomplete="new-password" placeholder="http://user:password@host:port" @input="proxyResults[item.key] = null" /><a-button :loading="proxyTesting === item.key" @click="testProxy(item.key)"><template #icon><IconRefresh /></template>测试</a-button></div><div v-if="proxyResults[item.key]" class="test-result" :class="proxyResults[item.key]?.ok ? 'success' : 'failed'"><IconCheckCircle v-if="proxyResults[item.key]?.ok" /><IconExclamationCircle v-else /><span>{{ proxyResults[item.key]?.ok ? `连接正常 · 出口 IP ${proxyResults[item.key]?.exitIp} · ${proxyResults[item.key]?.elapsedMs} ms` : proxyResults[item.key]?.detail }}</span></div></div><div class="form-actions"><a-button type="primary" :loading="busy === 'proxy'" @click="saveProxy">保存代理策略</a-button></div></div></section>
        </a-tab-pane>
      </a-tabs>
    </a-spin>
  </div>
</template>

<style scoped>
.settings :deep(.arco-tabs-nav-tab-list){flex-wrap:wrap}.settings :deep(.arco-tabs-nav-button){display:none}
.settings{max-width:1120px}.eyebrow{display:block;margin-bottom:6px;color:#8a7628;font-size:9px;font-weight:750;letter-spacing:.13em}.settings-body{width:100%}.settings section{display:grid;grid-template-columns:230px minmax(0,1fr);gap:42px;padding:27px 4px;border-top:1px solid var(--ns-line)}.section-copy h3{font-size:13px;margin:0 0 6px}.section-copy p{max-width:205px;font-size:10px;color:var(--ns-ink-faint);margin:0;line-height:1.7}.logo-editor{min-height:116px;padding:18px;border:1px solid var(--ns-line);border-radius:8px;background:#fff;display:flex;align-items:center;gap:18px}.logo-preview{width:76px;height:76px;flex:0 0 76px;display:grid;place-items:center;border:1px solid var(--ns-line);border-radius:8px;background:#fafaf7;overflow:hidden}.logo-preview img{width:100%;height:100%;object-fit:contain}.logo-preview span{font-size:25px;font-weight:750;color:#8a7628}.logo-editor strong{font-size:12px}.logo-editor p{margin:5px 0 12px;color:var(--ns-ink-faint);font-size:9px}.logo-actions{display:flex;gap:8px}.upload-command,.remove-command{height:32px;padding:0 11px;border:1px solid var(--ns-line);border-radius:6px;background:#fff;color:var(--ns-ink);display:inline-flex;align-items:center;gap:6px;font-size:10px;cursor:pointer}.upload-command input{display:none}.remove-command{color:var(--ns-danger)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 14px}.form-actions{display:flex;justify-content:flex-end;margin-top:12px}.switches label{display:flex;align-items:center;justify-content:space-between;padding:15px 0;border-bottom:1px solid var(--ns-line)}.switches label>span,.number-setting>span,.retention-row>div{display:flex;flex-direction:column}.switches strong,.number-setting strong,.retention-row strong{font-size:11px}.switches small,.number-setting small,.retention-row small{font-size:9px;color:var(--ns-ink-faint);margin-top:4px}.compact-form{margin-top:20px}.inline-control{display:flex;align-items:center;gap:8px}.number-setting{display:flex;align-items:center;justify-content:space-between;gap:20px;padding:8px 0}.number-setting :deep(.arco-input-wrapper),.number-setting :deep(.arco-input-number){width:180px}.reward-inputs{margin-top:20px}.tier-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-top:20px}.tier-grid label{display:flex;flex-direction:column;gap:7px}.tier-grid span{font-size:10px;font-weight:650}.announcement-preview{margin-top:14px;padding:14px 16px;border-left:3px solid #d8bb45;background:#faf9f3}.announcement-preview strong{font-size:10px}.announcement-preview p{white-space:pre-wrap;overflow-wrap:anywhere;margin:6px 0 0;color:var(--ns-ink-soft);font-size:10px;line-height:1.7}.ticker-editor{margin-top:14px}.ticker-preview{height:40px;margin-top:14px;padding:0 14px;display:grid;grid-template-columns:22px auto minmax(0,1fr);align-items:center;gap:8px;overflow:hidden;border:1px solid #ded8b7;background:#f1efdf;color:#3e463c}.ticker-preview>span{width:22px;height:22px;display:grid;place-items:center;border-radius:50%;background:#d8c766}.ticker-preview>span :deep(svg){width:13px}.ticker-preview strong{font-size:10px;white-space:nowrap}.ticker-preview p{margin:0;overflow:hidden;color:#666754;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.retention-row{display:grid;grid-template-columns:minmax(0,1fr) 140px auto;align-items:center;gap:12px}.proxy-list{display:flex;flex-direction:column;gap:18px}.proxy-item{padding-bottom:18px;border-bottom:1px solid var(--ns-line)}.proxy-title{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-bottom:10px}.proxy-title>span{display:flex;flex-direction:column}.proxy-title strong{font-size:11px}.proxy-title small{margin-top:4px;color:var(--ns-ink-faint);font-size:9px}.proxy-title i{padding:4px 8px;border-radius:999px;background:#f0f1ed;color:var(--ns-ink-faint);font-size:8px;font-style:normal}.proxy-title i.configured{background:#eef3df;color:#617126}.test-result{display:flex;align-items:center;gap:7px;margin-top:8px;padding:9px 11px;border:1px solid;border-radius:6px;font-size:9px}.test-result.success{border-color:#d7e1b5;background:#f6f8ed;color:#617126}.test-result.failed{border-color:#ead2ce;background:#fff7f5;color:#a84636}
@media(max-width:760px){.settings section{grid-template-columns:1fr;gap:18px}.section-copy p{max-width:none}.form-grid,.tier-grid{grid-template-columns:1fr}.logo-editor{align-items:flex-start}.inline-control{align-items:stretch;flex-direction:column}.inline-control :deep(.arco-btn){width:100%}.retention-row{grid-template-columns:1fr 1fr}.retention-row>div{grid-column:1/-1}}
.template-manager{min-width:0}.template-toolbar{display:flex;align-items:center;justify-content:space-between;gap:14px;margin-bottom:14px}.welcome-toggle{display:flex;align-items:center;gap:9px;font-size:10px;font-weight:650}.placeholder-list{display:flex;align-items:center;gap:7px;flex-wrap:wrap;margin-bottom:18px}.placeholder-list>span{font-size:9px;color:var(--ns-ink-faint)}.placeholder-list code{padding:4px 7px;border:1px solid var(--ns-line);border-radius:5px;background:#fafaf7;color:#66705f;font-size:9px}.template-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:14px}.template-editor,.template-preview{min-width:0}.template-editor>span,.template-preview>span{display:block;margin-bottom:7px;font-size:10px;font-weight:650}.template-editor :deep(textarea){font-family:Consolas,'Courier New',monospace;font-size:10px;line-height:1.6}.template-preview iframe{display:block;width:100%;height:390px;border:1px solid var(--ns-line);border-radius:7px;background:#f5f5f1}
@media(max-width:760px){.template-grid{grid-template-columns:1fr}.template-toolbar{align-items:flex-start;flex-direction:column}.package-row{grid-template-columns:1fr 1fr}.package-row :deep(.arco-switch){justify-self:start}}
.package-editor{display:flex;flex-direction:column;gap:8px}.package-row{display:grid;grid-template-columns:1.1fr 1.3fr 120px 130px 42px 34px;align-items:center;gap:8px;padding:9px 0;border-bottom:1px solid var(--ns-line)}.package-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:14px}
</style>

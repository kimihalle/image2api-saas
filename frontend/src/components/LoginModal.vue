<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconClose, IconEmail, IconLock, IconUser } from '@arco-design/web-vue/es/icon'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../services/api'
import { useAuthStore } from '../stores/auth'
import { useSiteStore } from '../stores/site'

const auth = useAuthStore()
const site = useSiteStore()
const router = useRouter()
const route = useRoute()
const mode = ref<'login' | 'register' | 'reset'>('login')
const busy = ref(false)
const sendingCode = ref(false)
const cooldown = ref(0)
const error = ref('')
const config = reactive({ open: true, email_code: false, allow_password_reset: false, has_admin: false, invite_enabled: false, registration_reward: 0 })
const form = reactive({ identifier: '', email: '', username: '', password: '', code: '', inviteCode: '' })
const heading = computed(() => mode.value === 'login' ? '欢迎回来' : mode.value === 'register' ? '创建工作区账号' : '找回登录密码')
const needsCode = computed(() => (mode.value === 'register' && config.email_code && config.has_admin) || mode.value === 'reset')

watch(() => auth.loginOpen, async (open) => {
  if (!open) return
  mode.value = auth.startMode
  error.value = ''
  Object.assign(form, { identifier: '', email: '', username: '', password: '', code: '', inviteCode: String(route.query.invite || '') })
  const r = await api('/auth/config')
  if (r.ok) Object.assign(config, r.data)
})

function close() { auth.closeLogin() }
function switchMode(next: 'login' | 'register' | 'reset') { mode.value = next; error.value = '' }

async function sendCode() {
  if (!form.email) { error.value = '请先填写邮箱'; return }
  if (sendingCode.value || cooldown.value) return
  sendingCode.value = true
  const response = await api('/auth/send-code', { method: 'POST', body: JSON.stringify({ email: form.email.trim(), purpose: mode.value === 'reset' ? 'reset' : 'register' }) })
  sendingCode.value = false
  if (!response.ok) { error.value = response.data?.detail || '验证码发送失败'; return }
  cooldown.value = 60
  const timer = window.setInterval(() => { cooldown.value -= 1; if (cooldown.value <= 0) window.clearInterval(timer) }, 1000)
  Message.success('验证码已发送')
}

async function submit() {
  error.value = ''
  if (mode.value === 'login' && (!form.identifier || !form.password)) { error.value = '请输入账号和密码'; return }
  if (mode.value === 'register' && (!form.email || !form.username || !form.password)) { error.value = '请完整填写注册信息'; return }
  if (mode.value === 'reset' && (!form.email || !form.password || !form.code)) { error.value = '请填写邮箱、新密码和验证码'; return }
  busy.value = true
  try {
    if (mode.value === 'reset') {
      const result = await api('/auth/reset-password', { method: 'POST', body: JSON.stringify({ email: form.email.trim(), password: form.password, email_code: form.code.trim() }) })
      if (!result.ok) { error.value = result.data?.detail || '密码重置失败'; return }
      Message.success('密码已重置，请使用新密码登录')
      form.identifier = form.email.trim(); form.password = ''; form.code = ''; mode.value = 'login'
      return
    }
    const result = mode.value === 'login' ? await auth.login(form.identifier.trim(), form.password) : await auth.register(form.email.trim(), form.username.trim(), form.password, form.code.trim(), form.inviteCode.trim())
    if (!result.ok) { error.value = result.data?.detail || result.data?.message || '操作失败，请检查输入'; return }
    close()
    await router.push(auth.isAdmin ? '/admin/overview' : '/')
    Message.success(mode.value === 'login' ? '登录成功' : '账号创建成功')
  } finally { busy.value = false }
}
</script>

<template>
  <a-modal v-model:visible="auth.loginOpen" :footer="false" :closable="false" :width="430" modal-class="user-dialog" @cancel="close">
    <div class="auth-card">
      <button class="close-btn" aria-label="关闭" @click="close"><IconClose /></button>
      <div class="auth-mark"><img v-if="site.logoUrl" :src="site.logoUrl" :alt="site.title" /><span v-else>{{ site.title.slice(0, 1).toUpperCase() }}</span></div>
      <h2>{{ heading }}</h2>
      <p class="auth-sub">{{ mode === 'reset' ? '验证码将发送到你的注册邮箱' : '登录后即可进入完整创作空间' }}</p>
      <div class="auth-tabs">
        <button :class="{ active: mode === 'login' || mode === 'reset' }" @click="switchMode('login')">登录</button>
        <button v-if="config.open || !config.has_admin" :class="{ active: mode === 'register' }" @click="switchMode('register')">注册</button>
      </div>
      <form @submit.prevent="submit">
        <label v-if="mode === 'register'" class="field"><span><IconUser />用户名</span><input v-model="form.username" autocomplete="username" placeholder="6-24 位字母或数字" /></label>
        <label v-if="mode === 'login'" class="field"><span><IconUser />账号</span><input v-model="form.identifier" autocomplete="username" placeholder="邮箱或用户名" /></label>
        <label v-else class="field"><span><IconEmail />邮箱</span><input v-model="form.email" type="email" autocomplete="email" placeholder="name@company.com" /></label>
        <label class="field"><span><IconLock />{{ mode === 'reset' ? '新密码' : '密码' }}</span><input v-model="form.password" type="password" :autocomplete="mode === 'reset' ? 'new-password' : 'current-password'" placeholder="至少 8 位" /></label>
        <label v-if="mode === 'register' && config.invite_enabled" class="field"><span><IconUser />邀请码</span><input v-model="form.inviteCode" autocomplete="off" placeholder="选填，邀请链接会自动填写" /></label>
        <div v-if="needsCode" class="code-row"><label class="field"><span><IconEmail />邮箱验证码</span><input v-model="form.code" inputmode="numeric" maxlength="6" placeholder="请输入验证码" /></label><button type="button" :disabled="sendingCode || cooldown > 0" @click="sendCode">{{ cooldown > 0 ? `${cooldown}s` : sendingCode ? '发送中' : '发送验证码' }}</button></div>
        <button v-if="mode === 'login' && config.allow_password_reset" type="button" class="reset-link" @click="switchMode('reset')">忘记密码</button>
        <p v-if="error" class="error">{{ error }}</p>
        <button class="submit" :disabled="busy"><span v-if="busy" class="spinner"></span>{{ busy ? '处理中' : mode === 'login' ? '登录工作台' : mode === 'register' ? '创建账号' : '重置密码' }}</button>
      </form>
      <p class="auth-foot">{{ site.title }} · OpenAI Compatible API</p>
    </div>
  </a-modal>
</template>

<style scoped>
.auth-card{position:relative;padding:8px 14px 4px;color:var(--ns-ink)}.close-btn{position:absolute;right:4px;top:2px;width:30px;height:30px;border:0;border-radius:50%;background:transparent;color:var(--ns-ink-faint);cursor:pointer}.close-btn:hover{background:#f0f1ed;color:var(--ns-ink)}.auth-mark{width:48px;height:48px;display:grid;place-items:center;border-radius:12px;background:#202620;color:#e7d36c;font-weight:800;font-size:21px;margin:8px auto 14px;overflow:hidden}.auth-mark img{width:100%;height:100%;object-fit:contain;background:#fff}.auth-card h2{text-align:center;font-size:22px;margin:0}.auth-sub{text-align:center;color:var(--ns-ink-faint);font-size:12px;margin:7px 0 22px}.auth-tabs{display:grid;grid-template-columns:1fr 1fr;gap:4px;background:#f0f1ed;padding:4px;border-radius:999px;margin-bottom:18px}.auth-tabs button{height:34px;border:0;background:transparent;color:var(--ns-ink-soft);border-radius:999px;cursor:pointer}.auth-tabs button.active{background:#fff;color:var(--ns-ink);font-weight:650;box-shadow:0 1px 4px rgba(28,34,29,.1)}.field{display:flex;flex-direction:column;gap:7px;margin-bottom:14px}.field span{display:flex;align-items:center;gap:6px;font-size:11px;font-weight:650}.field span :deep(svg){width:14px;color:#a58a28}.field input{height:42px;padding:0 12px;border:1px solid var(--ns-line);border-radius:6px;background:#fafaf7;color:var(--ns-ink);outline:none}.field input:focus{border-color:#a58a28;box-shadow:0 0 0 3px rgba(192,161,50,.14)}.error{margin:4px 0 12px;color:#b34c3d;font-size:11px}.submit{width:100%;height:44px;border:0;border-radius:999px;background:#202620;color:#fff;font-weight:650;cursor:pointer}.submit:hover{background:#3f4f3a}.submit:disabled{opacity:.6;cursor:wait}.spinner{display:inline-block;width:14px;height:14px;border:2px solid rgba(255,255,255,.4);border-top-color:#fff;border-radius:50%;animation:spin .7s linear infinite;margin-right:7px;vertical-align:-2px}.auth-foot{text-align:center;color:var(--ns-ink-faint);font-size:10px;margin:18px 0 4px}@keyframes spin{to{transform:rotate(360deg)}}
.code-row{display:grid;grid-template-columns:1fr 112px;gap:8px;align-items:end}.code-row .field{margin-bottom:14px}.code-row>button{height:42px;margin-bottom:14px;border:1px solid #d5c47e;border-radius:999px;background:#f5f0d9;color:#75631e;font-size:11px;cursor:pointer}.code-row>button:disabled{opacity:.55;cursor:not-allowed}
.reset-link{display:block;margin:-5px 0 12px auto;border:0;background:transparent;color:#75631e;font-size:10px;cursor:pointer}
</style>

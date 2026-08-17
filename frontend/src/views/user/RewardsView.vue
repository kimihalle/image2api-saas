<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCheckCircle, IconCopy, IconGift, IconRefresh, IconUserGroup } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const config = ref<any>({ checkin_enabled: false, checkin_reward: 0, invite_enabled: false, invite_reward: 0 })
const rows = ref<any[]>([])
const loading = ref(false)
const checking = ref(false)
const inviteLink = computed(() => `${window.location.origin}/?invite=${auth.user?.invite_code || ''}`)
function formatTime(value?: string) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '--' }
async function load() {
  loading.value = true
  const [configResponse, inviteResponse, meResponse] = await Promise.all([api('/auth/config'), api('/auth/invites'), api('/auth/me')])
  if (configResponse.ok) config.value = configResponse.data
  if (inviteResponse.ok) rows.value = inviteResponse.data?.data || []
  if (meResponse.ok) auth.setSession(auth.token, meResponse.data.user)
  loading.value = false
}
async function checkin() {
  checking.value = true
  const response = await api('/auth/checkin', { method: 'POST' })
  checking.value = false
  if (!response.ok) return Message.warning(response.data?.detail || '签到失败')
  if (auth.user) { auth.user.credits = response.data.credits; auth.user.checkin_today = true; auth.user.checkin_streak = response.data.streak }
  Message.success(response.data.already ? '今天已经签到' : `签到成功，获得 ${response.data.awarded} 积分`)
}
async function copyLink() { await navigator.clipboard.writeText(inviteLink.value); Message.success('邀请链接已复制') }
onMounted(load)
</script>

<template>
  <div class="rewards-page">
    <div class="section-heading"><div><span class="eyebrow">REWARDS</span><h2>签到与邀请</h2><p>每日签到获得积分，邀请好友完成首次生成后发放邀请奖励。</p></div><a-button :loading="loading" @click="load"><template #icon><IconRefresh /></template>刷新</a-button></div>
    <div class="reward-grid">
      <section class="reward-block"><div class="reward-icon"><IconGift /></div><div><span>每日签到</span><strong>{{ config.checkin_enabled ? `+${config.checkin_reward} 积分` : '暂未开放' }}</strong><p>当前连续签到 {{ auth.user?.checkin_streak || 0 }} 天</p></div><a-button type="primary" :disabled="!config.checkin_enabled || auth.user?.checkin_today" :loading="checking" @click="checkin"><template #icon><IconCheckCircle /></template>{{ auth.user?.checkin_today ? '今日已签到' : '立即签到' }}</a-button></section>
      <section class="reward-block"><div class="reward-icon"><IconUserGroup /></div><div><span>邀请奖励</span><strong>{{ config.invite_enabled ? `每人 +${config.invite_reward} 积分` : '暂未开放' }}</strong><p>已邀请 {{ auth.user?.invite_count || 0 }} 人，累计获得 {{ auth.user?.invite_earned || 0 }} 积分</p></div></section>
    </div>
    <section v-if="config.invite_enabled" class="invite-link"><div><h3>专属邀请链接</h3><p>好友通过此链接注册，并完成首次成功生成后奖励自动到账。</p></div><div class="link-control"><a-input :model-value="inviteLink" readonly /><a-button type="primary" @click="copyLink"><template #icon><IconCopy /></template>复制链接</a-button></div></section>
    <section class="records"><div class="records-head"><div><h3>邀请明细</h3><p>奖励仅在被邀请人完成首次生成后发放。</p></div></div><a-table :data="rows" :loading="loading" :pagination="false"><template #columns><a-table-column title="用户" data-index="name" /><a-table-column title="注册时间"><template #cell="{ record }">{{ formatTime(record.registered_at) }}</template></a-table-column><a-table-column title="奖励"><template #cell="{ record }">{{ record.reward ? `+${record.reward}` : '--' }}</template></a-table-column><a-table-column title="状态"><template #cell="{ record }"><a-tag :color="record.status === 'completed' ? 'green' : 'orange'">{{ record.status === 'completed' ? '已发放' : '待首次生成' }}</a-tag></template></a-table-column></template></a-table></section>
  </div>
</template>

<style scoped>
.rewards-page{max-width:1050px}.eyebrow{display:block;margin-bottom:6px;color:#8a7628;font-size:9px;font-weight:750;letter-spacing:.12em}.reward-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-bottom:18px}.reward-block{min-height:130px;padding:20px;border:1px solid var(--ns-line);border-radius:8px;background:#fff;display:grid;grid-template-columns:42px minmax(0,1fr) auto;align-items:center;gap:15px}.reward-icon{width:42px;height:42px;border-radius:50%;display:grid;place-items:center;background:#f3edd1;color:#8a7628}.reward-block>div:nth-child(2){display:flex;flex-direction:column}.reward-block span{font-size:10px;color:var(--ns-ink-faint)}.reward-block strong{margin-top:4px;font-size:17px}.reward-block p{margin:5px 0 0;font-size:10px;color:var(--ns-ink-soft)}.invite-link{display:grid;grid-template-columns:260px 1fr;gap:34px;padding:24px 0;border-block:1px solid var(--ns-line)}.invite-link h3,.records h3{font-size:13px;margin:0}.invite-link p,.records p{margin:6px 0 0;color:var(--ns-ink-faint);font-size:10px;line-height:1.6}.link-control{display:flex;align-items:center;gap:8px}.records{margin-top:28px;border:1px solid var(--ns-line);border-radius:8px;background:#fff;overflow:hidden}.records-head{padding:17px 18px;border-bottom:1px solid var(--ns-line)}@media(max-width:760px){.reward-grid{grid-template-columns:1fr}.invite-link{grid-template-columns:1fr;gap:14px}.link-control{align-items:stretch;flex-direction:column}.reward-block{grid-template-columns:42px minmax(0,1fr)}.reward-block>.arco-btn{grid-column:1/-1}}
</style>

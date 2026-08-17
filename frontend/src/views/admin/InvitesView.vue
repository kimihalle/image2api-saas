<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { IconRefresh, IconSearch, IconUserGroup } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'

const rows = ref<any[]>([])
const stats = ref<any>({ total: 0, completed: 0, pending: 0, reward_paid: 0 })
const loading = ref(false)
const query = ref('')
const status = ref('all')
const filtered = computed(() => rows.value.filter((row) => {
  const keyword = query.value.trim().toLowerCase()
  const matchesText = !keyword || `${row.inviter} ${row.invitee}`.toLowerCase().includes(keyword)
  return matchesText && (status.value === 'all' || row.status === status.value)
}))
function formatTime(value?: string) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '--' }
async function load() { loading.value = true; const response = await api('/invites'); loading.value = false; if (response.ok) { rows.value = response.data?.data || []; stats.value = response.data?.stats || stats.value } }
onMounted(load)
</script>

<template>
  <div class="invite-page">
    <div class="section-heading"><div><span class="eyebrow">REFERRAL OPERATIONS</span><h2>邀请记录</h2><p>追踪邀请关系、首单生成完成状态与奖励发放。</p></div><a-button :loading="loading" @click="load"><template #icon><IconRefresh /></template>刷新</a-button></div>
    <section class="metrics"><div><span>总邀请</span><strong>{{ stats.total }}</strong></div><div><span>已完成</span><strong>{{ stats.completed }}</strong></div><div><span>待完成</span><strong>{{ stats.pending }}</strong></div><div><span>已发奖励</span><strong>{{ stats.reward_paid }}</strong></div></section>
    <div class="toolbar"><a-input v-model="query" allow-clear placeholder="搜索邀请人或被邀请人"><template #prefix><IconSearch /></template></a-input><a-radio-group v-model="status" type="button"><a-radio value="all">全部</a-radio><a-radio value="completed">已完成</a-radio><a-radio value="pending">待完成</a-radio></a-radio-group></div>
    <div class="table-wrap"><a-table :data="filtered" :loading="loading" :pagination="{ pageSize: 20, showTotal: true }" row-key="registered_at" :scroll="{ x: 860 }"><template #columns><a-table-column title="邀请人" data-index="inviter" :width="180" /><a-table-column title="被邀请人" data-index="invitee" :width="180" /><a-table-column title="注册时间" :width="190"><template #cell="{ record }">{{ formatTime(record.registered_at) }}</template></a-table-column><a-table-column title="完成时间" :width="190"><template #cell="{ record }">{{ formatTime(record.completed_at) }}</template></a-table-column><a-table-column title="奖励" :width="100"><template #cell="{ record }"><strong :class="{ rewarded: record.reward }">{{ record.reward ? `+${record.reward}` : '--' }}</strong></template></a-table-column><a-table-column title="状态" :width="110"><template #cell="{ record }"><a-tag :color="record.status === 'completed' ? 'green' : 'orange'">{{ record.status === 'completed' ? '已完成' : '待首次生成' }}</a-tag></template></a-table-column></template><template #empty><div class="empty"><IconUserGroup /><span>暂无邀请记录</span></div></template></a-table></div>
  </div>
</template>

<style scoped>
.invite-page{max-width:1200px}.eyebrow{display:block;margin-bottom:6px;color:#8a7628;font-size:9px;font-weight:750;letter-spacing:.12em}.metrics{display:grid;grid-template-columns:repeat(4,1fr);border-block:1px solid var(--ns-line);margin-bottom:18px}.metrics>div{padding:17px 20px;border-right:1px solid var(--ns-line);display:flex;flex-direction:column}.metrics>div:last-child{border:0}.metrics span{font-size:9px;color:var(--ns-ink-faint)}.metrics strong{margin-top:5px;font-size:21px}.toolbar{display:grid;grid-template-columns:minmax(240px,380px) auto;justify-content:space-between;gap:12px;margin-bottom:14px}.table-wrap{border:1px solid var(--ns-line);border-radius:8px;overflow:hidden;background:#fff}.rewarded{color:#617126}.empty{height:180px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:8px;color:var(--ns-ink-faint)}.empty svg{width:26px;height:26px}@media(max-width:650px){.metrics{grid-template-columns:1fr 1fr}.metrics>div:nth-child(2){border-right:0}.toolbar{grid-template-columns:1fr}}
</style>

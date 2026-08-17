<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { IconNotification } from '@arco-design/web-vue/es/icon'
import { api } from '../services/api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const visible = ref(false)
const content = ref('')
const version = ref('')

async function load(force = false) {
  if (!auth.isAuthed) return
  const response = await api('/announcement')
  if (!response.ok || !String(response.data?.content || '').trim()) return
  content.value = response.data.content
  version.value = response.data.version || ''
  if (force || !response.data.seen) visible.value = true
}
async function close() {
  visible.value = false
  if (version.value) await api('/announcement/seen', { method: 'POST', body: JSON.stringify({ version: version.value }) })
}
function openFromEvent() { load(true) }
function refreshFromEvent() { load() }
watch(() => auth.isAuthed, (ready) => { if (ready) load() }, { immediate: true })
let refreshTimer = 0
onMounted(() => {
  window.addEventListener('open-announcement', openFromEvent)
  window.addEventListener('announcement-updated', refreshFromEvent)
  refreshTimer = window.setInterval(() => load(), 60000)
})
onBeforeUnmount(() => {
  window.removeEventListener('open-announcement', openFromEvent)
  window.removeEventListener('announcement-updated', refreshFromEvent)
  window.clearInterval(refreshTimer)
})
</script>

<template>
  <a-modal v-model:visible="visible" :footer="false" :closable="false" :width="560" modal-class="user-dialog" @cancel="close">
    <div class="announcement">
      <span class="announcement-icon"><IconNotification /></span>
      <div><span class="eyebrow">PLATFORM NOTICE</span><h2>平台公告</h2></div>
      <div class="announcement-content">{{ content }}</div>
      <a-button type="primary" long @click="close">我知道了</a-button>
    </div>
  </a-modal>
</template>

<style scoped>
.announcement{padding:8px 10px 4px}.announcement-icon{width:42px;height:42px;display:grid;place-items:center;border-radius:50%;background:#f3edd1;color:#8a7628;margin-bottom:14px}.announcement-icon svg{width:20px;height:20px}.eyebrow{display:block;color:#8a7628;font-size:8px;font-weight:750;letter-spacing:.12em}.announcement h2{margin:4px 0 0;font-size:20px}.announcement-content{margin:20px 0;padding:18px 0;border-block:1px solid var(--ns-line);white-space:pre-wrap;overflow-wrap:anywhere;color:var(--ns-ink-soft);font-size:12px;line-height:1.8}
</style>

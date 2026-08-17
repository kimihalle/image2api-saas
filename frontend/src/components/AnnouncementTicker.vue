<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { IconClose, IconNotification } from '@arco-design/web-vue/es/icon'
import { api } from '../services/api'

const content = ref('')
const version = ref('')
const dismissed = ref(false)

const tickerText = computed(() => content.value
  .split(/\r?\n/)
  .map(line => line.trim().replace(/^#{1,6}\s*/, '').replace(/^[-*]\s*/, ''))
  .filter(Boolean)
  .join('  ·  '))

const visible = computed(() => !dismissed.value && !!tickerText.value)

async function load() {
  const response = await api('/ticker')
  if (!response.ok) return
  const nextVersion = String(response.data?.version || '')
  content.value = response.data?.enabled ? String(response.data?.content || '') : ''
  version.value = nextVersion
  dismissed.value = !!nextVersion && localStorage.getItem('announcement_ticker_dismissed') === nextVersion
}

function dismiss() {
  dismissed.value = true
  if (version.value) localStorage.setItem('announcement_ticker_dismissed', version.value)
}

let refreshTimer = 0
onMounted(() => {
  load()
  window.addEventListener('ticker-updated', load)
  refreshTimer = window.setInterval(load, 60000)
})
onBeforeUnmount(() => {
  window.removeEventListener('ticker-updated', load)
  window.clearInterval(refreshTimer)
})
</script>

<template>
  <div v-if="visible" class="announcement-ticker" role="status" aria-label="平台公告">
    <div class="ticker-lead">
      <span class="ticker-icon"><IconNotification /></span>
      <strong>平台公告</strong>
    </div>
    <div class="ticker-window">
      <span class="ticker-track">
        <span>{{ tickerText }}</span><i aria-hidden="true"></i><span aria-hidden="true">{{ tickerText }}</span><i aria-hidden="true"></i>
      </span>
    </div>
    <button class="ticker-close" type="button" aria-label="关闭公告" @click="dismiss"><IconClose /></button>
  </div>
</template>

<style scoped>
.announcement-ticker{height:40px;display:grid;grid-template-columns:auto minmax(0,1fr) 34px;align-items:center;padding:0 27px 0 34px;border-bottom:1px solid rgba(173,154,77,.2);background:#f1efdf;color:#3e463c;position:sticky;top:76px;z-index:14;overflow:hidden}.ticker-close{border:0;background:transparent;color:inherit;cursor:pointer}.ticker-lead{height:100%;display:flex;align-items:center;gap:8px;padding:0 18px 0 0}.ticker-lead strong{font-size:10px;white-space:nowrap}.ticker-icon{width:22px;height:22px;display:grid;place-items:center;border-radius:50%;background:#d8c766;color:#4c4d29}.ticker-icon :deep(svg){width:13px;height:13px}.ticker-window{min-width:0;height:100%;display:flex;align-items:center;overflow:hidden;mask-image:linear-gradient(90deg,transparent,#000 18px,#000 calc(100% - 18px),transparent)}.ticker-track{width:max-content;display:flex;align-items:center;gap:28px;white-space:nowrap;animation:ticker-scroll 24s linear infinite;will-change:transform}.ticker-window:hover .ticker-track{animation-play-state:paused}.ticker-track span{font-size:10px;color:#666754}.ticker-track i{width:4px;height:4px;flex:0 0 4px;border-radius:50%;background:#b59b30}.ticker-close{width:30px;height:30px;display:grid;place-items:center;border-radius:50%;color:#74766d}.ticker-close:hover{background:rgba(255,255,255,.68);color:#292f2a}.ticker-close :deep(svg){width:14px;height:14px}@keyframes ticker-scroll{from{transform:translateX(0)}to{transform:translateX(calc(-50% - 16px))}}@media(prefers-reduced-motion:reduce){.ticker-track{animation:none}}@media(max-width:900px){.announcement-ticker{top:68px;padding:0 10px 0 16px}.ticker-lead{padding-right:10px}.ticker-lead strong{display:none}.ticker-track{animation-duration:20s}}@media(max-width:520px){.announcement-ticker{height:38px;grid-template-columns:auto minmax(0,1fr) 30px}.ticker-icon{width:20px;height:20px}.ticker-track span{font-size:9px}}
</style>

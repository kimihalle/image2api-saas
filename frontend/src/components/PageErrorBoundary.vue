<script setup lang="ts">
import { onErrorCaptured, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { IconExclamationCircle, IconRefresh } from '@arco-design/web-vue/es/icon'

const route = useRoute()
const failed = ref(false)

onErrorCaptured((error) => {
  console.error('page render failed', error)
  failed.value = true
  return false
})

watch(() => route.fullPath, () => {
  failed.value = false
})

function retry() {
  window.location.reload()
}
</script>

<template>
  <div v-if="failed" class="page-error" role="alert">
    <span class="error-icon"><IconExclamationCircle /></span>
    <div>
      <h2>页面加载失败</h2>
      <p>页面资源可能刚刚更新，请重新加载后继续。</p>
    </div>
    <a-button type="primary" @click="retry"><IconRefresh />重新加载</a-button>
  </div>
  <slot v-else />
</template>

<style scoped>
.page-error{min-height:360px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;color:var(--ns-ink)}
.error-icon{width:42px;height:42px;display:grid;place-items:center;border-radius:50%;background:#f3ead1;color:#9a7410}.error-icon :deep(svg){width:21px;height:21px}
.page-error h2{margin:16px 0 6px;font-size:17px}.page-error p{margin:0 0 18px;color:var(--ns-ink-soft);font-size:12px}
</style>

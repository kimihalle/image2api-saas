<script setup lang="ts">
import { useSiteStore } from '../stores/site'
defineProps<{ compact?: boolean }>()
const site = useSiteStore()
</script>
<template>
  <div class="brand" :class="{ compact }">
    <img v-if="site.logoUrl" class="brand-logo" :src="site.logoUrl" :alt="site.title" />
    <span v-else class="mark"><i></i><i></i><i></i></span>
    <span v-if="!compact" class="wordmark">{{ site.title }}</span>
  </div>
</template>
<style scoped>
.brand { display:flex; align-items:center; gap:11px; min-width:0; }
.brand-logo { width:28px; height:28px; display:block; object-fit:contain; border-radius:6px; }
.mark { width:28px; height:28px; display:grid; grid-template-columns:repeat(3, 1fr); align-items:end; gap:2px; padding:5px; background:#202521; border-radius:6px; }
.mark i { display:block; background:#dce6d4; border-radius:1px; }
.mark i:nth-child(1){height:8px}.mark i:nth-child(2){height:16px}.mark i:nth-child(3){height:12px}
.wordmark { max-width:150px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-weight:680; font-size:16px; letter-spacing:0; color:var(--ns-ink); }
</style>

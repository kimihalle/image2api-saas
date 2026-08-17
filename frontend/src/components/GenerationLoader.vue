<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  kind?: 'image' | 'video'
  title: string
  detail?: string
  progress?: number
}>(), {
  kind: 'image',
  detail: '',
  progress: 0,
})

const resolutionCells = Array.from({ length: 36 }, (_, index) => {
  const x = index % 6
  const y = Math.floor(index / 6)
  const distance = Math.abs(x - 2.5) + Math.abs(y - 2.5)
  const tone = 28 + ((x * 13 + y * 9) % 34)
  return {
    index,
    style: {
      '--iml-delay': `${(distance - 6) * 0.055}s`,
      '--iml-tone': `${tone}%`,
    },
  }
})

const progressValue = computed(() => Math.max(0, Math.min(100, Number(props.progress || 0))))
const progressStyle = computed(() => ({ width: progressValue.value > 0 ? `${Math.max(7, progressValue.value)}%` : '34%' }))
</script>

<template>
  <div class="generation-loader" :class="kind" role="status" :aria-label="`${title}${detail ? `，${detail}` : ''}`">
    <span class="resolution-loader" aria-hidden="true">
      <span class="iml-visual">
        <span class="iml-resolution">
          <i v-for="cell in resolutionCells" :key="cell.index" :style="cell.style"></i>
        </span>
      </span>
    </span>
    <div class="loader-copy">
      <strong>{{ title }}</strong>
      <small v-if="detail">{{ detail }}</small>
    </div>
    <div class="loader-progress" :class="{ indeterminate: progressValue === 0 }" aria-hidden="true"><i :style="progressStyle"></i></div>
  </div>
</template>

<style scoped>
.generation-loader{box-sizing:border-box;width:100%;height:100%;min-height:160px;position:relative;overflow:hidden;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:10px;padding:18px;color:#edf1e8;background:#232b25}
.generation-loader.image{height:auto;aspect-ratio:1}

/* Generative Loaders "resolution" animation, adapted under the MIT license. */
.resolution-loader,.resolution-loader *{box-sizing:border-box}
.resolution-loader{--iml-size:clamp(76px,44%,112px);--iml-duration:2.35s;width:var(--iml-size);height:var(--iml-size);flex:0 0 auto;display:inline-grid;place-items:center;color:#e7d36c;line-height:1}
.iml-visual{position:relative;display:block;width:100%;height:100%;overflow:hidden;border-radius:8px;background:color-mix(in srgb,currentColor 3%,transparent);isolation:isolate}
.iml-visual>span{position:absolute;inset:0;display:block}
.iml-visual i{position:absolute;display:block;margin:0;font:inherit}
.iml-resolution{display:grid!important;grid-template-columns:repeat(6,1fr);gap:2%;padding:8%;background:color-mix(in srgb,currentColor 3%,transparent)}
.iml-resolution i{position:relative;width:100%;height:100%;border-radius:16%;background:color-mix(in srgb,currentColor var(--iml-tone),transparent);opacity:.1;transform:scale(.9);animation:iml-resolution var(--iml-duration) cubic-bezier(.4,0,.2,1) infinite;animation-delay:var(--iml-delay)}
@keyframes iml-resolution{0%,18%,100%{opacity:.1;transform:scale(.9);filter:blur(.35px)}46%,70%{opacity:.82;transform:scale(1);filter:blur(0)}84%{opacity:.26;transform:scale(.96);filter:blur(0)}}

.loader-copy{max-width:90%;display:flex;flex-direction:column;align-items:center;gap:4px;text-align:center}
.loader-copy strong{font-size:11px;font-weight:700;letter-spacing:0}
.loader-copy small{max-width:100%;overflow:hidden;color:rgba(231,237,226,.62);font-size:8px;text-overflow:ellipsis;white-space:nowrap}
.loader-progress{width:min(58%,138px);height:3px;overflow:hidden;border-radius:999px;background:rgba(232,238,227,.12)}
.loader-progress i{height:100%;display:block;border-radius:inherit;background:#e7d36c;transition:width .45s ease}
.loader-progress.indeterminate i{animation:progress-travel 1.7s ease-in-out infinite}
.generation-loader.video{min-height:0;aspect-ratio:auto}
.video .resolution-loader{--iml-size:clamp(82px,32%,120px)}
@keyframes progress-travel{0%{transform:translateX(-120%)}50%{transform:translateX(145%)}100%{transform:translateX(330%)}}
@media(max-width:560px){.generation-loader{min-height:138px;padding:12px;gap:7px}.resolution-loader{--iml-size:72px}.video .resolution-loader{--iml-size:68px}.loader-copy small{max-width:160px}}
@media(prefers-reduced-motion:reduce){.iml-resolution i,.loader-progress i{animation:none!important}.iml-resolution i{opacity:.58;transform:scale(1);filter:none}.loader-progress.indeterminate i{width:42%!important}}
</style>

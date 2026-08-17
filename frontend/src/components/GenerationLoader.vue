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

const progressValue = computed(() => Math.max(0, Math.min(100, Number(props.progress || 0))))
const progressStyle = computed(() => ({ width: progressValue.value > 0 ? `${Math.max(7, progressValue.value)}%` : '34%' }))
</script>

<template>
  <div class="generation-loader" :class="kind" role="status" :aria-label="`${title}${detail ? `，${detail}` : ''}`">
    <div class="develop-stage" aria-hidden="true">
      <i class="develop-grid"></i>
      <i class="develop-plane plane-back"></i>
      <i class="develop-plane plane-middle"></i>
      <i class="develop-plane plane-front"></i>
      <i class="develop-scan"></i>
      <i v-for="index in 5" :key="index" :class="`develop-spark spark-${index}`"></i>
      <span v-if="kind === 'video'" class="frame-track"><b v-for="index in 5" :key="index"></b></span>
    </div>
    <div class="loader-copy">
      <strong>{{ title }}</strong>
      <small v-if="detail">{{ detail }}</small>
    </div>
    <div class="loader-progress" :class="{ indeterminate: progressValue === 0 }" aria-hidden="true"><i :style="progressStyle"></i></div>
  </div>
</template>

<style scoped>
.generation-loader{box-sizing:border-box;width:100%;height:100%;min-height:160px;position:relative;isolation:isolate;overflow:hidden;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:9px;padding:18px;color:#edf1e8;background:linear-gradient(145deg,#1e251f 0%,#2b332b 58%,#242a25 100%)}
.generation-loader.image{height:auto;aspect-ratio:1}
.generation-loader::before{content:'';position:absolute;inset:0;z-index:-1;opacity:.46;background-image:linear-gradient(rgba(231,211,108,.05) 1px,transparent 1px),linear-gradient(90deg,rgba(231,211,108,.05) 1px,transparent 1px);background-size:22px 22px;animation:grid-drift 8s linear infinite}
.generation-loader::after{content:'';width:58%;height:1px;position:absolute;left:-58%;top:23%;background:linear-gradient(90deg,transparent,rgba(241,220,114,.62),transparent);box-shadow:0 0 13px rgba(231,211,108,.24);animation:ambient-pass 3.8s ease-in-out infinite}
.develop-stage{width:clamp(76px,44%,112px);aspect-ratio:1;position:relative;flex:0 0 auto;filter:drop-shadow(0 12px 18px rgba(8,12,9,.24))}
.develop-grid{position:absolute;inset:12%;border:1px solid rgba(220,229,211,.14);background-image:linear-gradient(rgba(226,234,218,.08) 1px,transparent 1px),linear-gradient(90deg,rgba(226,234,218,.08) 1px,transparent 1px);background-size:25% 25%}
.develop-plane{position:absolute;border:1px solid rgba(237,241,232,.3);background:#394438;transform-origin:center;animation:plane-develop 3.4s cubic-bezier(.45,0,.2,1) infinite}
.plane-back{inset:7% 21% 25% 8%;opacity:.38;animation-delay:-2.25s}
.plane-middle{inset:18% 8% 11% 24%;border-color:rgba(231,211,108,.42);background:#4d5947;animation-delay:-1.13s}
.plane-front{inset:26% 17% 19% 15%;border-color:rgba(244,227,139,.72);background:#65725d;box-shadow:inset 0 0 0 1px rgba(255,255,255,.05),0 0 18px rgba(231,211,108,.09)}
.develop-scan{height:2px;position:absolute;z-index:3;left:10%;right:10%;top:18%;background:#f1dc72;box-shadow:0 0 12px rgba(241,220,114,.82),0 5px 18px rgba(241,220,114,.25);animation:scan-develop 2.5s cubic-bezier(.4,0,.2,1) infinite}
.develop-spark{width:3px;height:3px;position:absolute;z-index:4;background:#f1dc72;transform:rotate(45deg);box-shadow:0 0 7px rgba(241,220,114,.78);animation:spark 2.4s ease-in-out infinite}
.develop-spark::after{content:'';width:1px;height:9px;position:absolute;left:1px;top:-3px;background:rgba(241,220,114,.65)}
.spark-1{left:8%;top:21%;animation-delay:-.3s}.spark-2{right:10%;top:34%;animation-delay:-1.45s}.spark-3{left:20%;bottom:10%;animation-delay:-.9s}.spark-4{right:24%;bottom:4%;animation-delay:-1.9s}.spark-5{right:3%;top:8%;animation-delay:-2.2s}
.loader-copy{max-width:90%;display:flex;flex-direction:column;align-items:center;gap:4px;text-align:center}.loader-copy strong{font-size:11px;font-weight:700;letter-spacing:0}.loader-copy small{max-width:100%;overflow:hidden;color:rgba(231,237,226,.62);font-size:8px;text-overflow:ellipsis;white-space:nowrap}
.loader-progress{width:min(58%,138px);height:3px;overflow:hidden;border-radius:999px;background:rgba(232,238,227,.12)}.loader-progress i{height:100%;display:block;border-radius:inherit;background:#e7d36c;box-shadow:0 0 10px rgba(231,211,108,.38);transition:width .45s ease}.loader-progress.indeterminate i{animation:progress-travel 1.7s ease-in-out infinite}
.generation-loader.video{min-height:0;aspect-ratio:auto}.video .develop-stage{width:clamp(108px,54%,158px);aspect-ratio:16/9;margin-bottom:3px}.video .plane-back{inset:5% 24% 21% 4%}.video .plane-middle{inset:14% 5% 9% 26%}.video .plane-front{inset:20% 14% 17% 12%}.video .develop-scan{left:5%;right:5%;animation-duration:2.1s}.frame-track{height:9px;position:absolute;z-index:5;left:8%;right:8%;bottom:-4px;display:grid;grid-template-columns:repeat(5,1fr);gap:3px}.frame-track b{border:1px solid rgba(241,220,114,.38);background:#465143;animation:frame-sequence 1.5s ease-in-out infinite}.frame-track b:nth-child(2){animation-delay:.12s}.frame-track b:nth-child(3){animation-delay:.24s}.frame-track b:nth-child(4){animation-delay:.36s}.frame-track b:nth-child(5){animation-delay:.48s}
@keyframes plane-develop{0%,100%{opacity:.28;transform:translate3d(-3px,4px,0) scale(.95)}48%{opacity:.9;transform:translate3d(2px,-2px,0) scale(1.025)}72%{opacity:.56;transform:translate3d(0,0,0) scale(1)}}
@keyframes scan-develop{0%{top:14%;opacity:0}12%{opacity:1}72%{opacity:1}88%,100%{top:84%;opacity:0}}
@keyframes spark{0%,100%{opacity:.15;transform:rotate(45deg) scale(.55)}48%{opacity:1;transform:rotate(45deg) scale(1.2)}}
@keyframes frame-sequence{0%,100%{background:#465143;border-color:rgba(241,220,114,.24)}45%{background:#d8c765;border-color:#f1dc72}}
@keyframes progress-travel{0%{transform:translateX(-120%)}50%{transform:translateX(145%)}100%{transform:translateX(330%)}}
@keyframes grid-drift{to{background-position:22px 22px}}
@keyframes ambient-pass{0%,18%{transform:translateX(0);opacity:0}42%,64%{opacity:1}86%,100%{transform:translateX(280%);opacity:0}}
@media(max-width:560px){.generation-loader{min-height:138px;padding:12px;gap:7px}.develop-stage{width:72px}.video .develop-stage{width:112px}.loader-copy small{max-width:130px}}
@media(prefers-reduced-motion:reduce){.generation-loader::before,.generation-loader::after,.develop-plane,.develop-scan,.develop-spark,.loader-progress i,.frame-track b{animation:none!important}.develop-scan{top:54%;opacity:.65}.loader-progress.indeterminate i{width:42%!important}}
</style>

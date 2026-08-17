<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { IconApps, IconCode, IconCustomerService, IconEmail, IconGift, IconImage, IconQq, IconUserGroup, IconVideoCamera } from '@arco-design/web-vue/es/icon'
import { useRouter } from 'vue-router'
import ModelAtmosphere from '../components/ModelAtmosphere.vue'
import { api, imageUrl } from '../services/api'
import { useAuthStore } from '../stores/auth'
import { useSiteStore } from '../stores/site'

const router = useRouter()
const auth = useAuthStore()
const site = useSiteStore()
const showcase = ref<any>({ hero: [], bento: [], work: [] })
const models = ref<any[]>([])
const stats = ref<any>({})
const loading = ref(true)
const heroIndex = ref(0)
let heroTimer = 0
const heroCards = computed(() => (showcase.value.hero || []).slice(0, 3))
const workCards = computed(() => (showcase.value.work || []).slice(0, 6))
const enabledModels = computed(() => models.value.filter((item) => item.enabled !== false))
const imageModelCount = computed(() => enabledModels.value.filter((item) => item.type === 'image').length)
const videoModelCount = computed(() => enabledModels.value.filter((item) => item.type === 'video').length)
const hasContact = computed(() => Boolean(site.contact.email || site.contact.qq || site.contact.qq_group || site.contact.shop))
const year = new Date().getFullYear()
const avgTime = computed(() => {
  const ms = Number(stats.value.avg_elapsed_ms_24h || stats.value.avg_elapsed_ms || 0)
  return ms ? (ms > 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`) : '暂无'
})
function src(item: any) { return imageUrl(item?.image) }
function start(path = '/app/generate') { auth.isAuthed ? router.push(path) : auth.openLogin(path) }
function safeExternalURL(value: string) {
  try {
    const target = new URL(String(value || '').trim())
    return ['http:', 'https:'].includes(target.protocol) ? target.toString() : ''
  } catch { return '' }
}
function deckPosition(index: number) {
  const total = heroCards.value.length || 1
  return ((index - heroIndex.value + total) % total) + 1
}
function stopHeroRotation() {
  if (heroTimer) window.clearInterval(heroTimer)
  heroTimer = 0
}
function startHeroRotation() {
  stopHeroRotation()
  if (heroCards.value.length < 2) return
  heroTimer = window.setInterval(() => {
    if (!document.hidden) heroIndex.value = (heroIndex.value + 1) % heroCards.value.length
  }, 4500)
}
async function refresh() {
  loading.value = true
  const [showcaseResponse, modelResponse, statsResponse] = await Promise.all([api('/showcase'), api('/models'), api('/stats')])
  if (showcaseResponse.ok) showcase.value = showcaseResponse.data?.data || showcaseResponse.data || showcase.value
  if (modelResponse.ok) models.value = modelResponse.data?.data || modelResponse.data || []
  if (statsResponse.ok) stats.value = statsResponse.data || {}
  heroIndex.value = 0
  loading.value = false
}
onMounted(async () => { await refresh(); startHeroRotation() })
onBeforeUnmount(stopHeroRotation)
</script>

<template>
  <div class="home-page">
    <section class="hero">
      <div class="hero-copy">
        <span class="eyebrow"><i></i> CREATIVE WORKSPACE</span>
        <h2>把想法<br /><em>变成作品</em></h2>
        <p>一个工作台，连接图像、视频和 OpenAI 兼容 API。写下描述，选择模型，剩下的交给可靠的生成服务。</p>
        <div class="hero-actions"><button class="primary" @click="start()"><IconApps />开始创作</button><button class="quiet" @click="start('/app/docs')"><IconCode />查看 API 文档</button></div>
        <div class="signals"><div><strong>{{ enabledModels.length }}</strong><span>已接入模型</span></div><div><strong>{{ Number(stats.generated_count || 0).toLocaleString() }}</strong><span>累计生成</span></div><div><strong>{{ avgTime }}</strong><span>平均出片</span></div></div>
      </div>
      <div class="hero-stage" @mouseenter="stopHeroRotation" @mouseleave="startHeroRotation">
        <template v-if="heroCards.length">
          <figure v-for="(card, index) in heroCards" :key="card.id" :class="['deck-card', `deck-${deckPosition(index)}`, { 'is-front': deckPosition(index) === 1 }]"><img :src="src(card)" :alt="card.title || 'showcase'" /><figcaption><span>{{ card.subtitle || 'SHOWCASE' }}</span><strong>{{ card.title }}</strong></figcaption></figure>
        </template>
        <div v-else class="hero-empty"><IconImage /><strong>{{ loading ? '正在载入首页内容' : '首页展示内容即将上线' }}</strong><span>{{ loading ? '正在读取运营团队配置的作品' : '运营团队可以在后台内容管理中添加作品' }}</span></div>
        <span class="stage-note">CURATED BY {{ site.title.toUpperCase() }}</span>
      </div>
    </section>

    <section class="section work" v-if="workCards.length"><div class="section-head"><div><span class="eyebrow">SELECTED WORK</span><h3>精选作品</h3></div><button class="text-action" @click="start('/app/history')">查看生成记录</button></div><div class="work-grid"><button v-for="(card, index) in workCards" :key="card.id" class="work-card" @click="start()"><img :src="src(card)" :alt="card.title" /><span class="work-caption"><small>{{ String(index + 1).padStart(2, '0') }}</small><strong>{{ card.title }}</strong></span></button></div></section>

    <section class="section models"><div class="section-head"><div><span class="eyebrow">MODEL CAPABILITIES</span><h3>覆盖图片与视频创作</h3></div><button class="text-action" @click="start('/app/docs')">查看接入方式</button></div><div class="model-summary"><article class="image-summary"><ModelAtmosphere tone="image" /><div class="summary-content"><span class="summary-label"><i><IconImage /></i><span><small>IMAGE GENERATION</small><b>图片生成</b></span></span><strong><b>{{ imageModelCount }}</b><em>个模型</em></strong><p>覆盖文生图、参考图创作与多种画面比例</p></div></article><article class="video-summary"><ModelAtmosphere tone="video" /><div class="summary-content"><span class="summary-label"><i><IconVideoCamera /></i><span><small>VIDEO CREATION</small><b>视频创作</b></span></span><strong><b>{{ videoModelCount }}</b><em>个模型</em></strong><p>按模型能力提供比例、时长、清晰度与素材输入</p></div></article></div></section>

    <section class="openai-band"><div><span class="eyebrow">OPENAI COMPATIBLE</span><h3>用熟悉的接口接入 {{ site.title }}</h3><p>统一的鉴权、额度和失败退款逻辑，为产品团队提供清晰稳定的生成服务。</p></div><button class="primary" @click="start('/app/docs')"><IconCode />查看开发文档</button></section>

    <footer class="site-footer"><div class="footer-brand"><strong>{{ site.title }}</strong><p>{{ site.subtitle || '稳定连接图片、视频与 OpenAI 兼容 API。' }}</p><small>© {{ year }} {{ site.title }}</small></div><div v-if="hasContact" class="footer-contact"><span class="footer-label"><IconCustomerService />联系我们</span><a v-if="site.contact.email" :href="`mailto:${site.contact.email}`"><IconEmail /><span>客服邮箱</span><strong>{{ site.contact.email }}</strong></a><a v-if="site.contact.qq" :href="safeExternalURL(site.contact.qq_link) || undefined" :target="safeExternalURL(site.contact.qq_link) ? '_blank' : undefined" rel="noopener noreferrer"><IconQq /><span>客服 QQ</span><strong>{{ site.contact.qq }}</strong></a><a v-if="site.contact.qq_group" :href="safeExternalURL(site.contact.qq_group_link) || undefined" :target="safeExternalURL(site.contact.qq_group_link) ? '_blank' : undefined" rel="noopener noreferrer"><IconUserGroup /><span>QQ 群</span><strong>{{ site.contact.qq_group }}</strong></a><a v-if="site.contact.shop && safeExternalURL(site.contact.shop)" :href="safeExternalURL(site.contact.shop)" target="_blank" rel="noopener noreferrer"><IconGift /><span>兑换码</span><strong>购买兑换码</strong></a></div></footer>
  </div>
</template>

<style scoped>
.home-page{display:flex;flex-direction:column;gap:58px}.hero{display:grid;grid-template-columns:minmax(320px,.86fr) minmax(430px,1.14fr);gap:42px;align-items:center;padding:16px 0 34px}.eyebrow{display:flex;align-items:center;gap:8px;color:#8c7825;font-size:9px;font-weight:760;letter-spacing:.13em}.eyebrow i{width:7px;height:7px;border-radius:50%;background:#c8a536;box-shadow:0 0 0 4px #f1ead2}.hero-copy h2{margin:22px 0 18px;font-size:clamp(48px,6vw,88px);line-height:.94;letter-spacing:0;font-weight:760}.hero-copy h2 em{font-style:normal;color:#8f7a2a}.hero-copy p{max-width:480px;color:var(--ns-ink-soft);font-size:14px;line-height:1.8}.hero-actions{display:flex;gap:10px;align-items:center;margin-top:28px}.primary{height:42px;padding:0 18px;border:1px solid #242a25;border-radius:999px;background:#242a25;color:#fff;display:inline-flex;align-items:center;gap:8px;font-size:12px;font-weight:650;cursor:pointer}.primary:hover{border-color:#3f4f3a;background:#3f4f3a}.quiet,.text-action{height:38px;padding:0 15px;border:1px solid #d8ddd5;border-radius:999px;background:#f3f5f0;color:var(--ns-ink-soft);display:inline-flex;align-items:center;gap:7px;font-size:11px;font-weight:650;cursor:pointer}.quiet:hover,.text-action:hover{border-color:#bcc5b8;background:#e9ede6;color:var(--ns-ink)}.signals{display:grid;grid-template-columns:repeat(3,1fr);max-width:440px;margin-top:52px;border-block:1px solid var(--ns-line)}.signals div{padding:15px 12px 14px 0;border-right:1px solid var(--ns-line);display:flex;flex-direction:column}.signals div+div{padding-left:12px}.signals div:last-child{border-right:0}.signals strong{font-size:23px}.signals span{margin-top:5px;color:var(--ns-ink-faint);font-size:9px}.hero-stage{position:relative;height:530px;min-height:400px}.deck-card{position:absolute;inset-block:12px;right:8%;width:70%;margin:0;overflow:hidden;border-radius:8px;background:#454c45;box-shadow:0 22px 44px rgba(31,36,33,.18);transform-origin:center bottom}.deck-card img{width:100%;height:100%;object-fit:cover;display:block}.deck-card figcaption{position:absolute;left:14px;right:14px;bottom:14px;padding:10px 12px;background:rgba(26,31,27,.84);color:#fff;display:flex;justify-content:space-between;align-items:center;gap:12px}.deck-card figcaption span{font-size:9px;color:#c8d0c4}.deck-card figcaption strong{font-size:11px}.deck-1{z-index:3;transform:rotate(2deg)}.deck-2{z-index:2;right:17%;top:24px;bottom:0;transform:rotate(-5deg);opacity:.9}.deck-3{z-index:1;right:27%;top:42px;bottom:-4px;transform:rotate(8deg);opacity:.74}.hero-stage:hover .deck-1{transform:rotate(0) translateY(-10px)}.hero-stage:hover .deck-2{transform:rotate(-9deg) translate(-24px,10px)}.hero-stage:hover .deck-3{transform:rotate(13deg) translate(-48px,18px)}.stage-note{position:absolute;top:0;left:0;padding:8px 10px;border-radius:999px;background:#dedfca;color:#43503c;font-size:9px;font-weight:700;z-index:4}.hero-empty{height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:10px;border:1px dashed var(--ns-line-strong);border-radius:8px;background:#fafaf7;color:var(--ns-ink-faint);text-align:center}.hero-empty :deep(svg){width:30px;height:30px;color:#b09b43}.hero-empty strong{color:var(--ns-ink-soft);font-size:14px}.hero-empty span{font-size:11px}.section{padding-top:2px}.section-head{display:flex;align-items:end;justify-content:space-between;gap:20px;margin-bottom:20px}.section-head h3{margin:10px 0 0;font-size:28px;line-height:1.15}.work-grid{display:grid;grid-template-columns:repeat(6,1fr);gap:8px}.work-card{position:relative;aspect-ratio:1;border:0;border-radius:6px;overflow:hidden;background:#ddd;padding:0;cursor:pointer}.work-card img{width:100%;height:100%;display:block;object-fit:cover;transition:transform .35s}.work-card:hover img{transform:scale(1.05)}.work-card span{position:absolute;left:8px;right:8px;bottom:8px;padding:7px;background:rgba(25,29,26,.8);color:#fff;text-align:left;font-size:9px}.models{padding-bottom:4px}.model-list{display:grid;grid-template-columns:repeat(3,1fr);border-block:1px solid var(--ns-line)}.model-item{min-height:74px;padding:13px 14px;border-right:1px solid var(--ns-line);display:flex;align-items:center;gap:10px}.model-item:nth-child(3n){border-right:0}.model-icon{width:30px;height:30px;border-radius:50%;display:grid;place-items:center;background:#e7ebe2;color:#8b7728}.model-item div{display:flex;flex-direction:column;min-width:0}.model-item strong{font-size:12px}.model-item small{margin-top:4px;font-size:9px;color:var(--ns-ink-faint)}.available{margin-left:auto;color:#6f805f;font-size:10px}.empty-line{grid-column:1/-1;padding:24px;text-align:center;color:var(--ns-ink-faint);font-size:11px}.openai-band{display:flex;align-items:center;justify-content:space-between;gap:20px;padding:30px 34px;background:#dedfca;border-radius:8px}.openai-band h3{margin:9px 0 7px;font-size:24px}.openai-band p{margin:0;color:#687161;font-size:12px}
@media(max-width:1050px){.hero{grid-template-columns:1fr;gap:28px}.hero-copy h2{font-size:68px}.hero-stage{height:420px}.work-grid{grid-template-columns:repeat(3,1fr)}.model-list{grid-template-columns:repeat(2,1fr)}.model-item:nth-child(3n){border-right:1px solid var(--ns-line)}.model-item:nth-child(2n){border-right:0}}
@media(max-width:620px){.home-page{gap:42px}.hero{padding-top:0}.hero-copy h2{font-size:54px}.hero-copy p{font-size:13px}.hero-actions{flex-wrap:wrap}.signals{margin-top:34px}.hero-stage{height:300px;min-height:280px}.deck-card{right:0;width:76%}.deck-2{right:10%}.deck-3{right:20%}.section-head{align-items:flex-start;flex-direction:column}.section-head h3{font-size:24px}.work-grid{grid-template-columns:repeat(2,1fr)}.model-list{grid-template-columns:1fr}.model-item,.model-item:nth-child(2n),.model-item:nth-child(3n){border-right:0;border-bottom:1px solid var(--ns-line)}.openai-band{align-items:flex-start;flex-direction:column;padding:25px 22px}.openai-band h3{font-size:21px}}
.deck-card{transition:transform .72s cubic-bezier(.22,.8,.24,1),opacity .55s ease,filter .55s ease;will-change:transform,opacity}.deck-card:not(.is-front){filter:saturate(.86)}.deck-card.is-front img{animation:hero-focus 4.5s ease both}@keyframes hero-focus{from{transform:scale(1)}to{transform:scale(1.025)}}
@media(prefers-reduced-motion:reduce){.deck-card{transition:none}.deck-card.is-front img{animation:none}}
.model-summary{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.model-summary article{min-height:190px;padding:27px 29px;display:flex;align-items:flex-start;gap:20px;border:1px solid var(--ns-line);border-radius:8px;position:relative;overflow:hidden}.image-summary{background:#f3eed8}.video-summary{background:#e5ece3}.summary-icon{width:48px;height:48px;flex:0 0 48px;display:grid;place-items:center;border-radius:50%;background:rgba(255,255,255,.7);color:#77661f}.video-summary .summary-icon{color:#4e654d}.summary-icon :deep(svg){width:21px;height:21px}.model-summary article>div{display:flex;flex-direction:column}.model-summary small{color:#747c73;font-size:10px;font-weight:700}.model-summary strong{margin-top:8px;font-size:46px;line-height:1;letter-spacing:0}.model-summary em{margin-left:8px;color:#596159;font-size:12px;font-style:normal;font-weight:650}.model-summary p{max-width:380px;margin:16px 0 0;color:#697068;font-size:11px;line-height:1.7}.site-footer{padding:34px 2px 6px;display:grid;grid-template-columns:minmax(240px,.8fr) minmax(480px,1.2fr);gap:50px;border-top:1px solid var(--ns-line)}.footer-brand{display:flex;flex-direction:column}.footer-brand>strong{font-size:20px}.footer-brand p{max-width:390px;margin:8px 0 20px;color:var(--ns-ink-soft);font-size:11px;line-height:1.7}.footer-brand small{color:var(--ns-ink-faint);font-size:9px}.footer-contact{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.footer-label{grid-column:1/-1;margin-bottom:3px;display:flex;align-items:center;gap:7px;color:var(--ns-ink-soft);font-size:10px;font-weight:700}.footer-contact a{min-width:0;padding:11px 13px;display:grid;grid-template-columns:20px minmax(0,1fr);gap:2px 8px;border:1px solid var(--ns-line);border-radius:7px;background:#fff;color:var(--ns-ink);text-decoration:none}.footer-contact a>svg{grid-row:1/3;align-self:center;width:16px;color:#92791d}.footer-contact a span{color:var(--ns-ink-faint);font-size:8px}.footer-contact a strong{overflow:hidden;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.footer-contact a[href]:hover{border-color:#aeb7aa;background:#f7f8f4}@media(max-width:760px){.model-summary{grid-template-columns:1fr}.model-summary article{min-height:150px;padding:22px}.model-summary strong{font-size:38px}.site-footer{grid-template-columns:1fr;gap:26px;padding-top:28px}.footer-contact{grid-template-columns:1fr}.footer-brand p{margin-bottom:10px}}
/* 首页内容区使用真实作品形成视觉背景，模型信息仍只保留运营数量。 */
.work{padding-block:30px;border-block:1px solid var(--ns-line)}
.work-grid{grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}
.work-card{aspect-ratio:4/3;border-radius:7px;background:#dfe2dc;box-shadow:0 12px 26px rgba(31,36,33,.08)}
.work-card img{transition:transform .42s ease,filter .42s ease}
.work-card:hover img{transform:scale(1.04);filter:saturate(1.06)}
.work-card .work-caption{left:12px;right:12px;bottom:12px;min-height:44px;padding:9px 11px;display:grid;grid-template-columns:24px minmax(0,1fr);align-items:center;gap:9px;background:rgba(25,30,26,.86)}
.work-caption small{color:#c9d0c5;font:600 9px ui-monospace}
.work-caption strong{overflow:hidden;font-size:11px;text-overflow:ellipsis;white-space:nowrap}
.model-summary{gap:12px}
.model-summary article{min-height:260px;padding:0;border:1px solid rgba(255,255,255,.12);background:#20251f;box-shadow:0 14px 30px rgba(31,36,33,.12)}
.summary-content{position:relative;z-index:1;width:100%;min-height:260px;padding:25px 27px;display:flex;flex-direction:column;color:#fff}
.summary-label{display:flex;align-items:center;gap:11px}
.summary-label>i{width:42px;height:42px;display:grid;place-items:center;border-radius:50%;background:rgba(255,255,255,.14);font-style:normal}
.summary-label>i :deep(svg){width:19px;height:19px}
.summary-label>span{display:flex;flex-direction:column;gap:3px}
.model-summary .summary-label small{color:rgba(255,255,255,.58);font-size:8px;letter-spacing:.1em}
.summary-label b{font-size:12px}
.model-summary .summary-content>strong{margin-top:30px;display:flex;align-items:baseline;gap:10px;color:#fff;line-height:1}
.model-summary .summary-content>strong>b{font-size:68px;font-weight:720}
.model-summary .summary-content em{margin:0;color:rgba(255,255,255,.74);font-size:12px}
.model-summary .summary-content p{margin:auto 0 0;color:rgba(255,255,255,.72);font-size:11px}
@media(max-width:760px){.work{padding-block:24px}.work-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.work-card{aspect-ratio:1}.work-card:last-child:nth-child(odd){grid-column:1/-1;aspect-ratio:2/1}.work-card .work-caption{left:8px;right:8px;bottom:8px;grid-template-columns:20px minmax(0,1fr);padding:8px}.model-summary article,.summary-content{min-height:220px}.summary-content{padding:22px}.model-summary .summary-content>strong>b{font-size:54px}}
</style>

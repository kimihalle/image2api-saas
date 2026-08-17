<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCheckCircle, IconCode, IconCopy, IconImage, IconRefresh, IconVideoCamera } from '@arco-design/web-vue/es/icon'
import { api, apiOrigin } from '../../services/api'
import { useSiteStore } from '../../stores/site'

const site = useSiteStore()
const models = ref<any[]>([])
const loadingModels = ref(false)
const imageTab = ref<'curl' | 'python' | 'node'>('curl')
const videoTab = ref<'curl' | 'python' | 'node'>('curl')
const baseUrl = computed(() => apiOrigin())
const imageModels = computed(() => models.value.filter((item) => item.enabled !== false && item.type === 'image'))
const videoModels = computed(() => models.value.filter((item) => item.enabled !== false && item.type === 'video'))
const imageModelID = computed(() => publicModelID(imageModels.value[0]) || 'your-image-model')
const videoModelID = computed(() => publicModelID(videoModels.value[0]) || 'your-video-model')

function values(value: unknown) {
	return Array.isArray(value) ? value.map(String).filter(Boolean) : []
}
function publicModelID(model: any) {
  return String(model?.alias || model?.id || '').trim()
}
function displayName(model: any) {
  return String(model?.name || model?.alias || model?.id || '未命名模型')
}
function capabilitySummary(model: any) {
  const capability = model?.capabilities || {}
  const resolutions = values(model?.resolutions).length ? values(model.resolutions) : Object.keys(model?.prices || {})
  const durations = values(model?.durations).length ? values(model.durations) : Object.keys(model?.duration_prices || {})
  const ratios = values(model?.ratios).length ? values(model.ratios) : values(capability.ratios)
  return [
    resolutions.length ? `分辨率 ${resolutions.join(' / ')}` : '',
    durations.length ? `时长 ${durations.join(' / ')}` : '',
    ratios.length ? `比例 ${ratios.join(' / ')}` : '',
  ].filter(Boolean)
}
function priceSummary(model: any) {
  const entries = Object.entries(model?.type === 'video' ? (model?.duration_prices || {}) : (model?.prices || {}))
  if (!entries.length) return '价格以提交任务时为准'
  return entries.slice(0, 4).map(([key, value]) => `${key} · ${Number(value).toLocaleString()} 额度`).join('　')
}
async function loadModels() {
  loadingModels.value = true
  const response = await api('/models')
  loadingModels.value = false
  if (!response.ok) return Message.error('模型目录加载失败')
  models.value = (response.data?.data || response.data || []).filter((item: any) => item.enabled !== false)
}
async function copy(value: string) {
  await navigator.clipboard.writeText(value)
  Message.success('内容已复制')
}

const imageExamples = computed(() => ({
  curl: `curl ${baseUrl.value}/images/generations \\\n  -H "Authorization: Bearer sk-your-key" \\\n  -H "Content-Type: application/json" \\\n  -d '{"model":"${imageModelID.value}","prompt":"Editorial product photography, soft window light","size":"1024x1024","n":1}'`,
  python: `from openai import OpenAI

client = OpenAI(
    api_key="sk-your-key",
    base_url="${baseUrl.value}"
)

result = client.images.generate(
    model="${imageModelID.value}",
    prompt="Editorial product photography, soft window light",
    size="1024x1024",
    n=1
)
print(result.data[0].b64_json[:80])`,
  node: `import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-your-key",
  baseURL: "${baseUrl.value}"
});

const result = await client.images.generate({
  model: "${imageModelID.value}",
  prompt: "Editorial product photography, soft window light",
  size: "1024x1024",
  n: 1
});
console.log(result.data[0].b64_json.slice(0, 80));`,
}))
const imageCode = computed(() => imageExamples.value[imageTab.value])
const editCode = computed(() => [
  "curl " + baseUrl.value + "/images/edits \\",
  "  -H \"Authorization: Bearer sk-your-key\" \\",
  "  -F \"model=" + imageModelID.value + "\" \\",
  "  -F \"prompt=Keep the product and change the background\" \\",
  "  -F \"image=@reference.png\"",
].join(String.fromCharCode(10)))

const videoExamples = computed(() => ({
  curl: `curl ${baseUrl.value}/videos \\\n  -H "Authorization: Bearer sk-your-key" \\\n  -H "Content-Type: application/json" \\\n  -d '{"model":"${videoModelID.value}","prompt":"A quiet cinematic city street after rain","seconds":5,"ratio":"16:9","resolution":"720p"}'

# 使用创建结果中的 id 查询状态
curl ${baseUrl.value}/videos/VIDEO_ID \\\n  -H "Authorization: Bearer sk-your-key"

# completed 后下载视频
curl ${baseUrl.value}/videos/VIDEO_ID/content \\\n  -H "Authorization: Bearer sk-your-key" -o output.mp4`,
  python: `import time
import requests

base = "${baseUrl.value}"
headers = {"Authorization": "Bearer sk-your-key"}
job = requests.post(f"{base}/videos", headers=headers, json={
    "model": "${videoModelID.value}",
    "prompt": "A quiet cinematic city street after rain",
    "seconds": 5,
    "ratio": "16:9",
    "resolution": "720p"
}).json()

while True:
    status = requests.get(f"{base}/videos/{job['id']}", headers=headers).json()
    if status["status"] in ("completed", "failed"):
        break
    time.sleep(5)

if status["status"] == "completed":
    content = requests.get(f"{base}/videos/{job['id']}/content", headers=headers)
    open("output.mp4", "wb").write(content.content)`,
  node: `import { writeFile } from "node:fs/promises";

const base = "${baseUrl.value}";
const headers = {
  Authorization: "Bearer sk-your-key",
  "Content-Type": "application/json"
};

const job = await fetch(base + "/videos", {
  method: "POST",
  headers,
  body: JSON.stringify({
    model: "${videoModelID.value}",
    prompt: "A quiet cinematic city street after rain",
    seconds: 5,
    ratio: "16:9",
    resolution: "720p"
  })
}).then(r => r.json());

let status;
do {
  await new Promise(resolve => setTimeout(resolve, 5000));
  status = await fetch(base + "/videos/" + job.id, { headers }).then(r => r.json());
} while (!["completed", "failed"].includes(status.status));

if (status.status === "failed") {
  throw new Error(status.error || "Video generation failed");
}

const content = await fetch(base + "/videos/" + job.id + "/content", {
  headers
});
if (!content.ok) throw new Error("Video download failed");
await writeFile("output.mp4", Buffer.from(await content.arrayBuffer()));
console.log("Saved output.mp4");`,
}))
const videoCode = computed(() => videoExamples.value[videoTab.value])

onMounted(loadModels)
</script>

<template>
  <div class="docs-layout">
    <aside>
      <strong>API 文档</strong>
      <a-anchor>
        <a-anchor-link href="#quick" title="快速开始" />
        <a-anchor-link href="#auth" title="身份认证" />
        <a-anchor-link href="#models" title="可用模型" />
        <a-anchor-link href="#images" title="图像生成" />
        <a-anchor-link href="#edits" title="图像编辑" />
        <a-anchor-link href="#videos" title="视频生成" />
        <a-anchor-link href="#responses" title="响应结构" />
        <a-anchor-link href="#errors" title="错误与重试" />
      </a-anchor>
    </aside>

    <article>
      <section id="quick" class="intro-section">
        <span class="kicker">OPENAI COMPATIBLE</span>
        <h2>使用标准接口接入生成能力</h2>
        <p>{{ site.title }} 提供 OpenAI 兼容的图像与视频接口。替换 Base URL 和 API Key 后，即可使用常见 SDK 或标准 HTTP 客户端。</p>
        <div class="endpoint"><span>Base URL</span><code>{{ baseUrl }}</code><a-button class="frontend-icon-button" size="small" title="复制 Base URL" @click="copy(baseUrl)"><IconCopy /></a-button></div>
        <div class="quick-facts"><span><IconCheckCircle />Bearer API Key</span><span><IconCheckCircle />HTTPS JSON API</span><span><IconCheckCircle />失败任务自动退款</span></div>
      </section>

      <section id="auth">
        <h3>身份认证</h3>
        <p>在 API Keys 页面创建以 <code>sk-</code> 开头的密钥。所有接口通过 Authorization 请求头认证，请勿将密钥暴露在浏览器、公开仓库或客户端安装包中。</p>
        <pre class="light-code"><code>Authorization: Bearer sk-your-key</code></pre>
      </section>

      <section id="models">
        <div class="doc-head"><div><h3>当前可用模型</h3><p>此目录与后台模型启用状态实时同步。后台停用模型后，模型将从这里和生成接口中同时移除。</p></div><a-button :loading="loadingModels" @click="loadModels"><IconRefresh />刷新目录</a-button></div>
        <div class="model-group"><div class="group-title"><IconImage /><strong>图像模型</strong><span>{{ imageModels.length }} 个</span></div><div v-if="imageModels.length" class="model-list"><div v-for="model in imageModels" :key="model.id" class="model-row"><div class="model-main"><strong>{{ displayName(model) }}</strong><code>{{ publicModelID(model) }}</code></div><div class="model-caps"><span v-for="item in capabilitySummary(model)" :key="item">{{ item }}</span></div><small>{{ priceSummary(model) }}</small></div></div><a-empty v-else description="暂未启用图像模型" /></div>
        <div class="model-group"><div class="group-title video"><IconVideoCamera /><strong>视频模型</strong><span>{{ videoModels.length }} 个</span></div><div v-if="videoModels.length" class="model-list"><div v-for="model in videoModels" :key="model.id" class="model-row"><div class="model-main"><strong>{{ displayName(model) }}</strong><code>{{ publicModelID(model) }}</code></div><div class="model-caps"><span v-for="item in capabilitySummary(model)" :key="item">{{ item }}</span></div><small>{{ priceSummary(model) }}</small></div></div><a-empty v-else description="暂未启用视频模型" /></div>
        <p class="api-note"><IconCode /><code>GET /models</code> 也会返回当前 API Key 可调用的模型目录，建议客户端启动时动态读取。</p>
      </section>

      <section id="images">
        <div class="doc-head"><div><h3>生成图像</h3><p>创建文生图任务并返回 OpenAI 图像响应。</p></div><a-tag>POST /images/generations</a-tag></div>
        <div class="param-table"><div><b>model</b><span>string · 必填</span><p>使用上方动态目录中的图像模型 ID。</p></div><div><b>prompt</b><span>string · 必填</span><p>图像描述，建议明确主体、场景、光线与风格。</p></div><div><b>size</b><span>string · 可选</span><p>例如 1024x1024、1536x1024，实际能力以模型目录为准。</p></div><div><b>n</b><span>integer · 可选</span><p>生成数量，默认为 1，受账户并发限制。</p></div><div><b>quality</b><span>string · 兼容</span><p>兼容 OpenAI SDK 字段；实际清晰度档位由 size 与模型目录决定。</p></div><div><b>response_format</b><span>string · 兼容</span><p>当前响应统一返回 <code>b64_json</code>，便于直接使用 OpenAI SDK 解析。</p></div><div><b>user</b><span>string · 可选</span><p>可传入调用方用户标识，不参与平台身份认证。</p></div><div><b>Authorization</b><span>header · 必填</span><p>使用 <code>Bearer sk-your-key</code>，不要传登录会话令牌。</p></div></div>
        <a-radio-group v-model="imageTab" type="button" size="small"><a-radio value="curl">cURL</a-radio><a-radio value="python">Python</a-radio><a-radio value="node">Node.js</a-radio></a-radio-group>
        <div class="code-block"><button title="复制图像生成示例" @click="copy(imageCode)"><IconCopy /></button><pre><code>{{ imageCode }}</code></pre></div>
      </section>

      <section id="edits">
        <div class="doc-head"><div><h3>使用参考图编辑</h3><p>支持参考图的模型可通过 multipart/form-data 调用图像编辑接口。</p></div><a-tag>POST /images/edits</a-tag></div>
        <div class="code-block"><button title="复制图像编辑示例" @click="copy(editCode)"><IconCopy /></button><pre><code>{{ editCode }}</code></pre></div>
        <p class="field-note">可重复提交 <code>image</code> 或 <code>image[]</code>。单次参考图数量与文件大小受所选模型能力限制。</p>
      </section>

      <section id="videos">
        <div class="doc-head"><div><h3>生成视频</h3><p>视频采用异步任务：先创建，再轮询状态，完成后下载内容。</p></div><a-tag>POST /videos</a-tag></div>
        <div class="video-flow"><div><b>01</b><span><strong>创建任务</strong><small>返回任务 ID 和 queued 状态</small></span></div><div><b>02</b><span><strong>查询状态</strong><small>每 5 秒请求一次任务详情</small></span></div><div><b>03</b><span><strong>获取内容</strong><small>completed 后下载 MP4</small></span></div></div>
        <div class="param-table"><div><b>model</b><span>string · 必填</span><p>使用动态目录中的视频模型 ID。</p></div><div><b>prompt</b><span>string · 必填</span><p>描述镜头、主体动作、场景、光线和运镜。</p></div><div><b>seconds / duration</b><span>number · 必填</span><p>视频时长，必须属于所选模型公布的时长范围。</p></div><div><b>ratio</b><span>string · 可选</span><p>例如 16:9、9:16、1:1，以模型目录为准。</p></div><div><b>resolution</b><span>string · 可选</span><p>例如 480p、720p、1080p，以模型目录为准。</p></div><div><b>concurrency</b><span>integer · 可选</span><p>并发份数，不能超过账户和模型允许的范围。</p></div><div><b>images / videos / audios</b><span>array · 可选</span><p>JSON 请求可提交素材 URL；数量与大小由模型能力限制。</p></div><div><b>input_reference</b><span>file · 可选</span><p>multipart 请求中的参考图文件字段，可重复提交。</p></div></div>
        <a-radio-group v-model="videoTab" type="button" size="small"><a-radio value="curl">cURL</a-radio><a-radio value="python">Python</a-radio><a-radio value="node">Node.js</a-radio></a-radio-group>
        <div class="code-block"><button title="复制视频生成示例" @click="copy(videoCode)"><IconCopy /></button><pre><code>{{ videoCode }}</code></pre></div>
        <p class="field-note">参考图调用请使用 multipart/form-data，并以 <code>input_reference</code> 上传文件；比例、时长、分辨率不得超出所选模型在目录中公布的能力。</p>
      </section>

      <section id="responses">
        <h3>响应结构</h3>
        <p>图像接口返回 OpenAI 格式的 Base64 数据；视频创建接口返回任务对象。</p>
        <div class="response-grid"><div><span>图像响应</span><pre><code>{
  "created": 1786800000,
  "data": [{ "b64_json": "iVBORw0..." }]
}</code></pre></div><div><span>视频任务</span><pre><code>{
  "id": "evt-XXXXXXXX",
  "status": "queued",
  "model": "model-id"
}</code></pre></div></div>
      </section>

      <section id="errors">
        <h3>错误处理与计费</h3>
        <p>提交生成任务时会预留额度，只有生成和媒体处理成功后才确认扣费；失败、超时或取消会自动退回。POST 请求在未收到明确响应时不要盲目自动重试，建议先查询生成记录，避免创建重复任务。</p>
        <div class="error-table"><div><code>400</code><span>参数错误、提示词不合规或能力超出模型限制</span></div><div><code>401</code><span>API Key 无效，或上游额度暂不可用</span></div><div><code>402</code><span>账户可用额度不足</span></div><div><code>404</code><span>模型、任务或内容不存在</span></div><div><code>409</code><span>视频仍在生成，内容暂不可下载</span></div><div><code>413</code><span>参考文件超过允许大小</span></div><div><code>429</code><span>账户并发任务达到上限</span></div><div><code>502 / 503</code><span>生成服务暂时不可用，稍后再试</span></div></div>
      </section>
    </article>
  </div>
</template>

<style scoped>
.docs-layout{display:grid;grid-template-columns:210px minmax(0,940px);gap:44px}.docs-layout>aside{position:sticky;top:94px;height:max-content;padding-right:18px;border-right:1px solid var(--ns-line)}.docs-layout>aside>strong{display:block;margin-bottom:18px;font-size:12px}article{min-width:0}article section{padding:6px 0 38px;margin-bottom:32px;border-bottom:1px solid var(--ns-line)}article h2{margin:10px 0 12px;font-size:30px;letter-spacing:0}article h3{margin:0 0 10px;font-size:18px;letter-spacing:0}article p,article li{color:var(--ns-ink-soft);font-size:12px;line-height:1.75}.kicker{color:var(--ns-accent-strong);font-size:10px;font-weight:700}.endpoint{display:flex;align-items:center;gap:14px;padding:13px 14px;border:1px solid var(--ns-line);border-radius:6px;background:#fff}.endpoint span{color:var(--ns-ink-faint);font-size:10px}.endpoint code{min-width:0;flex:1;overflow:hidden;text-overflow:ellipsis}.quick-facts{display:flex;gap:20px;margin-top:13px;color:#5c6c57;font-size:10px}.quick-facts span{display:flex;align-items:center;gap:6px}.doc-head{display:flex;align-items:flex-start;justify-content:space-between;gap:24px}.doc-head>div{min-width:0}.doc-head p{margin:0}.model-group{padding:16px 0;border-top:1px solid var(--ns-line)}.group-title{display:flex;align-items:center;gap:8px;margin-bottom:10px;color:#53674e}.group-title.video{color:#6d604e}.group-title span{margin-left:auto;color:var(--ns-ink-faint);font-size:9px}.model-list{display:flex;flex-direction:column}.model-row{padding:13px 14px;display:grid;grid-template-columns:minmax(180px,.8fr) minmax(240px,1.2fr);gap:8px 18px;border-left:3px solid #a9b6a4;background:#f3f5f0}.model-row:nth-child(even){background:#ecefe9}.model-main{min-width:0;display:flex;flex-direction:column;gap:5px}.model-main strong{font-size:11px}.model-main code{overflow:hidden;text-overflow:ellipsis;color:#526050;font-size:9px}.model-caps{display:flex;align-items:center;flex-wrap:wrap;gap:5px}.model-caps span{padding:3px 6px;border-radius:4px;background:rgba(255,255,255,.7);color:var(--ns-ink-soft);font-size:8px}.model-row>small{grid-column:2;color:#7b816f;font-size:8px}.api-note{display:flex;align-items:center;gap:8px;margin:13px 0 0!important}.param-table{display:grid;grid-template-columns:1fr 1fr;margin:16px 0;border-block:1px solid var(--ns-line)}.param-table>div{padding:12px 14px}.param-table>div:nth-child(odd){border-right:1px solid var(--ns-line)}.param-table>div:nth-child(n+3){border-top:1px solid var(--ns-line)}.param-table b{font:600 10px ui-monospace}.param-table span{margin-left:8px;color:#7b806f;font-size:8px}.param-table p{margin:5px 0 0;font-size:9px}.code-block{position:relative;margin-top:12px}.code-block>button{position:absolute;z-index:2;right:10px;top:10px;padding:6px;border:0;border-radius:4px;background:#3d463f;color:#fff;cursor:pointer}pre{margin:0;padding:18px;border-radius:6px;background:#202521;color:#e8ebe4;font-size:11px;line-height:1.65;overflow:auto}.light-code{background:#edefe9;color:var(--ns-ink)}.field-note{margin-bottom:0}.video-flow{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));margin:16px 0;border-block:1px solid var(--ns-line)}.video-flow>div{padding:15px;display:flex;gap:9px}.video-flow>div+div{border-left:1px solid var(--ns-line)}.video-flow b{color:#8a917f;font:600 9px ui-monospace}.video-flow span{display:flex;flex-direction:column}.video-flow strong{font-size:10px}.video-flow small{margin-top:4px;color:var(--ns-ink-faint);font-size:8px}.response-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px}.response-grid>div>span{display:block;margin-bottom:7px;color:var(--ns-ink-faint);font-size:9px}.response-grid pre{height:128px}.error-table{border-block:1px solid var(--ns-line)}.error-table>div{padding:10px 12px;display:grid;grid-template-columns:76px 1fr;gap:14px}.error-table>div+div{border-top:1px solid var(--ns-line)}.error-table code{color:#53654f;font-weight:700}.error-table span{color:var(--ns-ink-soft);font-size:10px}@media(max-width:900px){.docs-layout{grid-template-columns:170px minmax(0,1fr);gap:28px}.model-row{grid-template-columns:1fr}.model-row>small{grid-column:1}}@media(max-width:760px){.docs-layout{grid-template-columns:1fr}.docs-layout>aside{display:none}.quick-facts{align-items:flex-start;flex-direction:column;gap:7px}.doc-head{align-items:flex-start}.model-row,.param-table,.response-grid,.video-flow{grid-template-columns:1fr}.param-table>div:nth-child(odd){border-right:0}.param-table>div+div,.video-flow>div+div{border-top:1px solid var(--ns-line);border-left:0}.response-grid pre{height:auto}}
</style>

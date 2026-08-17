<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconEdit, IconThunderbolt, IconVideoCamera } from '@arco-design/web-vue/es/icon'
import { api } from '../../services/api'
const loading=ref(false),syncing=ref(false),saving=ref(false),open=ref(false),rows=ref<any[]>([]),editing=ref<any>()
const form=reactive({name:'',alias:'',enabled:false,weight:0,per_second:0,prices:{} as Record<string,number>,agent_per_second:0,agent_prices:{} as Record<string,number>})
const resolutions=computed(()=>editing.value?.resolutions||[])
async function load(){loading.value=true;const r=await api('/sanbao/models');loading.value=false;if(r.ok)rows.value=r.data.data||[];else Message.error(r.data?.detail||'加载失败')}
async function sync(){syncing.value=true;const r=await api('/sanbao/models/sync',{method:'POST'});syncing.value=false;if(r.ok){Message.success(`已同步 ${r.data.synced} 个视频模型`);load()}else Message.error(r.data?.detail||'同步失败')}
function edit(row:any){editing.value=row;form.name=row.name;form.alias=row.alias||'';form.enabled=!!row.enabled;form.weight=Number(row.weight||0);form.per_second=Number(row.duration_prices?.per_second||0);form.agent_per_second=Number(row.duration_prices_agent?.per_second||0);form.prices=Object.fromEntries((row.resolutions||[]).map((x:string)=>[x,Number(row.prices?.[x]||0)]));form.agent_prices=Object.fromEntries((row.resolutions||[]).map((x:string)=>[x,Number(row.prices_agent?.[x]||0)]));open.value=true}
async function save(){saving.value=true;const r=await api(`/sanbao/models/${encodeURIComponent(editing.value.id)}`,{method:'PATCH',body:JSON.stringify({name:form.name,alias:form.alias,enabled:form.enabled,weight:Number(form.weight),prices:form.prices,duration_prices:{per_second:Number(form.per_second)},prices_agent:form.agent_prices,duration_prices_agent:{per_second:Number(form.agent_per_second)}})});saving.value=false;if(r.ok){Message.success('视频模型配置已保存');open.value=false;load()}else Message.error(r.data?.detail||'保存失败')}
async function toggle(row:any,value:boolean){const r=await api(`/sanbao/models/${encodeURIComponent(row.id)}`,{method:'PATCH',body:JSON.stringify({enabled:value})});if(!r.ok)Message.error(r.data?.detail||'更新失败');load()}
function caps(row:any){const c=row.capabilities||{};return `${(row.ratios||[]).length} 比例 · ${(row.resolutions||[]).length} 清晰度 · ${(row.durations||[]).length} 时长`}
function priceEntries(value:any){return Object.entries(value||{}).map(([resolution,price])=>({resolution,price:Number(price)})).filter(item=>Number.isFinite(item.price)&&item.price>0)}
function upstreamCosts(row:any){
  const capabilities=row?.capabilities||{}
  const fixed=new Map(priceEntries(capabilities.price_by_resolution_credits).map(item=>[item.resolution,item.price]))
  const perSecond=new Map(priceEntries(capabilities.price_per_second_by_resolution_credits).map(item=>[item.resolution,item.price]))
  const resolutions=Array.from(new Set([...(row?.resolutions||[]),...fixed.keys(),...perSecond.keys()]))
  return resolutions.flatMap((resolution:string)=>{
    const items=[] as {resolution:string;price:number;unit:string}[]
    if(fixed.has(resolution))items.push({resolution,price:fixed.get(resolution)!,unit:'次'})
    if(perSecond.has(resolution))items.push({resolution,price:perSecond.get(resolution)!,unit:'秒'})
    return items
  })
}
function upstreamSummary(row:any){const items=upstreamCosts(row);return items.length?items.map(item=>`${item.resolution} ${item.price}/${item.unit}`).join(' · '):'上游未返回'}
function saleSummary(row:any){
  const bases=priceEntries(row.prices).map(item=>`${item.resolution} ${item.price}`)
  const perSecond=Number(row.duration_prices?.per_second||0)
  if(perSecond>0)bases.push(`${perSecond}/秒`)
  return bases.length?bases.join(' · '):'未定价'
}
onMounted(load)
</script>
<template><div class="admin-page"><div class="section-heading"><div><h2>视频模型</h2><p>同步三宝模型能力，配置前台名称、价格、排序与发布状态。</p></div><a-button type="primary" :loading="syncing" @click="sync"><IconThunderbolt/>同步三宝模型</a-button></div>
<div class="notice"><IconVideoCamera/><div><strong>能力与售价分离</strong><p>比例、时长和素材上限来自三宝；前台售价由本系统独立管理。新同步模型默认不发布。</p></div></div>
<div class="model-table"><a-table :data="rows" :loading="loading" row-key="id" :pagination="{pageSize:15}" :scroll="{x:1050}"><template #columns>
<a-table-column title="模型" :width="250"><template #cell="{record}"><div class="model-id"><span><IconVideoCamera/></span><div><strong>{{ record.alias||record.name }}</strong><small>{{ record.upstream_model }}</small></div></div></template></a-table-column>
<a-table-column title="动态能力" :width="230"><template #cell="{record}"><div class="cap"><strong>{{ caps(record) }}</strong><small>最多 {{ record.capabilities?.max_images||0 }} 图 · {{ record.capabilities?.max_videos||0 }} 视频 · {{ record.capabilities?.max_audios||0 }} 音频</small></div></template></a-table-column>
<a-table-column title="售价 / 参考成本" :width="270"><template #cell="{record}"><div class="price"><strong>售价 {{ saleSummary(record) }}</strong><small :title="upstreamSummary(record)">成本 {{ upstreamSummary(record) }}</small></div></template></a-table-column>
<a-table-column title="生成次数" data-index="generation_count" :width="100"/>
<a-table-column title="前台发布" :width="110"><template #cell="{record}"><a-switch :model-value="record.enabled" @change="toggle(record,Boolean($event))"/></template></a-table-column>
<a-table-column title="操作" :width="90" fixed="right"><template #cell="{record}"><a-tooltip content="配置模型"><button class="edit" @click="edit(record)"><IconEdit/></button></a-tooltip></template></a-table-column>
</template><template #empty><a-empty description="先导入三宝账号，再同步视频模型"/></template></a-table></div>
<a-modal v-model:visible="open" title="配置视频模型" :ok-loading="saving" ok-text="保存配置" width="660px" @ok="save"><div class="edit-form"><div class="two"><div><label>前台名称</label><a-input v-model="form.name"/></div><div><label>OpenAI 模型别名</label><a-input v-model="form.alias" placeholder="留空使用系统 ID"/></div></div><div class="two compact"><div><label>排序权重</label><a-input-number v-model="form.weight" :min="0" :max="999"/></div><div><label>前台发布</label><a-switch v-model="form.enabled"/></div></div><div v-if="upstreamCosts(editing).length" class="upstream-cost"><div class="price-head"><strong>上游参考成本</strong><span>通过当前 Key 实时同步，仅供运营定价参考</span></div><div class="cost-list"><span v-for="item in upstreamCosts(editing)" :key="`${item.resolution}-${item.unit}`"><b>{{ item.resolution }}</b>{{ item.price }} 额度 / {{ item.unit }}</span></div></div><div class="price-section"><div class="price-head"><strong>普通用户售价</strong><span>最终价格 = 清晰度基础价 + 每秒单价 × 时长</span></div><div class="price-grid"><div v-for="r in resolutions" :key="r"><label>{{ r }} 基础价</label><a-input-number v-model="form.prices[r]" :min="0" :precision="4"/></div><div><label>每秒单价</label><a-input-number v-model="form.per_second" :min="0" :precision="4"/></div></div></div><div class="price-section agent"><div class="price-head"><strong>代理用户售价</strong><span>未配置时建议与普通售价保持一致</span></div><div class="price-grid"><div v-for="r in resolutions" :key="r"><label>{{ r }} 基础价</label><a-input-number v-model="form.agent_prices[r]" :min="0" :precision="4"/></div><div><label>每秒单价</label><a-input-number v-model="form.agent_per_second" :min="0" :precision="4"/></div></div></div></div></a-modal>
</div></template>
<style scoped>
.notice{display:flex;align-items:center;gap:13px;margin-bottom:18px;padding:14px 16px;border:1px solid #d9ded5;border-radius:7px;background:#f5f7f2;color:#566351}.notice>svg{width:22px}.notice div{display:flex;flex-direction:column}.notice strong{font-size:11px}.notice p{margin:4px 0 0;color:var(--ns-ink-soft);font-size:10px}.model-table{overflow:hidden;border:1px solid var(--ns-line);border-radius:7px;background:#fff}.model-id{display:flex;align-items:center;gap:10px}.model-id>span{width:34px;height:34px;display:grid;place-items:center;border-radius:50%;background:#e4e9e0;color:#52604e}.model-id div,.cap{display:flex;min-width:0;flex-direction:column}.model-id strong,.cap strong{font-size:11px}.model-id small,.cap small{margin-top:4px;color:var(--ns-ink-faint);font-size:9px}.price{display:flex;min-width:0;flex-direction:column;gap:4px;color:var(--ns-ink-soft)}.price strong{overflow:hidden;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.price small{overflow:hidden;color:var(--ns-ink-faint);font-size:9px;text-overflow:ellipsis;white-space:nowrap}.edit{width:30px;height:30px;display:grid;place-items:center;border:1px solid var(--ns-line);border-radius:50%;background:#fff;cursor:pointer}.edit-form label{display:block;margin-bottom:7px;font-size:10px;font-weight:650}.two{display:grid;grid-template-columns:1fr 1fr;gap:12px}.two.compact{grid-template-columns:1fr 1fr;align-items:end;margin-top:14px}.two :deep(.arco-input-number){width:100%}.upstream-cost{margin-top:18px;padding:14px;border:1px solid #dce1d8;border-radius:7px;background:#f6f8f4}.upstream-cost .price-head{margin-bottom:10px}.cost-list{display:flex;flex-wrap:wrap;gap:7px}.cost-list span{padding:6px 9px;border-radius:5px;background:#e7ebe4;color:#596454;font-size:9px}.cost-list b{margin-right:6px;color:#2f3931;font-weight:700}.price-section{margin-top:20px;padding-top:17px;border-top:1px solid var(--ns-line)}.price-section.agent{background:#fafaf7;margin-inline:-20px;margin-bottom:-20px;padding:17px 20px 20px}.price-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:12px}.price-head strong{font-size:11px}.price-head span{color:var(--ns-ink-faint);font-size:9px}.price-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}.price-grid :deep(.arco-input-number){width:100%}@media(max-width:600px){.two,.price-grid{grid-template-columns:1fr}.notice p{line-height:1.5}.price-head{align-items:flex-start;gap:8px;flex-direction:column}}
</style>

<script setup lang="ts">
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

type Bucket = { image?: number; video?: number; value?: number; date?: string }
const props = withDefaults(defineProps<{ data: Bucket[]; hours?: number }>(), { hours: 24 })
const el = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null
let observer: ResizeObserver | null = null

const visibleData = computed(() => props.data.slice(-props.hours))
const splitSeries = computed(() => visibleData.value.some((item) => item.image !== undefined || item.video !== undefined))

function labels() {
  const now = new Date()
  return visibleData.value.map((item, index) => {
    if (item.date) return item.date
    const date = new Date(now.getTime() - (visibleData.value.length - index - 1) * 3600000)
    return `${String(date.getHours()).padStart(2, '0')}:00`
  })
}

function render() {
  if (!el.value) return
  chart ||= echarts.init(el.value)
  chart.setOption({
    animationDuration: 420,
    color: ['#202521', '#9b9ea7'],
    grid: { left: 4, right: 8, top: 38, bottom: 8, containLabel: true },
    legend: { right: 2, top: 0, itemWidth: 14, itemHeight: 3, textStyle: { color: '#737972', fontSize: 11 }, data: splitSeries.value ? ['图像任务', '视频任务'] : ['生成任务'] },
    tooltip: { trigger: 'axis', backgroundColor: '#202521', borderWidth: 0, padding: [9, 11], textStyle: { color: '#fff', fontSize: 11 } },
    xAxis: { type: 'category', boundaryGap: false, data: labels(), axisLine: { lineStyle: { color: '#dedfd8' } }, axisTick: { show: false }, axisLabel: { color: '#92978f', fontSize: 10, interval: Math.max(0, Math.floor(visibleData.value.length / 6) - 1) } },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: '#ecece7' } }, axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: '#92978f', fontSize: 10 } },
    series: splitSeries.value ? [
      { name: '图像任务', type: 'line', smooth: 0.22, showSymbol: false, lineStyle: { width: 2.5 }, areaStyle: { color: 'rgba(32,37,33,.13)' }, data: visibleData.value.map((item) => Number(item.image || 0)) },
      { name: '视频任务', type: 'line', smooth: 0.22, showSymbol: false, lineStyle: { width: 2 }, areaStyle: { color: 'rgba(155,158,167,.08)' }, data: visibleData.value.map((item) => Number(item.video || 0)) },
    ] : [{ name: '生成任务', type: 'line', smooth: 0.22, showSymbol: false, lineStyle: { width: 2.5 }, areaStyle: { color: 'rgba(32,37,33,.13)' }, data: visibleData.value.map((item) => Number(item.value || 0)) }],
  }, true)
}

onMounted(() => {
  render()
  observer = new ResizeObserver(() => chart?.resize())
  if (el.value) observer.observe(el.value)
})
watch(() => [props.data, props.hours], render, { deep: true })
onBeforeUnmount(() => { observer?.disconnect(); chart?.dispose() })
</script>

<template><div ref="el" class="trend-chart"></div></template>
<style scoped>.trend-chart{width:100%;height:340px}</style>

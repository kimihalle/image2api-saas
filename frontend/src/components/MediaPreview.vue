<script setup lang="ts">
import { IconDownload, IconClose } from '@arco-design/web-vue/es/icon'

const props = withDefaults(defineProps<{
  visible: boolean
  src?: string
  kind?: 'image' | 'video' | string
  filename?: string
  downloadable?: boolean
}>(), { src: '', kind: 'image', filename: 'generated-work', downloadable: false })

const emit = defineEmits<{ close: [] }>()

function close() { emit('close') }
function download() {
  if (!props.src) return
  const link = document.createElement('a')
  link.href = props.src
  link.download = props.filename || 'generated-work'
  link.click()
}
</script>

<template>
  <a-image-preview
    v-if="kind !== 'video'"
    :visible="visible"
    :src="src"
    :mask-closable="true"
    :esc-to-close="true"
    :keyboard="true"
    :wheel-zoom="true"
    :zoom-rate="1.2"
    :actions-layout="['zoomIn', 'zoomOut', 'originalSize']"
    @close="close"
  />
  <teleport to="body">
    <button v-if="visible && kind !== 'video' && downloadable" class="preview-download" title="下载原图" aria-label="下载原图" @click="download"><IconDownload /></button>
    <div v-if="visible && kind === 'video'" class="video-lightbox" @click.self="close">
      <button class="video-close" aria-label="关闭预览" @click="close"><IconClose /></button>
      <video :src="src" controls autoplay />
      <button v-if="downloadable" class="video-download" title="下载视频" aria-label="下载视频" @click="download"><IconDownload /></button>
    </div>
  </teleport>
</template>

<style>
.preview-download{width:34px;height:34px;padding:0;position:fixed;z-index:1003;top:35px;right:80px;display:grid;place-items:center;border:0;border-radius:50%;background:rgba(0,0,0,.55);color:#fff;cursor:pointer}.preview-download:hover{background:rgba(0,0,0,.78)}.preview-download svg{width:16px;height:16px}.video-lightbox{position:fixed;inset:0;z-index:1001;display:grid;place-items:center;padding:42px;background:rgba(15,17,16,.9)}.video-lightbox video{display:block;max-width:calc(100vw - 84px);max-height:calc(100vh - 84px);object-fit:contain}.video-close,.video-download{width:34px;height:34px;padding:0;position:absolute;top:35px;display:grid;place-items:center;border:0;border-radius:50%;background:rgba(0,0,0,.55);color:#fff;cursor:pointer}.video-close{right:36px}.video-download{right:80px}.video-close:hover,.video-download:hover{background:rgba(0,0,0,.78)}
@media(max-width:600px){.preview-download,.video-download{top:20px;right:66px}.video-close{top:20px;right:20px}.video-lightbox{padding:20px}.video-lightbox video{max-width:calc(100vw - 40px);max-height:calc(100vh - 40px)}}
</style>

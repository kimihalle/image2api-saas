<script setup lang="ts">
import { onMounted } from 'vue'
import LoginModal from './components/LoginModal.vue'
import AnnouncementModal from './components/AnnouncementModal.vue'
import { useSiteStore } from './stores/site'

const site = useSiteStore()
onMounted(() => site.loadSite())
</script>

<template>
  <router-view />
  <LoginModal />
  <AnnouncementModal />
</template>

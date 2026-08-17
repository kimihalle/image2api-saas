import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api, imageUrl } from '../services/api'

export const useSiteStore = defineStore('site', () => {
  const title = ref('Vivid')
  const subtitle = ref('')
  const logo = ref('')
  const contact = ref<Record<string, string>>({})
  const loaded = ref(false)
  const logoUrl = computed(() => imageUrl(logo.value))

  async function loadSite(force = false) {
    if (loaded.value && !force) return
    const response = await api('/site')
    if (response.ok) {
      title.value = response.data?.title || 'Vivid'
      subtitle.value = response.data?.subtitle || ''
      logo.value = response.data?.logo || ''
      contact.value = response.data?.contact || {}
      document.title = `${document.title.split(' · ')[0]} · ${title.value}`
    }
    loaded.value = true
  }

  return { title, subtitle, logo, logoUrl, contact, loaded, loadSite }
})

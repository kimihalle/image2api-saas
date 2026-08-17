import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '../services/api'

const TOKEN_KEY = 'northstar_session'
const API_KEY = 'northstar_api_key'

function storedAPIKey() {
  const value = localStorage.getItem(API_KEY) || ''
  if (!value || value.startsWith('sk-')) return value
  localStorage.removeItem(API_KEY)
  return ''
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) || '')
  const apiKey = ref(storedAPIKey())
  const user = ref<any>(null)
  const ready = ref(false)
  const loginOpen = ref(false)
  const loginIntent = ref('')
  const startMode = ref<'login' | 'register'>('login')
  const isAuthed = computed(() => !!user.value && !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function ensureReady() {
    if (ready.value) return
    if (!token.value) { ready.value = true; return }
    const result = await api('/auth/me')
    if (result.ok) {
      user.value = result.data.user
    } else clear()
    ready.value = true
  }

  async function refreshUser() {
    if (!token.value) return null
    const result = await api('/auth/me')
    if (result.ok) user.value = result.data.user
    else clear()
    return result
  }

  async function login(identifier: string, password: string) {
    const result = await api('/auth/login', { method: 'POST', body: JSON.stringify({ identifier, password }) })
    if (result.ok) setSession(result.data.token, result.data.user)
    return result
  }

  async function register(email: string, username: string, password: string, emailCode = '', inviteCode = '') {
    const result = await api('/auth/register', { method: 'POST', body: JSON.stringify({ email, username, password, email_code: emailCode, invite_code: inviteCode }) })
    if (result.ok) setSession(result.data.token, result.data.user)
    return result
  }

  function setSession(nextToken: string, nextUser: any) {
    token.value = nextToken || ''
    user.value = nextUser || null
    ready.value = true
    if (token.value) localStorage.setItem(TOKEN_KEY, token.value)
  }

  function clear() {
    token.value = ''
    user.value = null
    ready.value = true
    localStorage.removeItem(TOKEN_KEY)
  }

  async function logout() {
    if (token.value) await api('/auth/logout', { method: 'POST' })
    clear()
  }

  function setApiKey(value: unknown) {
    const normalized = typeof value === 'string' && value.startsWith('sk-') ? value : ''
    apiKey.value = normalized
    if (normalized) localStorage.setItem(API_KEY, normalized)
    else localStorage.removeItem(API_KEY)
  }

  function openLogin(intent = '', mode: 'login' | 'register' = 'login') {
    loginIntent.value = intent
    startMode.value = mode
    loginOpen.value = true
  }

  function closeLogin() { loginOpen.value = false }

  return { token, apiKey, user, ready, loginOpen, loginIntent, startMode, isAuthed, isAdmin, ensureReady, refreshUser, login, register, setSession, logout, clear, setApiKey, openLogin, closeLogin }
})

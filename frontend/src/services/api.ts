import { useAuthStore } from '../stores/auth'

const BASE = import.meta.env.VITE_API_BASE || ''

type RequestOptions = RequestInit

export async function api<T = any>(path: string, options: RequestOptions = {}): Promise<{ ok: boolean; status: number; data: T }> {
  const auth = useAuthStore()
  const headers = new Headers(options.headers)
  if (auth.token) headers.set('Authorization', `Bearer ${auth.token}`)
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const response = await fetch(`${BASE}/admin/api${path}`, { ...options, headers })
  let data: any = null
  try { data = await response.json() } catch { /* empty response */ }
  if (response.status === 401) auth.clear()
  return { ok: response.ok, status: response.status, data }
}

export async function openai<T = any>(path: string, options: RequestInit = {}) {
  const auth = useAuthStore()
  const headers = new Headers(options.headers)
  if (auth.apiKey) headers.set('Authorization', `Bearer ${auth.apiKey}`)
  return fetch(`${BASE}/v1${path}`, { ...options, headers })
}

export function imageUrl(path?: string | null) {
  if (!path) return ''
  return path.startsWith('http') ? path : `${BASE}${path}`
}

export function apiOrigin() {
  return `${window.location.origin}/v1`
}

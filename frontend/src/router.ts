import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useSiteStore } from './stores/site'

const userChildren = [
  { path: '', component: () => import('./views/PublicHomeView.vue'), meta: { public: true, title: '首页' } },
  { path: 'inspiration', component: () => import('./views/user/InspirationView.vue'), meta: { public: true, title: '灵感广场' } },
  { path: 'generate', component: () => import('./views/user/GenerateView.vue'), meta: { title: '图片生成' } },
  { path: 'video', component: () => import('./views/user/VideoStudioView.vue'), meta: { title: '视频创作' } },
  { path: 'history', component: () => import('./views/user/HistoryView.vue'), meta: { title: '生成记录' } },
  { path: 'api-keys', component: () => import('./views/user/APIKeysView.vue'), meta: { title: 'API Keys' } },
  { path: 'billing', component: () => import('./views/user/BillingView.vue'), meta: { title: '账单与额度' } },
  { path: 'docs', component: () => import('./views/user/DocsView.vue'), meta: { title: '开发文档' } },
  { path: 'settings', component: () => import('./views/user/SettingsView.vue'), meta: { title: '账户设置' } },
  { path: 'rewards', component: () => import('./views/user/RewardsView.vue'), meta: { title: '签到与邀请' } },
]

const adminChildren = [
  { path: '', redirect: '/admin/overview' },
  { path: 'overview', component: () => import('./views/admin/AdminOverviewView.vue'), meta: { title: '运营概览', admin: true } },
  { path: 'operations', component: () => import('./views/admin/OperationsView.vue'), meta: { title: '系统保障', admin: true } },
  { path: 'models', component: () => import('./views/admin/ModelsView.vue'), meta: { title: '模型目录', admin: true } },
  { path: 'providers', component: () => import('./views/admin/ProvidersView.vue'), meta: { title: 'Provider 账号', admin: true } },
  { path: 'video-models', component: () => import('./views/admin/VideoModelsView.vue'), meta: { title: '视频模型', admin: true } },
  { path: 'sanbao-accounts', component: () => import('./views/admin/SanbaoAccountsView.vue'), meta: { title: '三宝账号', admin: true } },
  { path: 'users', component: () => import('./views/admin/UsersView.vue'), meta: { title: '用户与权限', admin: true } },
  { path: 'billing', component: () => import('./views/admin/AdminBillingView.vue'), meta: { title: '订单与账本', admin: true } },
  { path: 'packages', component: () => import('./views/admin/RechargePackagesView.vue'), meta: { title: '充值套餐', admin: true } },
  { path: 'cdks', component: () => import('./views/admin/AdminCDKView.vue'), meta: { title: '兑换码管理', admin: true } },
  { path: 'logs', component: () => import('./views/admin/LogsView.vue'), meta: { title: '生成日志', admin: true } },
  { path: 'works', component: () => import('./views/admin/WorksView.vue'), meta: { title: '作品管理', admin: true } },
  { path: 'invites', component: () => import('./views/admin/InvitesView.vue'), meta: { title: '邀请记录', admin: true } },
  { path: 'showcase', component: () => import('./views/admin/ShowcaseAdminView.vue'), meta: { title: '首页内容', admin: true } },
  { path: 'prompts', component: () => import('./views/admin/PromptTemplatesView.vue'), meta: { title: '灵感模板', admin: true } },
  { path: 'banned-words', component: () => import('./views/admin/BannedWordsView.vue'), meta: { title: '违禁词管理', admin: true } },
  { path: 'settings', component: () => import('./views/admin/AdminSettingsView.vue'), meta: { title: '系统设置', admin: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./layouts/WorkspaceLayout.vue'), children: userChildren },
    {
      path: '/app',
      component: () => import('./layouts/WorkspaceLayout.vue'),
      children: [
        { path: '', redirect: '/app/generate' },
        { path: 'overview', redirect: '/app/generate' },
        ...userChildren.slice(1),
      ],
    },
    { path: '/admin', component: () => import('./layouts/WorkspaceLayout.vue'), meta: { admin: true }, children: adminChildren },
    { path: '/login', redirect: '/' },
  ],
})

const chunkReloadKey = 'northstar_chunk_reload'

router.onError((error, to) => {
  const message = String(error?.message || error)
  const isChunkFailure = /dynamically imported module|module script failed|unable to preload css/i.test(message)
  if (!isChunkFailure) return

  const target = to.fullPath || window.location.pathname + window.location.search
  if (sessionStorage.getItem(chunkReloadKey) === target) {
    sessionStorage.removeItem(chunkReloadKey)
    return
  }
  sessionStorage.setItem(chunkReloadKey, target)
  window.location.replace(target)
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.ensureReady()
  if (to.path === '/' || to.meta.public) return true
  if (!auth.isAuthed) {
    auth.openLogin(to.fullPath)
    return '/'
  }
  if (to.meta.admin && !auth.isAdmin) return '/'
  return true
})

router.afterEach((to) => {
  sessionStorage.removeItem(chunkReloadKey)
  const site = useSiteStore()
  document.title = `${String(to.meta.title || '首页')} · ${site.title}`
})

export default router

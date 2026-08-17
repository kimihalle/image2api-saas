<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  IconApps, IconArchive, IconBook, IconCloud, IconCode, IconDashboard,
  IconCalendar, IconFile, IconGift, IconImage, IconLayers, IconMenu, IconNotification, IconPoweroff, IconQuestionCircle, IconSafe,
  IconSettings, IconStorage, IconStop, IconThunderbolt, IconUserGroup, IconVideoCamera, IconBulb,
} from '@arco-design/web-vue/es/icon'
import BrandMark from '../components/BrandMark.vue'
import PageErrorBoundary from '../components/PageErrorBoundary.vue'
import { useAuthStore } from '../stores/auth'
import { useSiteStore } from '../stores/site'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const site = useSiteStore()
const mobileOpen = ref(false)
const sidebarCollapsed = ref(localStorage.getItem('workspace_sidebar_collapsed') === 'true')
const adminMode = computed(() => route.path.startsWith('/admin'))
const title = computed(() => String(route.meta.title || '首页'))

const userGroups = [
  { label: '创作空间', items: [
    { path: '/', label: '首页', icon: IconDashboard },
    { path: '/inspiration', label: '灵感广场', icon: IconBulb },
    { path: '/app/generate', label: '图片生成', icon: IconImage },
    { path: '/app/video', label: '视频创作', icon: IconVideoCamera },
    { path: '/app/history', label: '生成记录', icon: IconArchive },
  ] },
  { label: '开发与用量', items: [
    { path: '/app/api-keys', label: 'API Keys', icon: IconCode },
    { path: '/app/docs', label: '开发文档', icon: IconBook },
    { path: '/app/billing', label: '账单与额度', icon: IconSafe },
  ] },
  { label: '账户', items: [
    { path: '/app/rewards', label: '签到与邀请', icon: IconCalendar },
    { path: '/app/settings', label: '账户设置', icon: IconSettings },
  ] },
]

const adminGroups = [
  { label: '运营', items: [
    { path: '/admin/overview', label: '运营概览', icon: IconDashboard },
    { path: '/admin/logs', label: '生成日志', icon: IconFile },
    { path: '/admin/works', label: '作品管理', icon: IconImage },
    { path: '/admin/users', label: '用户与权限', icon: IconUserGroup },
    { path: '/admin/invites', label: '邀请记录', icon: IconUserGroup },
    { path: '/admin/billing', label: '订单与账本', icon: IconGift },
    { path: '/admin/packages', label: '充值套餐', icon: IconLayers },
    { path: '/admin/cdks', label: '兑换码管理', icon: IconThunderbolt },
    { path: '/admin/showcase', label: '首页内容', icon: IconApps },
    { path: '/admin/prompts', label: '灵感模板', icon: IconBulb },
    { path: '/admin/banned-words', label: '违禁词管理', icon: IconStop },
  ] },
  { label: '资源', items: [
    { path: '/admin/models', label: '模型目录', icon: IconCloud },
    { path: '/admin/video-models', label: '视频模型', icon: IconVideoCamera },
    { path: '/admin/sanbao-accounts', label: '三宝账号', icon: IconStorage },
    { path: '/admin/providers', label: 'Provider 账号', icon: IconStorage },
  ] },
  { label: '系统', items: [
    { path: '/admin/settings', label: '系统设置', icon: IconSafe },
    { path: '/', label: '返回创作空间', icon: IconApps },
  ] },
]

const groups = computed(() => adminMode.value ? adminGroups : userGroups)
const activePath = computed(() => route.path === '/' ? '/' : route.path)

function navigate(path: string) {
  mobileOpen.value = false
  if (path === '/' || adminMode.value || auth.isAuthed) router.push(path)
  else auth.openLogin(path)
}

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  localStorage.setItem('workspace_sidebar_collapsed', String(sidebarCollapsed.value))
}

function openAnnouncement() {
  window.dispatchEvent(new Event('open-announcement'))
}

function primaryAction() { auth.openLogin('') }

async function logout() {
  await auth.logout()
  router.push('/')
}
</script>

<template>
  <div class="shell" :class="{ 'admin-shell': adminMode, collapsed: sidebarCollapsed }">
    <aside class="sidebar">
      <div class="brand-row">
        <BrandMark :compact="sidebarCollapsed" />
        <span v-if="!sidebarCollapsed" class="product-chip">{{ adminMode ? 'OPS' : 'STUDIO' }}</span>
        <a-tooltip :content="sidebarCollapsed ? '展开侧栏' : '收起侧栏'"><button class="collapse-control" :aria-label="sidebarCollapsed ? '展开侧栏' : '收起侧栏'" @click="toggleSidebar"><IconMenu /></button></a-tooltip>
      </div>
      <nav class="nav-groups">
        <div v-for="group in groups" :key="group.label" class="nav-group">
          <span class="nav-label">{{ group.label }}</span>
          <div class="nav-list">
            <button v-for="item in group.items" :key="item.path" :class="{ active: activePath === item.path }" @click="navigate(item.path)">
              <span class="nav-icon"><component :is="item.icon" /></span><span>{{ item.label }}</span>
            </button>
          </div>
        </div>
      </nav>
      <div class="side-bottom">
        <button v-if="!auth.isAuthed && !adminMode" class="guest-profile" @click="auth.openLogin('')">
          <span class="guest-avatar">{{ site.title.slice(0, 1).toUpperCase() }}</span><span><strong>登录工作台</strong><small>登录或注册后开始创作</small></span>
        </button>
        <div v-else class="profile">
          <a-avatar :size="36" :style="{ background: '#dfe7da', color: '#3f4f3a' }">{{ auth.user?.name?.slice(0, 1) || 'N' }}</a-avatar>
          <div><strong>{{ auth.user?.name || 'Admin' }}</strong><small>{{ auth.user?.email || '运营空间' }}</small></div>
          <a-tooltip content="退出登录"><button class="icon-action" aria-label="退出登录" @click="logout"><IconPoweroff /></button></a-tooltip>
        </div>
      </div>
    </aside>

    <main class="main">
      <header class="topbar">
        <div class="topbar-title"><a-button class="mobile-menu" type="text" aria-label="打开菜单" @click="mobileOpen = true"><IconMenu /></a-button><div><span class="crumb">{{ adminMode ? 'ADMIN CONSOLE' : site.title.toUpperCase() }}</span><h1>{{ title }}</h1></div></div>
        <div v-if="!adminMode" class="top-actions">
          <a-tooltip v-if="auth.isAuthed" content="平台公告"><button class="top-icon" aria-label="平台公告" @click="openAnnouncement"><IconNotification /></button></a-tooltip>
          <a-tooltip v-if="!adminMode" content="开发文档"><button class="top-icon" aria-label="开发文档" @click="navigate('/app/docs')"><IconQuestionCircle /></button></a-tooltip>
          <span class="credit-pill"><IconThunderbolt /><span>可用额度</span><strong v-if="auth.isAuthed">{{ Number(auth.user?.credits || 0).toLocaleString() }}</strong><strong v-else>登录后查看</strong></span>
          <a-button v-if="!auth.isAuthed" type="primary" shape="round" @click="primaryAction">登录</a-button>
        </div>
      </header>
      <section class="content"><PageErrorBoundary><router-view :key="route.fullPath" /></PageErrorBoundary></section>
    </main>

    <a-drawer v-model:visible="mobileOpen" :footer="false" :width="296" placement="left" unmount-on-close>
      <template #title><BrandMark /></template>
      <nav class="nav-groups mobile"><div v-for="group in groups" :key="group.label" class="nav-group"><span class="nav-label">{{ group.label }}</span><div class="nav-list"><button v-for="item in group.items" :key="item.path" :class="{ active: activePath === item.path }" @click="navigate(item.path)"><span class="nav-icon"><component :is="item.icon" /></span><span>{{ item.label }}</span></button></div></div></nav>
    </a-drawer>
  </div>
</template>

<style scoped>
.nav-groups{scrollbar-width:none;-ms-overflow-style:none}.nav-groups::-webkit-scrollbar{display:none}
.shell{min-height:100vh;background:var(--ns-bg)}.sidebar{position:fixed;inset:0 auto 0 0;width:268px;display:flex;flex-direction:column;padding:0 18px;background:#fff;border-right:1px solid var(--ns-line);z-index:20;transition:width .2s ease,padding .2s ease}.brand-row{height:76px;display:flex;align-items:center;gap:8px;padding:0 8px}.brand-row .brand{margin-right:auto}.product-chip{padding:4px 8px;border:1px solid var(--ns-line);border-radius:999px;color:var(--ns-ink-faint);font-size:9px;font-weight:700}.collapse-control{width:30px;height:30px;flex:0 0 30px;display:grid;place-items:center;border:0;border-radius:50%;background:transparent;color:var(--ns-ink-soft);cursor:pointer}.collapse-control:hover{background:#f0f1ed;color:var(--ns-ink)}.workspace-switch,.mobile-workspace{display:grid;grid-template-columns:34px minmax(0,1fr) 8px;align-items:center;gap:10px;padding:9px 11px;margin-bottom:22px;border:1px solid var(--ns-line);border-radius:999px;background:#fafaf7}.workspace-avatar,.guest-avatar{width:34px;height:34px;display:grid;place-items:center;background:#dfe6da;color:#40503c;border-radius:50%;font-weight:750}.workspace-switch div,.mobile-workspace div,.profile div,.guest-profile span:last-child{min-width:0;display:flex;flex-direction:column}.workspace-switch strong,.mobile-workspace strong,.profile strong,.guest-profile strong{font-size:12px;font-weight:660;color:var(--ns-ink);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.workspace-switch small,.mobile-workspace small,.profile small,.guest-profile small{margin-top:2px;font-size:10px;color:var(--ns-ink-faint);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.online-dot{width:7px;height:7px;border-radius:50%;background:#73866a;box-shadow:0 0 0 3px #e8ede4}.nav-groups{display:flex;flex-direction:column;gap:20px;overflow:auto}.nav-group{display:flex;flex-direction:column;gap:7px}.nav-label{padding-left:13px;color:#9a9f98;font-size:9px;font-weight:700;text-transform:uppercase}.nav-list{display:flex;flex-direction:column;gap:4px}.nav-list button{width:100%;min-height:42px;padding:0 13px;border:0;border-radius:999px;background:transparent;color:var(--ns-ink-soft);display:flex;align-items:center;gap:11px;cursor:pointer;text-align:left;white-space:nowrap;transition:background .16s ease,color .16s ease,transform .16s ease}.nav-list button:hover{background:#f0f1ed;color:var(--ns-ink)}.nav-list button:active{transform:scale(.985)}.nav-list button.active{background:#242a25;color:#fff;font-weight:630;box-shadow:0 5px 14px rgba(31,36,33,.14)}.nav-icon{width:24px;height:24px;flex:0 0 24px;display:grid;place-items:center;border-radius:50%;background:rgba(102,112,103,.08)}.nav-list button.active .nav-icon{background:rgba(255,255,255,.12)}.nav-icon :deep(svg){width:15px;height:15px}.side-bottom{margin-top:auto;padding:16px 0 18px;border-top:1px solid var(--ns-line);background:#fff}.profile{display:grid;grid-template-columns:36px minmax(0,1fr) 32px;align-items:center;gap:9px;padding:3px 5px}.guest-profile{width:100%;display:grid;grid-template-columns:36px minmax(0,1fr);gap:9px;align-items:center;border:1px solid var(--ns-line);background:#fafaf7;border-radius:999px;padding:7px 10px;text-align:left;cursor:pointer}.icon-action,.top-icon{width:32px;height:32px;display:grid;place-items:center;border:1px solid transparent;border-radius:50%;background:transparent;color:var(--ns-ink-soft);cursor:pointer}.icon-action:hover,.top-icon:hover{border-color:var(--ns-line);background:#fff;color:var(--ns-ink)}.main{min-width:0;margin-left:268px;transition:margin-left .2s ease}.topbar{height:76px;padding:0 34px;display:flex;align-items:center;justify-content:space-between;gap:20px;border-bottom:1px solid rgba(222,223,216,.8);background:rgba(246,246,242,.9);position:sticky;top:0;z-index:15;backdrop-filter:blur(16px)}.topbar-title{display:flex;align-items:center;gap:10px;min-width:0}.topbar h1{margin:2px 0 0;font-size:17px;line-height:1.2}.crumb{color:var(--ns-ink-faint);font-size:9px;font-weight:650}.top-actions{display:flex;align-items:center;gap:10px}.credit-pill{height:36px;display:flex;align-items:center;gap:8px;padding:0 14px;border:1px solid var(--ns-line);border-radius:999px;background:rgba(255,255,255,.82)}.credit-pill :deep(svg){width:15px;color:#c9a62d}.credit-pill span{font-size:10px;color:var(--ns-ink-faint)}.credit-pill strong{font-size:12px}.content{padding:28px 34px 56px;max-width:1500px;margin:0 auto}.mobile-menu{display:none;color:var(--ns-ink)!important}.nav-groups.mobile{padding-top:8px}.mobile-workspace{margin:8px 0 20px;grid-template-columns:34px minmax(0,1fr)}
.collapsed .sidebar{width:84px;padding-inline:13px}.collapsed .brand-row{padding-inline:2px;justify-content:center}.collapsed .brand-row .brand{margin-right:0}.collapsed .workspace-switch{display:flex;justify-content:center;padding:8px}.collapsed .workspace-switch>div,.collapsed .workspace-switch>.online-dot,.collapsed .nav-label,.collapsed .nav-list button>span:not(.nav-icon),.collapsed .profile>div,.collapsed .profile .icon-action,.collapsed .guest-profile>span:last-child{display:none}.collapsed .nav-groups{gap:14px;overflow:visible}.collapsed .nav-list button{justify-content:center;padding:0}.collapsed .nav-icon{width:30px;height:30px;flex-basis:30px}.collapsed .guest-profile{display:flex;justify-content:center;padding:7px}.collapsed .profile{display:flex;justify-content:center;padding:3px}.collapsed .main{margin-left:84px}
@media(max-width:900px){.sidebar{display:none}.main{margin-left:0}.mobile-menu{display:inline-flex;margin-left:-8px}.topbar{height:68px;padding:0 16px}.content{padding:22px 16px 44px}.top-icon{display:none}}@media(max-width:520px){.credit-pill{display:none}.top-actions .arco-btn{display:inline-flex;height:34px;padding-inline:15px}}
</style>

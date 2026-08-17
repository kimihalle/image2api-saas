import { chromium } from 'playwright-core'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'

const base = process.env.QA_BASE_URL || 'http://127.0.0.1:2100'
const token = process.env.QA_ADMIN_TOKEN || ''
const userToken = process.env.QA_USER_TOKEN || ''
const browserPath = process.env.QA_BROWSER_PATH || 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'
const output = path.resolve('../qa-screens')

if (!token) throw new Error('QA_ADMIN_TOKEN is required')
await mkdir(output, { recursive: true })

const browser = await chromium.launch({ executablePath: browserPath, headless: true })
const report = []

async function capture(item) {
  const context = await browser.newContext({ viewport: { width: item.width, height: item.height } })
  await context.addCookies([{ name: 'vivid_session', value: token, url: base, httpOnly: true, sameSite: 'Lax' }])
  await context.addInitScript((session) => localStorage.setItem('northstar_session', session), token)
  const page = await context.newPage()
  const errors = []
  page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()) })
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('response', (response) => {
    if (response.status() >= 400) errors.push(`HTTP ${response.status()} ${response.url()}`)
  })

  await page.goto(base + item.path, { waitUntil: 'networkidle' })
  await page.locator('main').waitFor({ state: 'visible' })
  if (item.waitFor) await page.locator('h2').filter({ hasText: item.waitFor }).first().waitFor({ state: 'visible' })

  const state = await page.evaluate(() => {
    const bodyText = document.body.innerText
    const visible = (element) => {
      const style = getComputedStyle(element)
      const rect = element.getBoundingClientRect()
      return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0
    }
    const directionalIcons = [...document.querySelectorAll('svg')]
      .filter(visible)
      .map((element) => element.className?.baseVal || element.getAttribute('class') || '')
      .filter((className) => /(left|right|arrow)/i.test(className))
    return {
      path: location.pathname,
      title: document.title,
      bodyText,
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
      directionalIcons,
    }
  })

  if (item.path.startsWith('/admin/')) {
    if (state.bodyText.includes('新建模型') || state.bodyText.includes('可用额度')) {
      throw new Error(`${item.path} still exposes a removed admin top-bar action`)
    }
  }
  if (state.scrollWidth > state.clientWidth + 1) throw new Error(`${item.path} has horizontal overflow (${state.scrollWidth}/${state.clientWidth})`)
  if (/[→←➜➔]/.test(state.bodyText)) throw new Error(`${item.path} contains a directional arrow glyph`)
  if (errors.length) throw new Error(`${item.path} browser errors: ${errors.join(' | ')}`)

  await page.screenshot({ path: path.join(output, `${item.name}.png`), fullPage: true })
  report.push({ name: item.name, path: state.path, width: item.width, title: state.title, directionalIcons: state.directionalIcons })
  await context.close()
}

try {
  const cases = [
    { name: 'settings-operations-desktop', path: '/admin/settings', width: 1440, height: 960, waitFor: '系统设置' },
    { name: 'settings-operations-mobile', path: '/admin/settings', width: 390, height: 844, waitFor: '系统设置' },
    { name: 'works-desktop', path: '/admin/works', width: 1440, height: 960, waitFor: '作品管理' },
    { name: 'works-mobile', path: '/admin/works', width: 390, height: 844, waitFor: '作品管理' },
    { name: 'invites-desktop', path: '/admin/invites', width: 1440, height: 960, waitFor: '邀请记录' },
    { name: 'billing-no-arrows-desktop', path: '/admin/billing', width: 1440, height: 960, waitFor: '订单与账本' },
    { name: 'logs-no-arrows-desktop', path: '/admin/logs', width: 1440, height: 960, waitFor: '生成日志' },
    { name: 'generate-managed-model-desktop', path: '/app/generate', width: 1440, height: 960, waitFor: '生成工作台' },
    { name: 'history-no-arrows-desktop', path: '/app/history', width: 1440, height: 960, waitFor: '生成记录' },
    { name: 'rewards-desktop', path: '/app/rewards', width: 1440, height: 960, waitFor: '签到与邀请' },
    { name: 'rewards-mobile', path: '/app/rewards', width: 390, height: 844, waitFor: '签到与邀请' },
  ]
  for (const item of cases) await capture(item)

  const context = await browser.newContext({ viewport: { width: 1440, height: 960 } })
  await context.addCookies([{ name: 'vivid_session', value: token, url: base, httpOnly: true, sameSite: 'Lax' }])
  await context.addInitScript((session) => localStorage.setItem('northstar_session', session), token)
  const page = await context.newPage()
  await page.goto(`${base}/admin/settings`, { waitUntil: 'networkidle' })
  await page.getByRole('button', { name: '返回创作空间', exact: true }).click()
  await page.waitForURL(`${base}/`)
  await page.goto(`${base}/admin/settings`, { waitUntil: 'networkidle' })
  const tabLabels = await page.locator('.arco-tabs-tab-title').allTextContents()
  const expectedTabs = ['站点品牌', '注册安全', 'SMTP 邮件', '积分奖励', '去 AI 特征', '平台公告', '文件存储', '网络代理']
  for (const label of expectedTabs) {
    if (!tabLabels.includes(label)) throw new Error(`Missing settings tab: ${label}`)
    await page.getByText(label, { exact: true }).first().click()
  }
  await page.goto(`${base}/app/generate`, { waitUntil: 'networkidle' })
  const modelControlText = await page.locator('.composer').innerText()
  if (!modelControlText.includes('Adobe Firefly GPT Image')) throw new Error('Managed model is not visible in the user generation workspace')
  await context.close()

  let announcementVerified = false
  if (userToken) {
    const userContext = await browser.newContext({ viewport: { width: 390, height: 844 } })
    await userContext.addCookies([{ name: 'vivid_session', value: userToken, url: base, httpOnly: true, sameSite: 'Lax' }])
    await userContext.addInitScript((session) => localStorage.setItem('northstar_session', session), userToken)
    const userPage = await userContext.newPage()
    await userPage.goto(`${base}/app/overview`, { waitUntil: 'networkidle' })
    await userPage.getByText('运营公告功能验证', { exact: true }).waitFor({ state: 'visible' })
    await userPage.screenshot({ path: path.join(output, 'announcement-user-mobile.png'), fullPage: true })
    const [seenResponse] = await Promise.all([
      userPage.waitForResponse((response) => response.url().includes('/announcement/seen')),
      userPage.getByRole('button', { name: '我知道了', exact: true }).click(),
    ])
    if (!seenResponse.ok()) throw new Error(`Announcement seen request failed: ${seenResponse.status()}`)
    announcementVerified = true
    await userContext.close()
  }

  console.log(JSON.stringify({ ok: true, tabs: expectedTabs, managedModel: 'Adobe Firefly GPT Image', announcementVerified, report }, null, 2))
} finally {
  await browser.close()
}

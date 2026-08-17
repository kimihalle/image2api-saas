import { chromium } from 'playwright-core'
import path from 'node:path'

const base = process.env.QA_BASE_URL || 'http://127.0.0.1:2100'
const token = process.env.QA_ADMIN_TOKEN || ''
const skipGeneration = process.env.QA_SKIP_GENERATION === '1'
const browserPath = process.env.QA_BROWSER_PATH || 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'
if (!token) throw new Error('QA_ADMIN_TOKEN is required')

const browser = await chromium.launch({ executablePath: browserPath, headless: true })
const context = await browser.newContext({ viewport: { width: 1440, height: 960 } })
await context.addInitScript((value) => localStorage.setItem('northstar_session', value), token)
const page = await context.newPage()
const errors = []
page.on('pageerror', (error) => errors.push(error.message))
page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()) })

async function open(route) {
  await page.goto(base + route, { waitUntil: 'networkidle' })
  const notice = page.getByRole('button', { name: '我知道了', exact: true })
  if (await notice.count()) await notice.click()
}

async function verifyViewer(route, trigger) {
  await open(route)
  await trigger()
  const viewer = page.locator('.arco-image-preview:not(.arco-image-preview-hide)')
  await viewer.waitFor({ state: 'visible' })
  if (await page.locator('.arco-modal-container:visible').count()) throw new Error(`${route} still uses a visible modal card for image preview`)
  const image = viewer.locator('.arco-image-preview-img')
  await image.waitFor({ state: 'visible' })
  const box = await image.boundingBox()
  if (!box) throw new Error(`${route} preview image has no visible bounds`)
  const before = await image.evaluate((element) => getComputedStyle(element.parentElement).transform)
  const actions = await viewer.locator('.arco-image-preview-toolbar-action').count()
  if (actions < 3) throw new Error(`${route} zoom toolbar is incomplete`)
  await viewer.locator('.arco-image-preview-toolbar-action').first().click()
  await page.waitForTimeout(250)
  const after = await image.evaluate((element) => getComputedStyle(element.parentElement).transform)
  if (before === after) {
    const detail = await viewer.locator('.arco-image-preview-toolbar-action').evaluateAll((items) => items.map((item) => ({ text: item.textContent, html: item.outerHTML.slice(0, 300) })))
    const imageDetail = await image.evaluate((element) => ({ style: element.getAttribute('style'), parent: element.parentElement?.getAttribute('style') }))
    throw new Error(`${route} zoom action did not change image transform: ${JSON.stringify({ before, after, detail, imageDetail })}`)
  }
  await page.screenshot({ path: path.resolve(`../qa-screens/${route.includes('works') ? 'works-zoom-viewer' : 'logs-zoom-viewer'}.png`), fullPage: true })
  await viewer.locator('.arco-image-preview-close-btn').click()
  return { actions, zoomChanged: true, modalCards: 0 }
}

try {
  const report = {}
  report.works = await verifyViewer('/admin/works', async () => page.locator('.media-button').first().click())
  report.logs = await verifyViewer('/admin/logs', async () => page.locator('.asset-thumb:not([disabled])').first().click())

  if (!skipGeneration) {
  await open('/admin/models')
  await page.getByRole('button', { name: '调用测试', exact: true }).first().click()
  await page.locator('.test-layout').waitFor({ state: 'visible' })
  const generationResponse = page.waitForResponse((response) => response.url().includes('/admin/api/test'), { timeout: 300000 })
  await page.getByRole('button', { name: '开始测试', exact: true }).click()
  await page.getByText(/图像生成中|视频生成中/).first().waitFor({ state: 'visible' })
  await page.getByRole('button', { name: '关闭', exact: true }).click()
  const started = Date.now()
  await page.getByRole('button', { name: '生成日志', exact: true }).click()
  await page.waitForURL(`${base}/admin/logs`)
  await page.getByRole('heading', { name: '生成日志', exact: true }).last().waitFor({ state: 'visible' })
  const navigationMs = Date.now() - started
  if (navigationMs > 5000) throw new Error(`navigation blocked during generation for ${navigationMs}ms`)
  const healthStarted = Date.now()
  const health = await page.request.get(`${base}/health`)
  const healthMs = Date.now() - healthStarted
  if (!health.ok() || healthMs > 2000) throw new Error(`health request blocked during generation: ${health.status()} ${healthMs}ms`)
  const response = await generationResponse
  report.concurrentGeneration = { navigationMs, healthMs, generationStatus: response.status() }
  }

  if (errors.length) throw new Error(`browser errors: ${errors.join(' | ')}`)
  console.log(JSON.stringify({ ok: true, report }, null, 2))
} finally {
  await context.close()
  await browser.close()
}

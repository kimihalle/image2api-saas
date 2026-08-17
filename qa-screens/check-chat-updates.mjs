import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'

const token = process.env.QA_TOKEN
if (!token) throw new Error('QA_TOKEN is required')

const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
})
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
const errors = []
page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`))
page.on('console', (message) => {
  if (message.type() === 'error') errors.push(`console: ${message.text()}`)
})
await page.addInitScript((sessionToken) => {
  localStorage.setItem('northstar_session', sessionToken)
  localStorage.setItem('image_generation_mode', 'chat')
}, token)
await page.route('**/admin/api/logs?*', async (route) => {
  const url = route.request().url()
  if (url.includes('kind=video')) {
    const now = Math.floor(Date.now() / 1000)
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [{
        id: 'qa-video-success', ts: now, kind: 'video', source: 'user', status: 'success',
        model: 'QA Video', prompt: '视频操作按钮回归测试', ratio: '16:9', resolution: '1080p', duration: '5s', cost: 2,
      }] }),
    })
  }
  if (!url.includes('kind=image')) return route.continue()
  const now = Math.floor(Date.now() / 1000)
  const rows = Array.from({ length: 16 }, (_, index) => ({
    id: `qa-history-${index}`,
    ts: now - (16 - index) * 60,
    kind: 'image',
    source: 'user',
    status: 'failed',
    model: 'Nano Banana Pro',
    prompt: `视觉回归测试对话 ${String(index + 1).padStart(2, '0')}`,
    ratio: index % 2 ? '16:9' : '1:1',
    resolution: '1K',
    cost: 1,
    error: '测试记录，不会提交生成任务',
  }))
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ data: rows }) })
})
const sampleRate = 8000
const sampleData = Buffer.alloc(sampleRate * 2)
const sampleMedia = Buffer.alloc(44 + sampleData.length)
sampleMedia.write('RIFF', 0)
sampleMedia.writeUInt32LE(36 + sampleData.length, 4)
sampleMedia.write('WAVEfmt ', 8)
sampleMedia.writeUInt32LE(16, 16)
sampleMedia.writeUInt16LE(1, 20)
sampleMedia.writeUInt16LE(1, 22)
sampleMedia.writeUInt32LE(sampleRate, 24)
sampleMedia.writeUInt32LE(sampleRate * 2, 28)
sampleMedia.writeUInt16LE(2, 32)
sampleMedia.writeUInt16LE(16, 34)
sampleMedia.write('data', 36)
sampleMedia.writeUInt32LE(sampleData.length, 40)
sampleData.copy(sampleMedia, 44)
await page.route('**/admin/api/video/jobs/qa-video-success/content', (route) => route.fulfill({ status: 200, contentType: 'audio/wav', body: sampleMedia }))

await page.goto('http://127.0.0.1:2100/app/generate', { waitUntil: 'networkidle' })
const noticeButton = page.getByRole('button', { name: '我知道了' })
try {
  await noticeButton.waitFor({ state: 'visible', timeout: 1500 })
  await noticeButton.click()
} catch {
  // No active announcement.
}
await page.locator('.image-turn').nth(15).waitFor({ state: 'attached' })
const initialScroll = await page.locator('.image-conversation').evaluate((element) => ({
  top: element.scrollTop,
  expected: element.scrollHeight - element.clientHeight,
}))
if (Math.abs(initialScroll.top - initialScroll.expected) > 2) throw new Error(`image conversation did not open at the bottom: ${JSON.stringify(initialScroll)}`)
if (await page.locator('.turn-actions button').count() !== 32) throw new Error('conversation action icons are incomplete')
await page.screenshot({ path: 'qa-screens/image-chat-actions.png', fullPage: true })
await page.getByRole('button', { name: '清屏' }).click()
if (await page.locator('.image-turn').count()) throw new Error('clear screen did not remove completed turns')
await page.getByRole('radio', { name: '工作台模式' }).click()
const studioButtonText = await page.locator('.studio-submit').innerText()
if (!studioButtonText.includes('额度')) throw new Error('image studio cost is not inside create button')
await page.screenshot({ path: 'qa-screens/image-studio-cost.png', fullPage: true })

await page.goto('http://127.0.0.1:2100/app/video', { waitUntil: 'networkidle' })
await page.locator('.create-button').waitFor({ state: 'visible' })
if (!(await page.locator('.create-button').innerText()).includes('额度')) throw new Error('video cost is not inside create button')
if (await page.locator('.video-actions button').count() !== 2) throw new Error('completed video does not show download and delete actions')
if (await page.getByRole('button', { name: '清屏' }).count() !== 1) throw new Error('video clear-screen action is missing')
await page.screenshot({ path: 'qa-screens/video-result-actions.png', fullPage: true })
await page.getByRole('button', { name: '清屏' }).click()
if (await page.locator('.turn').count()) throw new Error('video clear screen did not remove completed turns')
if (await page.locator('.capability-note').count()) {
  await page.locator('.capability-note').waitFor({ state: 'visible' })
}
await page.screenshot({ path: 'qa-screens/video-cost-capabilities.png', fullPage: true })

const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } })
await mobile.addInitScript((sessionToken) => localStorage.setItem('northstar_session', sessionToken), token)
await mobile.goto('http://127.0.0.1:2100/app/video', { waitUntil: 'networkidle' })
await mobile.locator('.create-button').waitFor({ state: 'visible' })
const horizontalOverflow = await mobile.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
const overflowElements = await mobile.evaluate(() => [...document.querySelectorAll('body *')].map((element) => {
  const rect = element.getBoundingClientRect()
  return { tag: element.tagName, className: String(element.className || ''), left: Math.round(rect.left), right: Math.round(rect.right), width: Math.round(rect.width) }
}).filter((item) => item.right > document.documentElement.clientWidth + 1 || item.left < -1).slice(0, 20))
await mobile.screenshot({ path: 'qa-screens/video-cost-capabilities-mobile.png', fullPage: true })
console.log(JSON.stringify({ initialScroll, horizontalOverflow, overflowElements, errors }, null, 2))
await browser.close()

import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'

const token = process.env.QA_TOKEN
if (!token) throw new Error('QA_TOKEN is required')

const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
})
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 })
const errors = []
page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`))
page.on('console', (message) => {
  if (message.type() === 'error') errors.push(`console: ${message.text()}`)
})

await page.addInitScript((sessionToken) => {
  localStorage.setItem('northstar_session', sessionToken)
}, token)
await page.goto('http://127.0.0.1:2100/', { waitUntil: 'domcontentloaded' })
await page.evaluate(() => localStorage.removeItem('image_generation_mode'))
await page.goto('http://127.0.0.1:2100/app/generate', { waitUntil: 'networkidle' })
await page.locator('.mode-options').waitFor({ state: 'visible' })
await page.screenshot({ path: 'qa-screens/image-mode-chooser.png', fullPage: true })

await page.locator('.mode-options > button').nth(1).click()
await page.getByText('记住我的选择').click()
await page.getByRole('button', { name: '进入图片创作' }).click()
await page.locator('.chat-studio').waitFor({ state: 'visible' })
const noticeButton = page.getByRole('button', { name: '我知道了' })
try {
  await noticeButton.waitFor({ state: 'visible', timeout: 1500 })
  await noticeButton.click()
} catch {
  // The active announcement may already have been acknowledged.
}
await page.screenshot({ path: 'qa-screens/image-chat-mode.png', fullPage: true })

await page.reload({ waitUntil: 'networkidle' })
if (await page.locator('.mode-options').isVisible()) throw new Error('remembered mode still opened chooser')
await page.locator('.chat-studio').waitFor({ state: 'visible' })
await page.getByRole('radio', { name: '工作台模式' }).click()
await page.locator('.studio').waitFor({ state: 'visible' })
await page.screenshot({ path: 'qa-screens/image-studio-mode.png', fullPage: true })

await page.setViewportSize({ width: 390, height: 844 })
await page.getByRole('radio', { name: '对话模式' }).click()
await page.locator('.chat-studio').waitFor({ state: 'visible' })
await page.screenshot({ path: 'qa-screens/image-chat-mobile.png', fullPage: true })

const result = {
  mode: await page.evaluate(() => localStorage.getItem('image_generation_mode')),
  chatVisible: await page.locator('.chat-studio').isVisible(),
  horizontalOverflow: await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth),
  errors,
}
console.log(JSON.stringify(result, null, 2))
await browser.close()
